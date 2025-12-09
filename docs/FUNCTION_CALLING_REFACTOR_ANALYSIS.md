# Function Calling 改造模块分析

## 改造概述

参考 Dify 的实现，将系统改造为支持两种模式：
1. **Function Calling 模式**：使用标准的 `tools` 参数和 `tool_calls` 响应
2. **Prompt-based 模式**：当前实现方式（作为后备）

根据模型能力自动选择使用哪种模式。

---

## 涉及的模块

### 1. LLM 客户端层 (`internal/web/llm/client.go`)

#### 1.1 接口扩展
**当前状态：**
```go
type Client interface {
    Chat(messages []Message) (string, error)
    TestConnection() error
}
```

**需要改造：**
```go
type Client interface {
    Chat(messages []Message) (string, error)
    ChatWithTools(messages []Message, tools []Tool) (*ChatResponse, error)  // 新增
    TestConnection() error
}

// ChatResponse 包含工具调用信息
type ChatResponse struct {
    Content   string      // LLM 文本响应
    ToolCalls []ToolCall  // 工具调用列表（如果有）
}
```

#### 1.2 类型定义扩展
**需要新增：**
```go
// Tool 工具定义（用于 Function Calling）
type Tool struct {
    Type        string                 `json:"type"`        // "function"
    Function    ToolFunction           `json:"function"`
}

type ToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`  // JSON Schema
}

// ToolCall LLM 返回的工具调用
type ToolCall struct {
    ID       string                 `json:"id"`
    Type     string                 `json:"type"`  // "function"
    Function ToolCallFunction       `json:"function"`
}

type ToolCallFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`  // JSON 字符串
}

// Message 扩展支持 tool_calls
type Message struct {
    Role      string      `json:"role"`
    Content   string      `json:"content,omitempty"`
    ToolCalls []ToolCall  `json:"tool_calls,omitempty"`  // 新增
}
```

#### 1.3 具体客户端实现
**需要修改的客户端：**
- `DashScopeClient.Chat()` → 新增 `ChatWithTools()`
- `OpenAIClient.Chat()` → 新增 `ChatWithTools()`
- `OllamaClient.Chat()` → 新增 `ChatWithTools()`（如果支持）

**改造点：**
1. 添加 `tools` 参数到 API 请求
2. 解析响应中的 `tool_calls` 字段
3. 返回 `ChatResponse` 而不是简单的 `string`

---

### 2. 类型定义层 (`internal/web/types/types.go`)

#### 2.1 新增类型
**需要添加：**
```go
// ModelFeature 模型能力枚举
type ModelFeature string

const (
    ModelFeatureToolCall      ModelFeature = "tool-call"
    ModelFeatureMultiToolCall ModelFeature = "multi-tool-call"
    ModelFeatureStreamToolCall ModelFeature = "stream-tool-call"
)

// ModelSchema 模型架构信息
type ModelSchema struct {
    Provider string         `json:"provider"`
    Model    string         `json:"model"`
    Features []ModelFeature `json:"features,omitempty"`
}

// AgentConfig 扩展
type AgentConfig struct {
    // ... 现有字段
    Strategy string `json:"strategy,omitempty"`  // "function_call" | "prompt_based" (自动设置)
}
```

---

### 3. Orchestrator 层 (`internal/web/chat/orchestrator.go`)

#### 3.1 核心改造
**当前流程：**
```
1. buildSystemPrompt() - 构建包含工具信息的提示词
2. llmClient.Chat() - 调用 LLM
3. parseLLMResponse() - 解析文本响应，提取工具调用
```

**改造后流程：**
```
1. detectFunctionCallingSupport() - 检测模型能力
2. 根据能力选择模式：
   - Function Calling: 
     * 转换工具为 Tool 格式
     * llmClient.ChatWithTools() - 传递 tools 参数
     * 直接从响应读取 tool_calls
   - Prompt-based:
     * buildSystemPrompt() - 构建提示词
     * llmClient.Chat() - 调用 LLM
     * parseLLMResponse() - 解析文本响应
```

#### 3.2 需要新增的方法
```go
// 检测模型是否支持 Function Calling
func (o *Orchestrator) detectFunctionCallingSupport(llmConfig *types.LLMConfig) bool

// 转换 MCP Tool 为 LLM Tool 格式
func (o *Orchestrator) convertToolsToLLMFormat(tools []mcp.Tool) []llm.Tool

// Function Calling 模式处理
func (o *Orchestrator) chatWithFunctionCalling(ctx context.Context, ...) (*types.ChatResponse, error)

// Prompt-based 模式处理（保留现有逻辑）
func (o *Orchestrator) chatWithPromptBased(ctx context.Context, ...) (*types.ChatResponse, error)
```

#### 3.3 需要修改的方法
- `Chat()` - 主入口，根据模式分发
- `buildSystemPrompt()` - 仅在 Prompt-based 模式下使用
- `parseLLMResponse()` - 仅在 Prompt-based 模式下使用（保留作为后备）

---

### 4. 模型能力检测模块（新建 `internal/web/llm/capabilities.go`）

#### 4.1 功能
检测不同 LLM 提供商和模型是否支持 Function Calling。

