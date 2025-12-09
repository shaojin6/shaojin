# Function Calling 完整改造计划

## 📋 改造总览

本次改造将系统升级为支持两种工具调用模式：
1. **Function Calling 模式**：使用标准的 `tools` 参数和 `tool_calls` 响应（优先）
2. **Prompt-based 模式**：当前实现方式（后备）

根据模型能力自动选择使用哪种模式，用户无感知。

---

## 🔧 需要改造的模块清单

### 1. LLM 客户端层 (`internal/web/llm/client.go`)

#### 1.1 接口扩展
**文件**：`internal/web/llm/client.go`

**改造内容**：
- [ ] 扩展 `Client` 接口，新增 `ChatWithTools()` 方法
- [ ] 新增 `ChatResponse` 结构体（包含 `Content` 和 `ToolCalls`）
- [ ] 新增 `Tool`、`ToolFunction`、`ToolCall`、`ToolCallFunction` 类型定义
- [ ] 扩展 `Message` 结构体，支持 `ToolCalls` 字段

**代码位置**：
```go
// 第 17-20 行：扩展接口
type Client interface {
    Chat(messages []Message) (string, error)
    ChatWithTools(messages []Message, tools []Tool) (*ChatResponse, error)  // 新增
    TestConnection() error
}

// 新增类型定义（在文件开头）
type ChatResponse struct {
    Content   string      `json:"content"`
    ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
}

type Tool struct {
    Type     string       `json:"type"`  // "function"
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`  // JSON Schema
}

