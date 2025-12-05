# 智能问答实现逻辑详解

## 整体架构流程图

```
用户输入问题
    ↓
前端 ChatWindow.vue (发送请求)
    ↓
后端 API Router (/api/chat)
    ↓
Orchestrator (编排器)
    ↓
┌─────────────────────────────────────┐
│  1. 获取 Agent 和工具列表          │
│  2. 构建系统提示词                 │
│  3. 调用 LLM                       │
│  4. 解析 LLM 响应                  │
│  5. 判断是否需要调用工具           │
│     ├─ 是 → 执行工具调用           │
│     │         ↓                    │
│     │    ToolManager               │
│     │         ↓                    │
│     │    RemoteClient (MCP)       │
│     │         ↓                    │
│     │    Kubernetes MCP Server     │
│     │         ↓                    │
│     │    K8s API                   │
│     │         ↓                    │
│     │    返回结果给 LLM            │
│     │         ↓                    │
│     └─ 继续循环 (最多10步)         │
│                                     │
│  6. LLM 生成最终回答               │
│  7. 保存会话到 MongoDB             │
│  8. 返回结果给前端                 │
└─────────────────────────────────────┘
    ↓
前端显示结果和工具执行步骤
```

## 详细实现步骤

### 第一步：前端发送请求

**文件位置**: `web-ui/src/components/ChatWindow.vue`

**关键代码**:
```javascript
const response = await sendChat(currentSessionId.value, userMessage, selectedAgentId.value)
```

**功能**:
- 用户输入问题
- 选择 Agent（智能体）
- 发送 POST 请求到 `/api/chat`
- 包含：`sessionId`, `message`, `agentId`

---

### 第二步：后端接收请求

**文件位置**: `internal/web/api/router.go:891`

**关键步骤**:

1. **验证 Agent 配置**
   ```go
   agent := cfgStore.GetAgent(agentID)
   // 检查 Agent 是否存在、是否启用、是否关联了 MCP 服务
   ```

2. **获取 LLM 配置**
   ```go
   llmConfig := cfgStore.GetDefaultLLMConfig()
   // 始终使用用户配置的默认 LLM（如 bailian/qwen-max）
   ```

3. **验证 MCP 服务**
   ```go
   mcpConfig := cfgStore.GetRemoteMCP(agent.MCPServerID)
   // 检查 MCP 服务是否存在、是否启用
   ```

4. **刷新工具列表**
   ```go
   toolManager.RefreshRemoteTools()
   // 从缓存（Redis/MySQL）或远程 MCP 服务获取工具列表
   ```

5. **创建 Orchestrator**
   ```go
   orch := getOrchestrator(llmConfig, mcpClient, globalSessionStore)
   // 初始化 LLM 客户端和工具管理器
   ```

6. **调用 Orchestrator.Chat()**
   ```go
   response, err := orch.Chat(ctx, req.SessionID, req.Message, agent)
   ```

---

### 第三步：Orchestrator 处理对话

**文件位置**: `internal/web/chat/orchestrator.go:46`

#### 3.1 初始化阶段

```go
// 1. 获取或创建会话
session, err := o.getOrCreateSession(ctx, sessionID, agent)

// 2. 添加用户消息到会话
session.Messages = append(session.Messages, llm.Message{
    Role:    "user",
    Content: userMessage,
})

// 3. 获取 Agent 可用的工具列表
allowedTools := o.toolManager.ListToolsForAgent(agent)
// 例如：["pods_list_in_namespace", "pods_get", "deployments_list", ...]

// 4. 构建系统提示词
systemPrompt := o.buildSystemPrompt(agent, allowedTools)
```

#### 3.2 系统提示词构建

**关键内容**:
```
你是智能体 "kubernetes-mcp-server Agent"，可以帮助用户管理和查询 Kubernetes 集群。

可用工具：
- pods_list_in_namespace: 列出指定命名空间中的所有 Pods
  参数: namespace (string): Kubernetes 命名空间名称
- pods_get: 获取指定 Pod 的详细信息
  参数: name (string): Pod 名称, namespace (string): 命名空间
...

工作流程：
1. 理解用户的问题
2. 如果需要查询或操作 Kubernetes 资源，使用相应的工具
3. 基于工具返回的结果，用自然语言回答用户的问题

响应格式：
- 如果需要调用工具，请以 JSON 格式回复：
  {
    "action": "call_tool",
    "tool": "工具名称",
    "arguments": {"参数名": "参数值"},
    "thought": "你的思考过程",
    "reply": "我将调用工具来查询信息，请稍候..."
  }
- 如果可以直接回答，请以 JSON 格式回复：
  {
    "action": "respond",
    "reply": "你的回答"
  }

重要：无论何时，都必须包含 "reply" 字段，用自然语言向用户说明当前的操作或回答。
```

