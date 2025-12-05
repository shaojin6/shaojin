# 工具使用流程详解

## 概述

本系统支持两种类型的 MCP 工具：
1. **本地工具**：直接集成在服务中的 Kubernetes 工具（6个）
2. **远程工具**：通过远程 MCP 服务提供的工具（如 kubernetes-mcp-server 的 21 个工具）

## 工具类型说明

### 本地工具（6个）

**位置**: `internal/server/handlers.go`

在服务启动时，如果 K8s 客户端初始化成功，会自动注册以下 6 个本地工具：

1. `list_pods` - 列出指定命名空间中的所有 Pods
2. `get_deployment_status` - 获取指定 Deployment 的详细状态信息
3. `restart_pod` - 重启指定的 Pod
4. `get_pod_logs` - 获取指定 Pod 的日志
5. `list_deployments` - 列出指定命名空间中的所有 Deployments
6. `get_service_info` - 获取指定 Service 的详细信息

**注册流程**:
```go
// cmd/web/main.go
mcpServer := mcp.NewServer("k8s-mcp-agent", "1.0.0")
if k8sClient != nil {
    server.RegisterK8sTools(mcpServer, k8sClient.Clientset)
}
```

### 远程工具（21个，以 kubernetes-mcp-server 为例）

**来源**: 远程 MCP 服务（如 `http://11.0.1.110:30080/mcp`）

远程工具包括：
- `configuration_view` - 查看配置
- `events_list` - 列出事件
- `helm_install` - 安装 Helm Chart
- `helm_list` - 列出 Helm Releases
- `helm_uninstall` - 卸载 Helm Release
- `namespaces_list` - 列出命名空间
- `nodes_log` - 获取节点日志
- `nodes_stats_summary` - 节点统计摘要
- `nodes_top` - 节点资源使用情况
- `pods_delete` - 删除 Pod
- `pods_exec` - 在 Pod 中执行命令
- `pods_get` - 获取 Pod 详情
- `pods_list` - 列出所有 Pods
- `pods_list_in_namespace` - 列出命名空间中的 Pods
- `pods_log` - 获取 Pod 日志
- `pods_run` - 运行 Pod
- `pods_top` - Pod 资源使用情况
- `resources_create_or_update` - 创建或更新资源
- `resources_delete` - 删除资源
- `resources_get` - 获取资源详情
- `resources_list` - 列出资源

## 工具加载流程

### 1. 服务启动时

**文件位置**: `internal/web/api/router.go:50`

```go
mcpClient := mcpclient.NewClient(mcpServer)  // 创建本地 MCP 客户端
toolManager := mcpclient.NewToolManager(mcpClient, cfgStore, toolsCache)  // 创建工具管理器
```

- `mcpClient`: 只管理本地注册的 6 个工具
- `toolManager`: 管理本地工具 + 远程工具，负责工具的统一调度

### 2. 聊天请求时刷新工具

**文件位置**: `internal/web/api/router.go:986-1004`

当用户发送聊天请求时，系统会：

1. **刷新远程工具列表**
   ```go
   toolManager.RefreshRemoteTools()
   ```

2. **工具刷新流程** (`internal/web/mcpclient/manager.go:51-140`):
   - 获取所有启用的远程 MCP 配置
   - 优先从缓存（Redis/MySQL）加载工具列表
   - 如果缓存未命中，创建远程客户端并获取工具
   - 将工具列表保存到缓存和内存映射中

3. **获取 Agent 可用工具**
   ```go
   tools := toolManager.ListToolsForAgent(agent)
   ```
   - 根据 Agent 配置的 `MCPServerID` 获取对应的工具列表
   - 对于 kubernetes-mcp-server Agent，返回 21 个远程工具

### 3. 工具缓存机制

**缓存层级**:
1. **Redis 缓存**（优先）
   - 快速访问
   - 缓存工具列表和工具定义
   
2. **MySQL 缓存**（备用）
   - 持久化存储
   - 服务重启后仍可用