type ToolCall struct {
    ID       string          `json:"id"`
    Type     string          `json:"type"`  // "function"
    Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`  // JSON 字符串
}

// 扩展 Message
type Message struct {
    Role      string      `json:"role"`
    Content   string      `json:"content,omitempty"`
    ToolCalls []ToolCall  `json:"tool_calls,omitempty"`  // 新增
}
```

#### 1.2 DashScopeClient 实现
**文件**：`internal/web/llm/client.go`

**改造内容**：
- [ ] 实现 `ChatWithTools()` 方法
- [ ] 修改请求结构，添加 `tools` 参数
- [ ] 修改响应解析，读取 `tool_calls` 字段
- [ ] 支持 OpenAI 兼容接口（`/compatible-mode/v1/chat/completions`）

**代码位置**：
```go
// 第 94-200 行：DashScopeClient.Chat() 方法
// 新增方法：ChatWithTools()
func (c *DashScopeClient) ChatWithTools(messages []Message, tools []Tool) (*ChatResponse, error) {
    // 1. 构建请求，包含 tools 参数
    // 2. 调用 API（使用兼容接口或原生接口）
    // 3. 解析响应，提取 tool_calls
    // 4. 返回 ChatResponse
}
```

#### 1.3 OpenAIClient 实现
**文件**：`internal/web/llm/client.go`

**改造内容**：
- [ ] 实现 `ChatWithTools()` 方法
- [ ] 添加 `tools` 参数到请求
- [ ] 解析 `tool_calls` 响应

**代码位置**：
```go
// 第 249-306 行：OpenAIClient.Chat() 方法
// 新增方法：ChatWithTools()
```

#### 1.4 OllamaClient 实现
**文件**：`internal/web/llm/client.go`

**改造内容**：
- [ ] 检查 Ollama 是否支持 Function Calling
- [ ] 如果支持，实现 `ChatWithTools()` 方法
- [ ] 如果不支持，返回错误或使用 Prompt-based

**代码位置**：
```go
// 第 352-403 行：OllamaClient.Chat() 方法
// 可选：ChatWithTools()
```

---

### 2. 类型定义层 (`internal/web/types/types.go`)

#### 2.1 AgentConfig 扩展
**文件**：`internal/web/types/types.go`

**改造内容**：
- [ ] 添加 `Strategy` 字段到 `AgentConfig`

**代码位置**：
```go
// 第 94-106 行：AgentConfig 结构体
type AgentConfig struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    Description  string `json:"description,omitempty"`
    MCPServerID  string `json:"mcpServerId"`
    LLMID        string `json:"llmId,omitempty"`
    SystemPrompt string `json:"systemPrompt,omitempty"`
    Strategy     string `json:"strategy,omitempty"`  // 新增："function_call" | "prompt_based"
    Enabled      bool   `json:"enabled"`
    IsDefault    bool   `json:"isDefault"`
    CreatedAt    int64  `json:"createdAt,omitempty"`
    UpdatedAt    int64  `json:"updatedAt,omitempty"`
}
```

#### 2.2 新增模型能力类型
**文件**：`internal/web/types/types.go`

**改造内容**：
- [ ] 添加 `ModelFeature` 枚举
- [ ] 添加 `ModelSchema` 结构体（可选）

**代码位置**：
```go
// 在文件末尾添加
type ModelFeature string

const (
    ModelFeatureToolCall      ModelFeature = "tool-call"
    ModelFeatureMultiToolCall ModelFeature = "multi-tool-call"
    ModelFeatureStreamToolCall ModelFeature = "stream-tool-call"
)

type ModelSchema struct {
    Provider string         `json:"provider"`
    Model    string         `json:"model"`
    Features []ModelFeature `json:"features,omitempty"`
}
```

---

### 3. 模型能力检测模块（新建 `internal/web/llm/capabilities.go`）

#### 3.1 新建文件
**文件**：`internal/web/llm/capabilities.go`（新建）

**改造内容**：
- [ ] 创建模型能力映射表
- [ ] 实现 `DetectModelCapabilities()` 函数
- [ ] 实现 `SupportsFunctionCalling()` 函数

**代码内容**：
```go
package llm

import "github.com/your-org/k8s-mcp-agent/internal/web/types"

// 模型能力映射表（硬编码）
var modelCapabilities = map[string][]types.ModelFeature{
    // DashScope/Qwen
    "qwen-max":     {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
    "qwen-plus":    {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
    "qwen-turbo":   {types.ModelFeatureToolCall},
    "qwen-7b-chat": {types.ModelFeatureToolCall},
    
    // OpenAI
    "gpt-4":              {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
    "gpt-4-turbo":        {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
    "gpt-3.5-turbo":      {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
    "gpt-3.5-turbo-16k":  {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
    
    // Ollama（需要确认）
    // "llama2": {...},
}

// DetectModelCapabilities 检测模型能力
func DetectModelCapabilities(provider, model string) []types.ModelFeature {
    key := provider + ":" + model
    if features, ok := modelCapabilities[key]; ok {
        return features
    }
    return []types.ModelFeature{}
}

// SupportsFunctionCalling 检查是否支持 Function Calling
func SupportsFunctionCalling(provider, model string) bool {
    features := DetectModelCapabilities(provider, model)
    for _, f := range features {
        if f == types.ModelFeatureToolCall || f == types.ModelFeatureMultiToolCall {
            return true
        }
    }
    return false
}
```

---

### 4. Orchestrator 层 (`internal/web/chat/orchestrator.go`)

#### 4.1 Chat 方法改造
**文件**：`internal/web/chat/orchestrator.go`

**改造内容**：
- [ ] 修改 `Chat()` 方法，添加模式检测和分发逻辑
- [ ] 保留现有 Prompt-based 逻辑

**代码位置**：
```go
// 第 46 行：Chat() 方法
func (o *Orchestrator) Chat(ctx context.Context, sessionID string, userMessage string, agent *types.AgentConfig) (*types.ChatResponse, error) {
    // 1. 获取或创建会话
    // 2. 检测或读取 Agent Strategy
    // 3. 根据 Strategy 选择处理方式
    //    - function_call: chatWithFunctionCalling()
    //    - prompt_based: chatWithPromptBased()
}
```

#### 4.2 新增方法
**文件**：`internal/web/chat/orchestrator.go`

**改造内容**：
- [ ] 新增 `detectFunctionCallingSupport()` 方法
- [ ] 新增 `chatWithFunctionCalling()` 方法
- [ ] 新增 `chatWithPromptBased()` 方法（重构现有逻辑）
- [ ] 新增 `convertToolsToLLMFormat()` 方法
- [ ] 新增 `saveAgentStrategy()` 方法

**代码位置**：
```go
// 在文件末尾添加新方法

// detectFunctionCallingSupport 检测模型是否支持 Function Calling
func (o *Orchestrator) detectFunctionCallingSupport(llmConfig *types.LLMConfig) bool {
    return llm.SupportsFunctionCalling(llmConfig.Provider, llmConfig.Model)
}

// chatWithFunctionCalling Function Calling 模式处理
func (o *Orchestrator) chatWithFunctionCalling(ctx context.Context, ...) (*types.ChatResponse, error) {
    // 1. 转换工具为 LLM 格式
    // 2. 调用 ChatWithTools()
    // 3. 从响应读取 tool_calls
    // 4. 执行工具调用
    // 5. 将结果反馈给 LLM
}

// chatWithPromptBased Prompt-based 模式处理（重构现有逻辑）
func (o *Orchestrator) chatWithPromptBased(ctx context.Context, ...) (*types.ChatResponse, error) {
    // 将现有的 Chat() 方法逻辑移到这里
    // 使用 buildSystemPrompt() 和 parseLLMResponse()
}

// convertToolsToLLMFormat 转换 MCP Tool 为 LLM Tool 格式
func (o *Orchestrator) convertToolsToLLMFormat(tools []mcp.Tool) []llm.Tool {
    // 转换逻辑
}

// saveAgentStrategy 保存 Agent Strategy
func (o *Orchestrator) saveAgentStrategy(agentID, strategy string) error {
    // 调用 store 保存
}
```

#### 4.3 保留的方法
**文件**：`internal/web/chat/orchestrator.go`

**改造内容**：
- [ ] 保留 `buildSystemPrompt()` 方法（Prompt-based 使用）
- [ ] 保留 `parseLLMResponse()` 方法（Prompt-based 使用）
- [ ] 保留 `getOrCreateSession()` 等方法

**代码位置**：
- 第 1010 行：`buildSystemPrompt()`
- 第 939 行：`parseLLMResponse()`
- 第 1201 行：`getOrCreateSession()`

---

### 5. 数据库存储层 (`internal/web/store/mysql_store.go`)

#### 5.1 数据库表结构变更
**文件**：`internal/web/store/mysql_store.go`

**改造内容**：
- [ ] 修改 `ensureTables()` 方法，添加 `strategy` 字段到 `agents` 表
- [ ] 添加数据库迁移逻辑（如果表已存在）

**代码位置**：
```go
// 第 54-69 行：agents 表 DDL
ddl := `
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mcp_server_id VARCHAR(255) NOT NULL,
    llm_id VARCHAR(255),
    system_prompt LONGTEXT,
    strategy VARCHAR(50) DEFAULT NULL COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)',  // 新增
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_mcp_server_id (mcp_server_id),
    INDEX idx_enabled (enabled),
    INDEX idx_is_default (is_default),
    INDEX idx_strategy (strategy)  // 新增
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`
```

