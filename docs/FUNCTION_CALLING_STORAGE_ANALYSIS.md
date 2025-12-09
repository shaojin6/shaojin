# Function Calling 改造 - 持久化存储分析

## 需要持久化存储的数据

### 1. Agent 的 Strategy（策略）字段 ⭐ **需要存储**

#### 为什么需要存储？
- **性能优化**：避免每次对话都重新检测模型能力
- **一致性**：确保同一 Agent 在重启后使用相同的策略
- **可追溯性**：记录 Agent 使用的策略，便于调试和审计

#### 存储位置
- **表**：`agents` 表
- **字段**：`strategy VARCHAR(50)` 
- **值**：`"function_call"` 或 `"prompt_based"` 或 `NULL`（自动检测）

#### 存储逻辑
```go
// 首次使用时检测并存储
if agent.Strategy == "" {
    // 检测模型能力
    strategy := detectStrategy(agent.LLMID)
    agent.Strategy = strategy
    // 保存到数据库
    saveAgent(agent)
}

// 后续直接使用存储的策略
```

#### 数据库变更
```sql
ALTER TABLE agents 
ADD COLUMN strategy VARCHAR(50) DEFAULT NULL 
COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)';

CREATE INDEX idx_strategy ON agents(strategy);
```

---

### 2. 模型能力映射表 ⚠️ **可选存储**

#### 为什么可能需要存储？
- **用户自定义**：用户可能知道某些模型的能力，但系统未识别
- **动态更新**：模型能力可能随版本更新而变化
- **性能优化**：避免每次都查询或检测

#### 存储位置
- **新表**：`model_capabilities` 表
- **字段**：
  - `provider VARCHAR(50)` - 提供商
  - `model VARCHAR(100)` - 模型名称
  - `features JSON` - 能力列表（JSON 数组）
  - `detected_at BIGINT` - 检测时间
  - `source VARCHAR(50)` - 来源：'hardcoded' | 'api' | 'user'

#### 存储逻辑
```go
// 优先使用数据库中的映射
capabilities := getModelCapabilitiesFromDB(provider, model)
if capabilities != nil {
    return capabilities
}

// 如果不存在，使用硬编码映射
capabilities = getHardcodedCapabilities(provider, model)

// 可选：保存到数据库（如果检测到新能力）
if capabilities != nil {
    saveModelCapabilities(provider, model, capabilities)
}
```

#### 数据库表结构
```sql
CREATE TABLE IF NOT EXISTS model_capabilities (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    features JSON NOT NULL COMMENT '模型能力列表: ["tool-call", "multi-tool-call"]',
    detected_at BIGINT NOT NULL COMMENT '检测时间戳',
    source VARCHAR(50) DEFAULT 'hardcoded' COMMENT '来源: hardcoded | api | user',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE KEY uk_provider_model (provider, model),
    INDEX idx_provider (provider),
    INDEX idx_detected_at (detected_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

#### 建议
- **阶段 1**：不存储，使用硬编码映射表（简单可靠）
- **阶段 2**：如果需要用户自定义或动态检测，再添加此表

---

### 3. LLM 配置中的能力信息 ⚠️ **可选存储**

#### 为什么可能需要存储？
- **缓存检测结果**：避免每次查询都检测
- **离线使用**：即使 API 不可用，也能知道模型能力

#### 存储位置
- **表**：`llm_configs` 表（已存在）
- **字段**：`capabilities JSON` - 模型能力列表

#### 数据库变更
```sql
ALTER TABLE llm_configs 
ADD COLUMN capabilities JSON DEFAULT NULL 
COMMENT '模型能力列表: ["tool-call", "multi-tool-call"]';
```

#### 建议
- **不推荐**：LLM 配置应该只存储配置信息，能力信息应该独立管理
- **替代方案**：使用独立的 `model_capabilities` 表

---

## 不需要持久化存储的数据

### 1. 工具转换结果 ❌ **不需要存储**
- **原因**：工具列表可能变化，转换是实时计算的
- **存储位置**：内存缓存（可选）

### 2. 模型能力检测的临时结果 ❌ **不需要存储**
- **原因**：可以每次检测，或使用硬编码映射
- **存储位置**：内存缓存（可选）

### 3. Function Calling 的调用历史 ❌ **不需要存储**
- **原因**：属于运行时数据，不需要持久化
- **存储位置**：日志文件（如果需要审计）

---

## 存储方案对比

### 方案 1：最小化存储（推荐）

**只存储 Agent Strategy**

**优点：**
- ✅ 简单，改动最小
- ✅ 性能优化明显（避免重复检测）
- ✅ 数据一致性好

**缺点：**
- ⚠️ 如果模型能力变化，需要手动更新 Agent

**实现：**
```go
// AgentConfig 扩展
type AgentConfig struct {
    // ... 现有字段
    Strategy string `json:"strategy,omitempty"`  // "function_call" | "prompt_based"
}