3. **内存缓存**（运行时）
   - `cachedTools` 映射：存储已加载的工具列表
   - `remoteClients` 映射：存储远程客户端连接

## 工具调用流程

### 完整调用链路

```
用户提问
    ↓
前端发送请求 (/api/chat)
    ↓
后端验证 Agent 和 MCP 配置
    ↓
刷新远程工具列表 (RefreshRemoteTools)
    ↓
获取 Agent 可用工具 (ListToolsForAgent)
    ↓
Orchestrator 构建系统提示词（包含工具列表）
    ↓
调用 LLM（工具列表作为上下文）
    ↓
LLM 返回工具调用请求
    ↓
Orchestrator 解析工具调用
    ↓
ToolManager.CallTool(toolName, args)
    ↓
判断工具类型：
    ├─ 本地工具 → mcpClient.CallTool() → 本地处理
    └─ 远程工具 → RemoteClient.CallTool() → HTTP 请求到远程 MCP 服务
    ↓
远程 MCP 服务处理请求
    ↓
返回结果给 ToolManager
    ↓
Orchestrator 将结果反馈给 LLM
    ↓
LLM 生成最终回答
    ↓
返回给前端显示
```

### 详细步骤

#### 步骤 1: 聊天请求处理

**文件位置**: `internal/web/api/router.go:906-1004`

```go
// 1. 验证 Agent 配置
agent := cfgStore.GetAgent(agentID)

// 2. 验证 MCP 服务配置
mcpConfig := cfgStore.GetRemoteMCP(agent.MCPServerID)

// 3. 刷新远程工具
toolManager.RefreshRemoteTools()

// 4. 获取 Agent 可用工具
tools := toolManager.ListToolsForAgent(agent)
// 对于 kubernetes-mcp-server Agent，返回 21 个工具
```

#### 步骤 2: Orchestrator 处理

**文件位置**: `internal/web/chat/orchestrator.go:46-94`

```go
// 1. 获取 Agent 可用工具列表
allowedTools := o.toolManager.ListToolsForAgent(agent)

// 2. 构建系统提示词（包含工具列表）
systemPrompt := o.buildSystemPrompt(agent, allowedTools)

// 3. 调用 LLM
llmResponse := o.llmClient.Chat(messages)
```

#### 步骤 3: 解析 LLM 响应

**文件位置**: `internal/web/chat/orchestrator.go:130-220`

LLM 可能返回两种响应：
1. **直接回答**：不需要调用工具
2. **工具调用请求**：需要调用工具获取信息

```go
// 解析 LLM 响应，提取工具调用信息
toolName, toolArgs := parseToolCall(llmResponse)

// 验证工具是否在可用列表中
if !isToolAvailable(toolName, allowedTools) {
    // 工具不可用，反馈给 LLM
    return error
}
```

#### 步骤 4: 执行工具调用

**文件位置**: `internal/web/chat/orchestrator.go:237-290`

```go
// 调用工具
toolResult, err := o.toolManager.CallTool(toolName, toolArgs)
```

**ToolManager 路由逻辑** (`internal/web/mcpclient/manager.go:240-318`):

```go
// 1. 查找工具所属的客户端
clientID := tm.toolToClient[toolName]

if clientID == "local" {
    // 本地工具：直接调用本地 MCP 服务器
    return tm.localClient.CallTool(toolName, args)
} else {
    // 远程工具：调用远程 MCP 服务
    remoteClient := tm.remoteClients[clientID]
    return remoteClient.CallTool(toolName, args)
}
```

#### 步骤 5: 远程工具调用

**文件位置**: `internal/web/mcpclient/remote.go:664-724`

```go
// 构建 JSON-RPC 请求
requestBody := {
    "jsonrpc": "2.0",
    "id": requestID,
    "method": "tools/call",
    "params": {
        "name": toolName,
        "arguments": args
    }
}

// 发送 HTTP POST 请求到远程 MCP 服务
POST http://11.0.1.110:30080/mcp
Content-Type: application/json

// 解析响应（支持 JSON 和 SSE 格式）
```

#### 步骤 6: 结果反馈