#### 5.2 数据访问方法修改
**文件**：`internal/web/store/mysql_store.go`

**改造内容**：
- [ ] 修改 `GetAllAgents()` 方法，读取 `strategy` 字段
- [ ] 修改 `GetAgent()` 方法，读取 `strategy` 字段
- [ ] 修改 `SetAgent()` 方法，保存 `strategy` 字段

**代码位置**：
```go
// 第 85-88 行：GetAllAgents() SELECT 语句
SELECT id, name, description, mcp_server_id, llm_id, system_prompt, 
       strategy, enabled, is_default, created_at, updated_at  // 添加 strategy
FROM agents

// 第 206-220 行：SetAgent() INSERT 语句
INSERT INTO agents (
    id, name, description, mcp_server_id, llm_id, system_prompt,
    strategy, enabled, is_default, created_at, updated_at  // 添加 strategy
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)  // 添加一个 ?

// 第 211-219 行：ON DUPLICATE KEY UPDATE
strategy = VALUES(strategy),  // 添加
```

---

### 6. API 路由层 (`internal/web/api/router.go`)

#### 6.1 现有接口（无需修改）
**文件**：`internal/web/api/router.go`

**改造内容**：
- [ ] `/api/chat` 接口无需修改（Orchestrator 内部处理）
- [ ] 可选：添加日志，记录使用的模式