#### 3.3 循环处理（最多10步）

```go
for step := 0; step < o.maxSteps; step++ {
    // Step 1: 调用 LLM
    llmResponse, err := o.llmClient.Chat(messages)
    
    // Step 2: 解析 LLM 响应
    action, toolName, toolArgs, thought, reply := o.parseLLMResponse(llmResponse, allowedTools)
    
    // Step 3: 判断是否需要调用工具
    if action == "call_tool" && toolName != "" {
        // 需要调用工具
        // ...
    } else {
        // LLM 返回最终答案
        finalReply = reply
        break
    }
}
```

#### 3.4 LLM 响应解析

**解析逻辑**:
1. **尝试解析 JSON 格式**
   ```json
   {
     "action": "call_tool",
     "tool": "pods_list_in_namespace",
     "arguments": {"namespace": "zk"},
     "thought": "用户想知道 zk 命名空间下有多少个 Pod",
     "reply": "我将查询 zk 命名空间下的 Pod 列表"
   }
   ```

2. **提取关键信息**
   - `action`: "call_tool" 或 "respond"
   - `tool`: 工具名称
   - `arguments`: 工具参数
   - `thought`: LLM 的思考过程
   - `reply`: 给用户的回复

#### 3.5 工具调用流程

**当 LLM 决定调用工具时**:

```go
// 1. 记录工具调用步骤
steps = append(steps, types.ChatStep{
    Type:      "tool",
    Tool:      toolName,
    Arguments: toolArgs,
})

// 2. 调用工具管理器
toolResult, err := o.toolManager.CallTool(toolName, toolArgs)

// 3. 处理工具返回结果
if toolResult != nil && len(toolResult.Content) > 0 {
    toolResultText = toolResult.Content[0].Text
    // 例如：{"pods": [{"name": "pod1", "status": "Running"}, ...]}
}

// 4. 更新步骤结果
steps[len(steps)-1].Result = toolResultData

// 5. 将工具结果反馈给 LLM
messages = append(messages, llm.Message{
    Role:    "user",
    Content: fmt.Sprintf("工具 %s 的执行结果:\n%s\n\n请基于这个真实的工具执行结果，用自然语言回答用户的问题。必须使用 JSON 格式回复，格式：{\"action\": \"respond\", \"reply\": \"你的自然语言回答\"}。回答必须准确反映工具返回的实际数据，不要使用假设或历史数据。", toolName, toolResultText),
})

// 6. 继续下一轮循环
continue
```

---

### 第四步：工具管理器

**文件位置**: `internal/web/mcpclient/manager.go:240`

**功能**: 路由工具调用到正确的客户端（本地或远程）

```go
func (tm *ToolManager) CallTool(toolName string, args map[string]interface{}) (*mcp.ToolsCallResult, error) {
    // 1. 查找工具所属的客户端
    clientID := tm.toolToClient[toolName]
    // 例如：clientID = "kubernetes-mcp-server"
    
    // 2. 判断是本地还是远程工具
    if clientID == "local" {
        return tm.localClient.CallTool(toolName, args)
    }
    
    // 3. 调用远程工具
    remoteClient := tm.remoteClients[clientID]
    return remoteClient.CallTool(toolName, args)
}
```

---

### 第五步：远程 MCP 客户端

**文件位置**: `internal/web/mcpclient/remote.go:664`

**功能**: 通过 JSON-RPC 协议调用远程 MCP 服务

```go
func (c *RemoteClient) CallTool(name string, args map[string]interface{}) (*mcp.ToolsCallResult, error) {
    // 1. 构建 JSON-RPC 请求
    requestBody := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      requestID,
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name":      name,
            "arguments": args,
        },
    }
    
    // 2. 发送 HTTP POST 请求到 MCP 服务
    // URL: http://kubernetes-mcp-server:port
    resp, err := http.Post(url, "application/json", jsonData)
    
    // 3. 解析 JSON-RPC 响应
    // 返回工具执行结果
    return &result, nil
}
```

---

### 第六步：Kubernetes MCP Server

**功能**: 
- 接收 JSON-RPC 工具调用请求
- 调用 Kubernetes API
- 返回 Pod、Deployment 等资源信息

**示例工具调用**:
```json
{
  "jsonrpc": "2.0",
  "id": "tool-call-123",
  "method": "tools/call",
  "params": {
    "name": "pods_list_in_namespace",
    "arguments": {
      "namespace": "zk"
    }
  }
}
```

