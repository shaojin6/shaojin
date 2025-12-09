# Function Calling 改造实施总结

## ✅ 已完成的工作

### 1. 类型定义扩展 (`internal/web/types/types.go`)
- ✅ 添加 `AgentConfig.Strategy` 字段（"function_call" | "prompt_based"）
- ✅ 添加 `ModelFeature` 枚举（tool-call, multi-tool-call, stream-tool-call）
- ✅ 添加 `ModelSchema` 结构体

### 2. 模型能力检测模块 (`internal/web/llm/capabilities.go`) - 新建
- ✅ 创建硬编码模型能力映射表
- ✅ 实现 `DetectModelCapabilities()` 函数
- ✅ 实现 `SupportsFunctionCalling()` 函数
- ✅ 支持 DashScope/Qwen、OpenAI、Ollama 等提供商

### 3. LLM 客户端层扩展 (`internal/web/llm/client.go`)
- ✅ 扩展 `Client` 接口，添加 `ChatWithTools()` 方法
- ✅ 添加 `ChatResponse`、`Tool`、`ToolFunction`、`ToolCall`、`ToolCallFunction` 类型
- ✅ 扩展 `Message` 结构体，支持 `ToolCalls` 和 `ToolCallID` 字段
- ✅ 实现 `DashScopeClient.ChatWithTools()`（使用 OpenAI 兼容接口）
- ✅ 实现 `OpenAIClient.ChatWithTools()`
- ✅ 实现 `OllamaClient.ChatWithTools()`（返回错误，不支持）

### 4. 数据库存储层 (`internal/web/store/mysql_store.go`)
- ✅ 修改 `ensureTables()`，添加 `strategy` 字段到 `agents` 表
- ✅ 添加数据库迁移逻辑（兼容现有表）
- ✅ 修改 `GetAllAgents()`、`GetAgent()`、`GetDefaultAgent()`，读取 `strategy` 字段
- ✅ 修改 `SetAgent()`，保存 `strategy` 字段

### 5. Orchestrator 层 (`internal/web/chat/orchestrator.go`)
- ✅ 修改 `Chat()` 方法签名，添加 `llmConfig` 参数
- ✅ 实现模式检测和分发逻辑
- ✅ 实现 `detectFunctionCallingSupport()` 方法
- ✅ 实现 `getOrDetermineStrategy()` 方法
- ✅ 实现 `convertToolsToLLMFormat()` 方法（MCP Tool → LLM Tool）
- ✅ 实现 `chatWithFunctionCalling()` 方法（Function Calling 模式）
- ✅ 实现 `chatWithPromptBased()` 方法（重构原有逻辑）
- ✅ 保留 `buildSystemPrompt()` 和 `parseLLMResponse()` 方法（Prompt-based 使用）

### 6. API 路由层 (`internal/web/api/router.go`)
- ✅ 在 Chat API 中添加 Strategy 检测和保存逻辑
- ✅ 修改 `orch.Chat()` 调用，传递 `llmConfig` 参数

### 7. 编译验证
- ✅ 所有模块编译通过
- ✅ 无 linter 错误

---

## 📋 待完成的工作

### 阶段 2：前端界面（可选）
- [ ] `AgentConfig.vue`：添加 Agent Mode 显示（只读）
- [ ] 新建 `web-ui/src/utils/model-capabilities.js`：模型能力工具函数
- [ ] 可选：`ConfigPanel.vue`：显示 Agent Mode 提示

---

## 🔧 核心实现细节

### 1. 模式选择逻辑
```go
// API 层：检测并保存 Strategy
if agent.Strategy == "" {
    supportsFC := llm.SupportsFunctionCalling(llmConfig.Provider, llmConfig.Model)
    if supportsFC {
        agent.Strategy = "function_call"
    } else {
        agent.Strategy = "prompt_based"
    }
    cfgStore.SetAgent(*agent) // 保存到数据库
}

// Orchestrator 层：根据 Strategy 分发
if strategy == "function_call" {
    return o.chatWithFunctionCalling(...)
} else {
    return o.chatWithPromptBased(...)
}
```

### 2. Function Calling 流程
```
1. 转换工具为 LLM 格式（convertToolsToLLMFormat）
2. 调用 ChatWithTools(messages, tools)
3. 从响应读取 tool_calls
4. 执行工具调用
5. 将工具结果添加到消息列表（包含 tool_call_id）
6. 继续下一轮或返回最终答案
```

### 3. 工具格式转换
- MCP Tool → LLM Tool（JSON Schema 格式）
- 处理 `InputSchema.Properties` → `parameters.properties`
- 处理 `InputSchema.Required` → `parameters.required`