**代码位置**：
```go
// 第 1122 行：Chat API
// 可选：添加日志
log.Printf("[Chat API] Using strategy: %s for agent %s", agent.Strategy, agent.Name)
```

#### 6.2 可选：新增接口
**文件**：`internal/web/api/router.go`

**改造内容**：
- [ ] 可选：添加模型能力查询接口

**代码位置**：
```go
// 新增路由
apiGroup.GET("/models/:provider/:model/capabilities", func(c *gin.Context) {
    provider := c.Param("provider")
    model := c.Param("model")
    features := llm.DetectModelCapabilities(provider, model)
    c.JSON(http.StatusOK, gin.H{
        "provider": provider,
        "model": model,
        "features": features,
    })
})
```

---

### 7. 前端界面层 (`web-ui/src/components/`)

#### 7.1 AgentConfig.vue
**文件**：`web-ui/src/components/AgentConfig.vue`

**改造内容**：
- [ ] 添加 Agent Mode 显示（只读）
- [ ] 根据模型能力显示当前模式

**代码位置**：
```vue
<!-- 在表单中添加 -->
<el-form-item label="Agent Mode">
  <el-tag :type="agentMode === 'function_call' ? 'success' : 'info'">
    {{ agentMode === 'function_call' ? 'Function Calling' : 'Prompt-based' }}
  </el-tag>
  <el-tooltip content="根据模型能力自动选择，不可手动修改">
    <el-icon><QuestionFilled /></el-icon>
  </el-tooltip>
</el-form-item>
```

#### 7.2 新增工具函数
**文件**：`web-ui/src/utils/model-capabilities.js`（新建）

**改造内容**：
- [ ] 创建模型能力检测工具函数

**代码内容**：
```javascript
// 从 API 获取模型能力
export async function getModelCapabilities(provider, model) {
  const response = await fetch(`/api/models/${provider}/${model}/capabilities`)
  return response.json()
}

// 检查是否支持 Function Calling
export function supportsFunctionCall(features) {
  return features && features.some(f => 
    ['tool-call', 'multi-tool-call', 'stream-tool-call'].includes(f)
  )
}
```

#### 7.3 ConfigPanel.vue（可选）
**文件**：`web-ui/src/components/ConfigPanel.vue`

**改造内容**：
- [ ] 可选：在选择 LLM 时显示 Agent Mode 提示

---

## 💾 需要存储的数据清单

### 1. Agent Strategy（必须存储）⭐

#### 存储位置
- **表**：`agents`
- **字段**：`strategy VARCHAR(50)`
- **值**：`"function_call"` | `"prompt_based"` | `NULL`

#### 数据库变更
```sql
-- 添加字段
ALTER TABLE agents 
ADD COLUMN strategy VARCHAR(50) DEFAULT NULL 
COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)';

-- 添加索引
CREATE INDEX idx_strategy ON agents(strategy);
```

#### 存储逻辑
- **首次使用**：检测模型能力 → 保存 Strategy → 使用
- **后续使用**：直接读取 Strategy → 使用
- **策略变化**：清空 Strategy → 重新检测 → 保存

#### 代码修改
- `internal/web/types/types.go`：添加 `Strategy` 字段
- `internal/web/store/mysql_store.go`：读取和保存 `strategy`
- `internal/web/chat/orchestrator.go`：检测和保存逻辑

---

### 2. 模型能力映射（可选存储）⚠️

#### 存储位置
- **新表**：`model_capabilities`（可选）

#### 建议
- **阶段 1**：不存储，使用硬编码映射表
- **阶段 2**：如果需要用户自定义，再添加此表

---

## ⚠️ 其他需要注意的事项

### 1. 向后兼容性

#### 1.1 数据库兼容
- [ ] 现有 Agent 的 `strategy` 字段为 `NULL`，需要兼容处理
- [ ] 数据库迁移脚本：如果表已存在，使用 `ALTER TABLE` 添加字段
- [ ] 检查字段是否存在：避免重复添加