**文件位置**: `internal/web/chat/orchestrator.go:292-320`

```go
// 工具调用成功，将结果添加到消息历史
messages = append(messages, {
    Role: "assistant",
    Content: llmResponse  // LLM 的工具调用请求
})
messages = append(messages, {
    Role: "user",
    Content: formatToolResult(toolResult)  // 工具执行结果
})

// 继续调用 LLM，让它基于工具结果生成最终回答
```

## 状态显示说明

### 状态 API (`/api/status`)

**文件位置**: `internal/web/api/router.go:1202-1225`

```go
// 只统计本地工具
toolsCount := len(mcpClient.ListTools())  // 返回 6
```

**显示结果**:
```json
{
    "k8s": {
        "connected": true,
        "enabled": true
    },
    "llm": {
        "configured": true
    },
    "mcp": {
        "tools": 6  // 只显示本地工具数量
    }
}
```

**注意**: 状态 API 显示的是本地工具数量，不包括远程工具。

### 实际使用的工具

**聊天时使用的工具** (`internal/web/api/router.go:998`):

```go
// 获取 Agent 可用工具（包括远程工具）
tools := toolManager.ListToolsForAgent(agent)
// 对于 kubernetes-mcp-server Agent，返回 21 个远程工具
```

**日志示例**:
```
[Chat API] Found 21 tools for agent kubernetes-mcp-server Agent: 
[configuration_view events_list helm_install helm_list helm_uninstall 
namespaces_list nodes_log nodes_stats_summary nodes_top pods_delete 
pods_exec pods_get pods_list pods_list_in_namespace pods_log pods_run 
pods_top resources_create_or_update resources_delete resources_get resources_list]

[Orchestrator] Agent kubernetes-mcp-server Agent has 21 available tools
```

## 工具使用验证

### 日志验证

查看 `service.log` 文件，可以找到以下关键日志：

1. **工具加载**:
   ```
   [Cache] Hit Redis cache for kubernetes-mcp-server (21 tools)
   [Chat API] Found 21 tools for agent kubernetes-mcp-server Agent
   ```

2. **工具调用**:
   ```
   [Orchestrator] LLM requested tool: namespaces_list with args: map[]
   [Orchestrator] Calling tool: namespaces_list
   [ToolManager] Calling remote tool namespaces_list on client kubernetes-mcp-server
   [RemoteClient] Calling tool namespaces_list on http://11.0.1.110:30080/mcp
   [Orchestrator] Tool namespaces_list called successfully
   ```

3. **工具结果**:
   ```
   [Orchestrator] Tool namespaces_list returned result (length: 2648)
   ```

### 实际使用情况

从日志分析可以确认：

1. ✅ **远程工具已加载**: 系统成功从缓存加载了 21 个远程工具
2. ✅ **工具已被调用**: 日志显示多个工具被成功调用（如 `namespaces_list`、`pods_list_in_namespace`、`resources_list` 等）
3. ✅ **工具调用成功**: 所有工具调用都返回了结果

## 总结

### 关键点

1. **本地工具（6个）**:
   - 在服务启动时注册
   - 直接调用本地 K8s 客户端
   - 状态 API 显示的就是这些工具

2. **远程工具（21个）**:
   - 在聊天请求时动态加载
   - 通过 HTTP 请求调用远程 MCP 服务
   - 实际聊天时使用的是这些工具

3. **工具选择**:
   - LLM 根据用户问题和可用工具列表，智能选择最合适的工具
   - 系统会自动路由到正确的客户端（本地或远程）

4. **缓存机制**:
   - 工具列表会被缓存到 Redis 和 MySQL
   - 减少对远程 MCP 服务的请求频率
   - 提高响应速度

### 注意事项

- 状态 API 显示的 6 个工具是本地工具，不影响远程工具的使用
- 实际聊天时，系统使用的是远程 MCP 服务提供的 21 个工具
- 如果远程 MCP 服务不可用，系统会尝试从缓存加载工具列表
- 工具调用失败时，系统会将错误信息反馈给 LLM，让 LLM 调整策略