### 4. 错误处理和回退
- Function Calling 失败时，自动回退到 Prompt-based
- 工具调用失败时，将错误信息反馈给 LLM

---

## 🗄️ 数据库变更

### agents 表
```sql
ALTER TABLE agents 
ADD COLUMN strategy VARCHAR(50) DEFAULT NULL 
COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)';

CREATE INDEX idx_strategy ON agents(strategy);
```

### 兼容性
- ✅ 自动检测字段是否存在，避免重复添加
- ✅ 现有 Agent 的 `strategy` 为 `NULL` 时，自动检测并保存

---

## 📊 支持的模型

### DashScope/Qwen（支持 Function Calling）
- qwen-max
- qwen-plus
- qwen-turbo
- qwen-7b-chat

### OpenAI（支持 Function Calling）
- gpt-4
- gpt-4-turbo
- gpt-3.5-turbo
- gpt-3.5-turbo-16k

### Ollama（不支持 Function Calling）
- 自动使用 Prompt-based 模式

---

## 🎯 关键特性

### 1. 自动检测
- ✅ 根据模型能力自动选择模式
- ✅ 首次使用时检测并保存 Strategy
- ✅ 后续直接使用存储的 Strategy

### 2. 向后兼容
- ✅ 保留所有现有方法（Chat、parseLLMResponse、buildSystemPrompt）
- ✅ Prompt-based 模式完全保留原有逻辑
- ✅ Strategy 为 NULL 时自动检测

### 3. 错误处理
- ✅ Function Calling 失败时自动回退
- ✅ 工具调用失败时反馈错误信息
- ✅ 详细的日志记录

### 4. 性能优化
- ✅ Strategy 缓存（存储在数据库）
- ✅ 避免重复检测模型能力

---

## 🧪 测试建议

### 单元测试
- [ ] 模型能力检测逻辑
- [ ] 工具格式转换
- [ ] Function Calling 响应解析

### 集成测试
- [ ] DashScope Function Calling 完整流程
- [ ] OpenAI Function Calling 完整流程
- [ ] Prompt-based 后备流程
- [ ] 模式自动切换
- [ ] Strategy 持久化

### 端到端测试
- [ ] 完整对话流程（Function Calling 模式）
- [ ] 完整对话流程（Prompt-based 模式）
- [ ] 工具调用成功/失败场景
- [ ] 服务重启后 Strategy 持久化

---

## 📝 使用说明

### 对于用户
1. **无需手动配置**：系统自动检测模型能力并选择最佳模式
2. **透明切换**：用户无感知，系统自动使用最适合的方式
3. **查看模式**：可在 Agent 配置页面查看当前使用的模式（待实现前端）

### 对于开发者
1. **添加新模型**：在 `capabilities.go` 中添加模型能力映射
2. **调试**：查看日志中的 `[Orchestrator] Using strategy: ...` 信息
3. **强制模式**：可通过数据库直接设置 Agent 的 `strategy` 字段

---

## ⚠️ 注意事项

### 1. DashScope Function Calling
- 使用 OpenAI 兼容接口：`https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions`
- 需要确认 DashScope API Key 有权限访问兼容接口

### 2. 工具格式转换
- MCP Tool 的 `InputSchema` 需要正确转换为 JSON Schema
- 确保 `properties` 和 `required` 字段正确映射

### 3. Tool Call ID
- Function Calling 中，tool 消息需要包含 `tool_call_id` 关联对应的 tool call
- 已实现：`Message.ToolCallID` 字段

### 4. 历史消息处理
- Function Calling 模式：不包含系统提示词（工具信息通过 `tools` 参数传递）
- Prompt-based 模式：包含系统提示词（工具信息在提示词中）

---

## 🚀 下一步

1. **测试**：进行全面的功能测试
2. **前端**：添加 Agent Mode 显示（可选）
3. **优化**：根据测试结果优化性能
4. **文档**：更新用户文档

---

## 📈 预期效果

### 改进
- ✅ 减少 JSON 解析错误（结构化响应）
- ✅ 提高工具调用成功率
- ✅ 更快的工具调用识别
- ✅ 减少系统提示词长度（Function Calling 模式）

### 兼容性
- ✅ 完全向后兼容
- ✅ 不支持 Function Calling 的模型自动使用 Prompt-based
- ✅ 现有功能不受影响

---

## ✅ 总结

**核心改造已完成**，系统现在支持：
1. ✅ 自动检测模型能力
2. ✅ 优先使用 Function Calling（如果支持）
3. ✅ 自动回退到 Prompt-based（如果不支持）
4. ✅ Strategy 持久化存储
5. ✅ 完全向后兼容

**建议**：进行测试验证，确保功能正常。