**代码位置**：
```go
// internal/web/store/mysql_store.go
func (m *MySQLStore) ensureTables() error {
    // 检查 strategy 字段是否存在
    // 如果不存在，添加字段
    // 如果已存在，跳过
}
```

#### 1.2 代码兼容
- [ ] 保留 `Chat()` 方法（Prompt-based 使用）
- [ ] 保留 `parseLLMResponse()` 方法（后备）
- [ ] 保留 `buildSystemPrompt()` 方法（Prompt-based 使用）
- [ ] `Strategy` 为 `NULL` 时，自动检测（向后兼容）

---

### 2. 错误处理

#### 2.1 Function Calling 失败处理
- [ ] 如果 `ChatWithTools()` 失败，自动回退到 Prompt-based
- [ ] 记录错误日志，便于排查

**代码位置**：
```go
// internal/web/chat/orchestrator.go
func (o *Orchestrator) chatWithFunctionCalling(...) {
    response, err := o.llmClient.ChatWithTools(messages, tools)
    if err != nil {
        log.Printf("[Orchestrator] Function Calling failed, fallback to Prompt-based: %v", err)
        return o.chatWithPromptBased(...)  // 自动回退
    }
}
```

#### 2.2 工具格式转换错误
- [ ] JSON Schema 转换失败处理
- [ ] 工具参数验证

---

### 3. 性能优化

#### 3.1 缓存策略
- [ ] Agent Strategy 缓存（内存）
- [ ] 模型能力检测结果缓存（可选）

**代码位置**：
```go
// internal/web/chat/orchestrator.go
var strategyCache = sync.Map{}  // 内存缓存

func (o *Orchestrator) getAgentStrategy(agentID string) string {
    // 1. 从缓存读取
    // 2. 从数据库读取
    // 3. 检测并保存
}
```

#### 3.2 避免重复检测
- [ ] Strategy 已存储时，直接使用
- [ ] 只在首次使用或 Strategy 为 NULL 时检测

---

### 4. 日志和监控

#### 4.1 关键日志点
- [ ] 模式选择日志：记录使用的模式（Function Calling 或 Prompt-based）
- [ ] 策略检测日志：记录检测结果
- [ ] 回退日志：记录 Function Calling 失败回退到 Prompt-based

**代码位置**：
```go
// internal/web/chat/orchestrator.go
log.Printf("[Orchestrator] Agent %s using strategy: %s", agent.Name, agent.Strategy)
log.Printf("[Orchestrator] Model %s supports Function Calling: %v", model, supports)
```

#### 4.2 监控指标
- [ ] Function Calling 使用率
- [ ] Prompt-based 使用率
- [ ] 模式切换次数
- [ ] Function Calling 失败率

---

### 5. 测试要点

#### 5.1 单元测试
- [ ] 模型能力检测逻辑
- [ ] 工具格式转换
- [ ] Function Calling 响应解析
- [ ] Strategy 存储和读取

#### 5.2 集成测试
- [ ] DashScope Function Calling 完整流程
- [ ] OpenAI Function Calling 完整流程
- [ ] Prompt-based 后备流程
- [ ] 模式自动切换
- [ ] 数据库迁移

#### 5.3 端到端测试
- [ ] 完整对话流程（Function Calling 模式）
- [ ] 完整对话流程（Prompt-based 模式）
- [ ] 工具调用成功/失败场景
- [ ] 服务重启后 Strategy 持久化

---

### 6. 文档更新

#### 6.1 代码文档
- [ ] 更新 `Client` 接口文档
- [ ] 更新 `Orchestrator` 方法文档
- [ ] 添加模型能力检测说明

#### 6.2 用户文档
- [ ] 更新 Agent 配置说明
- [ ] 说明 Agent Mode 的自动选择机制
- [ ] 说明支持的模型列表

---

### 7. 配置管理

#### 7.1 环境变量
- [ ] 可选：添加开关，强制使用某种模式（调试用）
- [ ] 可选：模型能力映射文件路径（如果使用外部文件）