// 数据库变更
ALTER TABLE agents ADD COLUMN strategy VARCHAR(50) DEFAULT NULL;
```

---

### 方案 2：完整存储（可选）

**存储 Agent Strategy + 模型能力映射**

**优点：**
- ✅ 支持用户自定义模型能力
- ✅ 支持动态检测和更新
- ✅ 更灵活

**缺点：**
- ⚠️ 实现复杂
- ⚠️ 需要维护额外的表
- ⚠️ 可能过度设计

**实现：**
```go
// 新增 model_capabilities 表
// Agent 策略从模型能力自动推导
```

---

## 推荐方案：方案 1（最小化存储）

### 理由
1. **简单有效**：只存储必要的 Agent Strategy
2. **性能优化**：避免每次对话都检测
3. **向后兼容**：Strategy 为 NULL 时自动检测
4. **易于维护**：不需要额外的表和数据同步

### 实现步骤

#### 1. 数据库变更
```sql
-- 添加 strategy 字段
ALTER TABLE agents 
ADD COLUMN strategy VARCHAR(50) DEFAULT NULL 
COMMENT 'Agent策略: function_call | prompt_based | NULL(自动检测)';

-- 添加索引
CREATE INDEX idx_strategy ON agents(strategy);
```

#### 2. 类型定义扩展
```go
// internal/web/types/types.go
type AgentConfig struct {
    // ... 现有字段
    Strategy string `json:"strategy,omitempty"`  // "function_call" | "prompt_based"
}
```

#### 3. 存储逻辑
```go
// internal/web/store/mysql_store.go
// SetAgent 方法中保存 strategy
INSERT INTO agents (..., strategy, ...) VALUES (..., ?, ...)

// GetAllAgents 方法中读取 strategy
SELECT ..., strategy, ... FROM agents
```

#### 4. Orchestrator 逻辑
```go
// internal/web/chat/orchestrator.go
func (o *Orchestrator) Chat(...) {
    // 1. 检查 Agent 是否有存储的策略
    if agent.Strategy != "" {
        useFunctionCalling = (agent.Strategy == "function_call")
    } else {
        // 2. 如果没有，检测模型能力
        useFunctionCalling = o.detectFunctionCallingSupport(llmConfig)
        
        // 3. 保存策略到数据库
        agent.Strategy = mapBoolToStrategy(useFunctionCalling)
        o.saveAgentStrategy(agent.ID, agent.Strategy)
    }
    
    // 4. 根据策略选择处理方式
    if useFunctionCalling {
        return o.chatWithFunctionCalling(...)
    } else {
        return o.chatWithPromptBased(...)
    }
}
```

---

## 数据迁移

### 现有 Agent 的处理
```sql
-- 现有 Agent 的 strategy 为 NULL，首次使用时自动检测并保存
-- 无需手动迁移
```

### 兼容性
- ✅ Strategy 为 NULL 时，自动检测（向后兼容）
- ✅ 支持手动设置 Strategy（如果需要）
- ✅ 模型能力变化时，可以清空 Strategy 重新检测

---

## 总结

### 必须存储
1. ✅ **Agent Strategy** - 避免重复检测，保证一致性

### 可选存储
2. ⚠️ **模型能力映射** - 如果需要用户自定义或动态检测

### 不需要存储
3. ❌ **工具转换结果** - 实时计算
4. ❌ **临时检测结果** - 可以每次检测
5. ❌ **调用历史** - 日志即可

### 推荐实现
- **阶段 1**：只存储 Agent Strategy（最小化改动）
- **阶段 2**：如果需要，再添加模型能力映射表