**返回结果**:
```json
{
  "jsonrpc": "2.0",
  "id": "tool-call-123",
  "result": {
    "content": [{
      "type": "text",
      "text": "{\"pods\": []}"
    }]
  }
}
```

---

### 第七步：LLM 生成最终回答

**当工具返回结果后**:

1. **工具结果反馈给 LLM**
   ```
   工具 pods_list_in_namespace 的执行结果:
   {"pods": []}
   
   请基于这个真实的工具执行结果，用自然语言回答用户的问题。
   ```

2. **LLM 再次调用，生成最终回答**
   ```json
   {
     "action": "respond",
     "reply": "在 zk 命名空间下没有找到任何 Pod。"
   }
   ```

3. **提取最终回答**
   ```go
   finalReply = reply  // "在 zk 命名空间下没有找到任何 Pod。"
   ```

---

### 第八步：保存和返回结果

```go
// 1. 保存会话到 MongoDB
sessionDoc := &store.SessionDoc{
    ID:        session.ID,
    AgentID:   session.AgentID,
    Messages:  session.Messages,
    CreatedAt: session.CreatedAt,
    UpdatedAt: session.UpdatedAt,
}
o.sessionStore.SaveSession(ctx, sessionDoc)

// 2. 返回响应
return &types.ChatResponse{
    SessionID: session.ID,
    AgentID:   agentID,
    Reply:     finalReply,  // 最终的自然语言回答
    Steps:     steps,       // 工具调用步骤（用于前端显示）
}, nil
```

---

### 第九步：前端显示结果

**显示内容**:

1. **最终回答** (`msg.content`)
   ```
   在 zk 命名空间下没有找到任何 Pod。
   ```

2. **工具执行步骤** (`msg.steps`)
   - 🔧 工具调用: `pods_list_in_namespace(namespace=zk)`
   - ✓ 执行结果: `{"pods": []}`
   - 💭 LLM 思考: `用户想知道 zk 命名空间下有多少个 Pod`

---

## 关键设计点

### 1. 工具列表缓存机制

- **Redis** → **MySQL** → **文件** → **远程 MCP 服务**
- 避免每次聊天都重新获取工具列表
- 工具刷新时间从 5 分钟降至 2.3 毫秒

### 2. 多轮对话循环

- 最多执行 10 步（`maxSteps = 10`）
- LLM 可以多次调用工具
- 每次工具调用后，结果反馈给 LLM，LLM 决定下一步

### 3. 强制使用真实数据

- 在提示词中明确要求："回答必须准确反映工具返回的实际数据，不要使用假设或历史数据"
- 工具结果直接传递给 LLM，不经过任何缓存或历史数据

### 4. 详细的日志记录

- 每个步骤都有日志记录
- 工具调用参数和结果都有预览
- 便于调试和问题排查

### 5. 超时控制

- 整体超时：60 秒
- LLM 调用超时：25 秒
- 工具调用超时：25 秒

---

## 数据流示例

**用户问题**: "zk下有多少个pod?"

**Step 1 - LLM 思考**:
```json
{
  "action": "call_tool",
  "tool": "pods_list_in_namespace",
  "arguments": {"namespace": "zk"},
  "thought": "用户想知道 zk 命名空间下有多少个 Pod",
  "reply": "我将查询 zk 命名空间下的 Pod 列表"
}
```

**Step 2 - 工具调用**:
- 工具: `pods_list_in_namespace`
- 参数: `{"namespace": "zk"}`
- 结果: `{"pods": []}`

**Step 3 - LLM 生成最终回答**:
```json
{
  "action": "respond",
  "reply": "在 zk 命名空间下没有找到任何 Pod。"
}
```

**最终返回**:
- `Reply`: "在 zk 命名空间下没有找到任何 Pod。"
- `Steps`: [
    {Type: "llm", Text: "..."},
    {Type: "tool", Tool: "pods_list_in_namespace", Arguments: {...}, Result: {...}},
    {Type: "llm", Text: "..."}
  ]

---

## 总结

整个智能问答系统的核心是：

1. **Agent（智能体）** → 绑定到 **MCP 服务** → 提供 **工具列表**
2. **LLM（大语言模型）** → 理解用户问题 → 决定调用哪些工具
3. **工具执行** → 调用 **Kubernetes MCP Server** → 查询 **K8s API** → 返回真实数据
4. **LLM 分析** → 基于工具真实结果 → 生成自然语言回答
5. **前端显示** → 显示最终回答 + 工具执行步骤

这样确保了：
- ✅ 回答基于真实的 K8s 集群数据
- ✅ 用户可以查看工具执行过程
- ✅ 支持多轮对话和多次工具调用
- ✅ 有完整的日志记录便于调试