**实现方式：**
```go
// 模型能力映射表
var modelCapabilities = map[string][]ModelFeature{
    // DashScope/Qwen
    "qwen-max":     {ModelFeatureToolCall, ModelFeatureMultiToolCall},
    "qwen-plus":    {ModelFeatureToolCall, ModelFeatureMultiToolCall},
    "qwen-turbo":   {ModelFeatureToolCall},
    
    // OpenAI
    "gpt-4":        {ModelFeatureToolCall, ModelFeatureMultiToolCall},
    "gpt-3.5-turbo": {ModelFeatureToolCall, ModelFeatureMultiToolCall},
    
    // Ollama（需要确认）
    // ...
}

// 检测模型能力
func DetectModelCapabilities(provider, model string) []ModelFeature

// 检查是否支持 Function Calling
func SupportsFunctionCalling(provider, model string) bool
```

#### 4.2 可选：动态检测
如果 LLM API 提供能力查询接口，可以动态检测而不是硬编码。

---

### 5. API 路由层 (`internal/web/api/router.go`)

#### 5.1 可能需要新增的接口
```go
// 获取模型能力信息
GET /api/models/:provider/:model/capabilities
Response: {
    "provider": "dashscope",
    "model": "qwen-max",
    "features": ["tool-call", "multi-tool-call"]
}
```

#### 5.2 现有接口修改
- `/api/chat` - 无需修改，Orchestrator 内部处理
- 可能需要添加日志，记录使用的模式

---

### 6. 前端界面层 (`web-ui/src/components/`)

#### 6.1 AgentConfig.vue
**需要添加：**
```vue
<!-- Agent Mode 显示（只读） -->
<el-form-item label="Agent Mode">
  <el-tag :type="isFunctionCall ? 'success' : 'info'">
    {{ isFunctionCall ? 'Function Calling' : 'Prompt-based' }}
  </el-tag>
  <el-tooltip content="根据模型能力自动选择，不可手动修改">
    <el-icon><QuestionFilled /></el-icon>
  </el-tooltip>
</el-form-item>
```

#### 6.2 需要新增的工具函数
```javascript
// utils/model-capabilities.js
export function supportsFunctionCall(features) {
  return features && features.some(f => 
    ['tool-call', 'multi-tool-call', 'stream-tool-call'].includes(f)
  )
}

// 从 API 获取模型能力
export async function getModelCapabilities(provider, model) {
  const response = await fetch(`/api/models/${provider}/${model}/capabilities`)
  return response.json()
}
```

#### 6.3 ConfigPanel.vue 或相关组件
- 在选择 LLM 模型时，自动检测并显示 Agent Mode
- 显示提示信息："此模型支持 Function Calling，将自动使用更可靠的工具调用方式"

---

### 7. 工具转换层（Orchestrator 内部）

#### 7.1 MCP Tool → LLM Tool 转换
```go
// 将 MCP Tool 转换为 LLM Function Calling 格式
func convertMCPToolToLLMTool(mcpTool mcp.Tool) llm.Tool {
    return llm.Tool{
        Type: "function",
        Function: llm.ToolFunction{
            Name:        mcpTool.Name,
            Description: mcpTool.Description,
            Parameters:  convertInputSchemaToJSONSchema(mcpTool.InputSchema),
        },
    }
}
```

#### 7.2 JSON Schema 转换
需要将 MCP 的 `InputSchema` 转换为标准的 JSON Schema 格式。

---

## 改造优先级

### 阶段 1：核心功能（必须）
1. ✅ LLM 客户端层扩展（`ChatWithTools` 方法）
2. ✅ 类型定义扩展（Tool, ToolCall 等）
3. ✅ Orchestrator 模式检测和分发
4. ✅ 模型能力检测模块

### 阶段 2：完善功能（重要）
5. ✅ 工具转换逻辑（MCP Tool → LLM Tool）
6. ✅ 前端显示 Agent Mode
7. ✅ 错误处理和日志

### 阶段 3：优化（可选）
8. ⚪ API 接口：模型能力查询
9. ⚪ 动态模型能力检测（如果 API 支持）
10. ⚪ 性能优化和缓存

---

## 兼容性考虑

### 向后兼容
- ✅ 保留 `Chat()` 方法（Prompt-based 模式）
- ✅ 保留 `parseLLMResponse()` 方法（后备）
- ✅ 保留 `buildSystemPrompt()` 方法（Prompt-based 使用）

### 迁移策略
1. 新功能不影响现有功能
2. 自动检测，无需用户配置
3. 不支持 Function Calling 的模型自动使用 Prompt-based

---

## 测试要点

### 单元测试
- 模型能力检测逻辑
- 工具格式转换
- Function Calling 响应解析

### 集成测试
- DashScope Function Calling 流程
- OpenAI Function Calling 流程
- Prompt-based 后备流程
- 模式自动切换

### 端到端测试
- 完整对话流程（Function Calling 模式）
- 完整对话流程（Prompt-based 模式）
- 工具调用成功/失败场景

---

## 风险评估

### 高风险
- LLM API 兼容性（不同提供商的 Function Calling 实现可能不同）
- 工具格式转换（JSON Schema 格式差异）

### 中风险
- 模型能力检测准确性（需要维护映射表）
- 两种模式的代码维护成本

### 低风险
- 前端显示（纯展示，不影响功能）
- 日志和监控（可观测性）

---

## 总结

改造涉及 **7 个主要模块**，其中：
- **核心模块**：LLM 客户端层、Orchestrator 层、模型能力检测
- **支持模块**：类型定义、工具转换、前端界面
- **可选模块**：API 接口扩展、动态检测

改造后系统将：
1. ✅ 自动检测模型能力
2. ✅ 优先使用 Function Calling（如果支持）
3. ✅ 自动回退到 Prompt-based（如果不支持）
4. ✅ 保持向后兼容