#### 7.2 配置文件
- [ ] 可选：模型能力映射配置文件（替代硬编码）

---

### 8. 安全性考虑

#### 8.1 工具参数验证
- [ ] Function Calling 返回的工具参数需要验证
- [ ] 防止注入攻击

#### 8.2 权限控制
- [ ] Agent Strategy 修改权限（如果需要手动设置）

---

### 9. 部署注意事项

#### 9.1 数据库迁移
- [ ] 生产环境数据库迁移脚本
- [ ] 迁移前备份
- [ ] 回滚方案

#### 9.2 版本兼容
- [ ] 新版本与旧版本数据兼容
- [ ] 平滑升级方案

---

### 10. 性能影响评估

#### 10.1 预期改进
- ✅ Function Calling 模式：减少 JSON 解析错误，提高成功率
- ✅ 减少系统提示词长度（工具信息不在提示词中）
- ✅ 更快的工具调用识别（结构化数据）

#### 10.2 潜在风险
- ⚠️ 首次检测可能增加延迟（可缓存解决）
- ⚠️ 两种模式的代码维护成本

---

## 📊 改造工作量估算

### 核心功能（必须）
1. LLM 客户端层扩展：**3-4 小时**
2. 类型定义扩展：**0.5 小时**
3. 模型能力检测模块：**1-2 小时**
4. Orchestrator 改造：**4-6 小时**
5. 数据库存储：**1-2 小时**

**小计**：**9.5-14.5 小时**

### 完善功能（重要）
6. 工具转换逻辑：**2-3 小时**
7. 前端显示：**1-2 小时**
8. 错误处理和日志：**1-2 小时**

**小计**：**4-7 小时**

### 测试和文档
9. 单元测试：**2-3 小时**
10. 集成测试：**2-3 小时**
11. 文档更新：**1-2 小时**

**小计**：**5-8 小时**

### 总计
**预计总工作量**：**18.5-29.5 小时**（约 2.5-4 个工作日）

---

## ✅ 改造检查清单

### 阶段 1：核心功能
- [ ] LLM 客户端接口扩展
- [ ] DashScopeClient.ChatWithTools() 实现
- [ ] OpenAIClient.ChatWithTools() 实现
- [ ] 类型定义扩展（AgentConfig.Strategy）
- [ ] 模型能力检测模块
- [ ] Orchestrator 模式检测和分发
- [ ] 数据库表结构变更
- [ ] 数据库访问方法修改

### 阶段 2：完善功能
- [ ] 工具转换逻辑
- [ ] Function Calling 模式处理
- [ ] Prompt-based 模式重构
- [ ] 错误处理和回退
- [ ] 前端 Agent Mode 显示
- [ ] 日志和监控

### 阶段 3：测试和优化
- [ ] 单元测试
- [ ] 集成测试
- [ ] 端到端测试
- [ ] 性能测试
- [ ] 文档更新

### 阶段 4：部署
- [ ] 数据库迁移脚本
- [ ] 部署文档
- [ ] 回滚方案
- [ ] 监控和告警

---

## 🎯 总结

### 必须改造的模块（7个）
1. ✅ LLM 客户端层
2. ✅ 类型定义层
3. ✅ 模型能力检测模块（新建）
4. ✅ Orchestrator 层
5. ✅ 数据库存储层
6. ✅ API 路由层（可选扩展）
7. ✅ 前端界面层

### 必须存储的数据（1个）
1. ✅ Agent Strategy（`agents.strategy` 字段）

### 关键注意事项（10项）
1. ✅ 向后兼容性
2. ✅ 错误处理和回退
3. ✅ 性能优化
4. ✅ 日志和监控
5. ✅ 测试覆盖
6. ✅ 文档更新
7. ✅ 配置管理
8. ✅ 安全性
9. ✅ 部署注意事项
10. ✅ 性能影响评估

---

## 📝 下一步行动

1. **确认改造范围**：与团队确认是否全部实施
2. **制定实施计划**：按阶段逐步实施
3. **开始编码**：从阶段 1 核心功能开始
4. **持续测试**：每个阶段完成后进行测试
5. **文档同步**：及时更新文档

