# 智能 Agent 配置引用 MCP 配置的方式说明

## 引用方式

智能 Agent 配置通过 **`MCPServerID` 字段**引用 MCP 配置。

### 数据结构

```go
// AgentConfig 智能体配置
type AgentConfig struct {
    ID           string `json:"id"`
    Name         string `json:"name"`
    Description  string `json:"description,omitempty"`
    MCPServerID  string `json:"mcpServerId"`  // ← 这里引用 MCP 配置
    LLMID        string `json:"llmId,omitempty"`
    SystemPrompt string `json:"systemPrompt,omitempty"`
    Enabled      bool   `json:"enabled"`
    IsDefault    bool   `json:"isDefault"`
    CreatedAt    int64  `json:"createdAt,omitempty"`
    UpdatedAt    int64  `json:"updatedAt,omitempty"`
}

// RemoteMCPConfig 远程 MCP 服务配置
type RemoteMCPConfig struct {
    Name            string            `json:"name"`      // 显示名称（如 "test"）
    ServerID        string            `json:"serverId"`  // ← 唯一标识符（如 "test-mcp"）
    Type            string            `json:"type"`
    BaseURL         string            `json:"baseUrl"`
    // ... 其他字段
}
```

## 引用关系

### 1. 引用字段
- **Agent 配置中的字段**：`MCPServerID`（JSON 字段名：`mcpServerId`）
- **MCP 配置中的字段**：`ServerID`（JSON 字段名：`serverId`）

### 2. 匹配规则
- Agent 的 `MCPServerID` 必须**完全匹配** MCP 配置的 `ServerID`
- 匹配是**严格按 ServerID**，不支持按 Name 匹配
- 例如：如果 Agent 的 `mcpServerId = "test-mcp"`，则必须找到 `serverId = "test-mcp"` 的 MCP 配置

### 3. 数据来源
- **Agent 配置**：从 MySQL 数据库的 `agents` 表加载
- **MCP 配置**：从 MySQL 数据库的 `remote_mcp_configs` 表加载

## 工作流程

### 启动时加载

```
1. 服务启动
   ↓
2. 从 MySQL 加载所有 Agent 配置
   [PersistentStore] Loaded 2 agents from MySQL
   ↓
3. 从 MySQL 加载所有 MCP 配置
   [PersistentStore] Loaded MCP config from MySQL: ServerID=test-mcp, Name=test, 1 headers
   [PersistentStore] Loaded MCP config from MySQL: ServerID=kubernetes-mcp-server, Name=kubernetes-mcp-server, 1 headers
   ↓
4. 检查 Agent 引用的 MCP 服务是否存在
   - 如果 Agent 的 MCPServerID 在已加载的 MCP 配置中找不到
   - 会尝试从 MySQL 恢复该 MCP 配置（restoreMissingMCPServicesFromAgents）
```

### 运行时使用

```
1. 用户发送消息，指定 Agent ID
   ↓
2. 系统获取 Agent 配置（从内存）
   agent = GetAgent(agentID)
   ↓
3. 使用 Agent.MCPServerID 查找对应的 MCP 配置
   mcpConfig = GetRemoteMCPByServerID(agent.MCPServerID)
   ↓
4. 获取该 MCP 服务提供的工具列表
   tools = toolManager.ListToolsForAgent(agent)
   ↓
5. 构建系统提示词，包含可用工具信息
   systemPrompt = buildSystemPrompt(agent, tools)
   ↓
6. 调用 LLM，LLM 可以使用这些工具
```

## 示例：test MCP 配置的引用

### MCP 配置（MySQL 中）
```json
{
  "name": "test",
  "serverId": "test-mcp",  // ← 这是 ServerID
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091/sse",
  "enabled": true
}
```

### Agent 配置（MySQL 中）
```json
{
  "id": "agent-xxx",
  "name": "Test Agent",
  "mcpServerId": "test-mcp",  // ← 这里引用 MCP 的 ServerID
  "enabled": true
}
```

### 匹配过程
1. Agent 的 `mcpServerId = "test-mcp"`
2. 系统查找 `serverId = "test-mcp"` 的 MCP 配置
3. 找到匹配的 MCP 配置
4. 使用该 MCP 服务的工具列表

## 关键代码位置

### 1. Agent 配置结构定义
- 文件：`internal/web/types/types.go`
- 字段：`MCPServerID string`（第 100 行）

### 2. MCP 配置结构定义
- 文件：`internal/web/types/types.go`
- 字段：`ServerID string`（第 80 行）

### 3. Agent 使用 MCP 配置
- 文件：`internal/web/chat/orchestrator.go`
- 方法：`Chat()`（第 46 行）
- 调用：`toolManager.ListToolsForAgent(agent)`（第 65 行）

### 4. 工具列表获取
- 文件：`internal/web/mcpclient/manager.go`
- 方法：`ListToolsForAgent()`（第 194 行）
- 逻辑：根据 `agent.MCPServerID` 查找对应的工具

### 5. 启动时恢复缺失的 MCP 服务
- 文件：`internal/web/store/persistent.go`
- 方法：`restoreMissingMCPServicesFromAgents()`（第 420 行）
- 逻辑：检查 Agent 引用的 MCP 服务是否存在，如果不存在则从 MySQL 恢复

## 数据库表结构

### agents 表
```sql
CREATE TABLE agents (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mcp_server_id VARCHAR(255) NOT NULL,  -- ← 引用 MCP 的 ServerID
    llm_id VARCHAR(255),
    system_prompt LONGTEXT,
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_mcp_server_id (mcp_server_id)  -- ← 索引，便于查询
);
```

### remote_mcp_configs 表
```sql
CREATE TABLE remote_mcp_configs (
    server_id VARCHAR(255) NOT NULL PRIMARY KEY,  -- ← 被 Agent 引用的字段
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    -- ... 其他字段
);
```

## 总结

1. **引用方式**：通过 `MCPServerID` 字段（Agent）匹配 `ServerID` 字段（MCP）
2. **数据来源**：两者都从 MySQL 数据库加载
3. **匹配规则**：严格按 ServerID 匹配，不支持按 Name 匹配
4. **自动恢复**：如果 Agent 引用的 MCP 服务不存在，系统会尝试从 MySQL 恢复
5. **运行时**：系统根据 Agent 的 `MCPServerID` 查找对应的 MCP 配置，获取工具列表供 LLM 使用

