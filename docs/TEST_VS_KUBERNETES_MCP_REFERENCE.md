# test 和 kubernetes-mcp-server 引用方式对比

## 答案：引用方式完全一样 ✅

两种 MCP 配置的引用方式**完全相同**，都是通过 **`ServerID`** 字段引用。

## 实际配置对比

### test MCP 配置

从日志可以看到：
```
[PersistentStore] Loaded MCP config from MySQL: ServerID=test-mcp, Name=test, 1 headers
```

**配置详情：**
```json
{
  "name": "test",              // ← 显示名称（Name）
  "serverId": "test-mcp",     // ← 唯一标识符（ServerID）
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091/sse"
}
```

### kubernetes-mcp-server MCP 配置

从日志可以看到：
```
[PersistentStore] Loaded MCP config from MySQL: ServerID=kubernetes-mcp-server, Name=kubernetes-mcp-server, 1 headers
```

**配置详情：**
```json
{
  "name": "kubernetes-mcp-server",        // ← 显示名称（Name）
  "serverId": "kubernetes-mcp-server",    // ← 唯一标识符（ServerID）
  "type": "http",
  "baseUrl": "http://11.0.1.110:30080/mcp"
}
```

## 关键区别

| MCP 配置 | Name（显示名称） | ServerID（唯一标识符） | Name 和 ServerID 是否相同 |
|---------|----------------|---------------------|------------------------|
| **test** | `"test"` | `"test-mcp"` | ❌ **不同** |
| **kubernetes-mcp-server** | `"kubernetes-mcp-server"` | `"kubernetes-mcp-server"` | ✅ **相同** |

## 引用方式（完全相同）

### Agent 引用 test MCP

```json
{
  "id": "agent-xxx",
  "name": "Test Agent",
  "mcpServerId": "test-mcp"  // ← 引用 ServerID，不是 Name
}
```

**匹配过程：**
1. Agent 的 `mcpServerId = "test-mcp"`
2. 系统查找 `serverId = "test-mcp"` 的 MCP 配置
3. 找到匹配的 MCP 配置（Name="test", ServerID="test-mcp"）
4. 使用该 MCP 服务的工具列表

### Agent 引用 kubernetes-mcp-server MCP

```json
{
  "id": "agent-yyy",
  "name": "K8s Agent",
  "mcpServerId": "kubernetes-mcp-server"  // ← 引用 ServerID
}
```

**匹配过程：**
1. Agent 的 `mcpServerId = "kubernetes-mcp-server"`
2. 系统查找 `serverId = "kubernetes-mcp-server"` 的 MCP 配置
3. 找到匹配的 MCP 配置（Name="kubernetes-mcp-server", ServerID="kubernetes-mcp-server"）
4. 使用该 MCP 服务的工具列表

## 代码实现（完全相同）

无论是引用 `test-mcp` 还是 `kubernetes-mcp-server`，代码逻辑完全一样：

```go
// ListToolsForAgent 方法（第 195 行）
func (tm *ToolManager) ListToolsForAgent(agent *types.AgentConfig) []mcp.Tool {
    // 使用 Agent.MCPServerID 作为标识符
    identifier := agent.MCPServerID  // ← 无论是 "test-mcp" 还是 "kubernetes-mcp-server"
    
    // 查找缓存的工具
    if cached, exists := tm.cachedTools[identifier]; exists {
        return cached
    }
    
    // 查找远程客户端
    if remoteClient, exists := tm.remoteClients[identifier]; exists {
        return remoteClient.ListTools()
    }
    
    // 从缓存系统获取
    // ...
}
```

```go
// GetRemoteMCPByServerID 方法（第 215 行）
func (s *Store) GetRemoteMCPByServerID(serverID string) *types.RemoteMCPConfig {
    // 严格按 serverId 查找，无论是 "test-mcp" 还是 "kubernetes-mcp-server"
    if config, ok := s.remoteMCPs[serverID]; ok {
        return config
    }
    return nil
}
```

## 为什么看起来不一样？

### 界面显示 vs 实际引用

**界面显示：**
- 前端可能显示 MCP 的 `Name`（如 "test" 或 "kubernetes-mcp-server"）
- 这是为了用户友好，显示可读的名称

**实际引用：**
- Agent 配置中的 `mcpServerId` 字段存储的是 `ServerID`
- 系统通过 `ServerID` 匹配，不是通过 `Name`

### 示例

**用户看到的（界面）：**
```
Agent: "Test Agent"
MCP 服务器: "test"  ← 显示的是 Name
```

**实际存储的（数据库）：**
```json
{
  "mcpServerId": "test-mcp"  // ← 存储的是 ServerID，不是 "test"
}
```

**匹配过程：**
```
Agent.mcpServerId = "test-mcp"
  ↓
查找 MCP 配置：serverId = "test-mcp"
  ↓
找到：{ name: "test", serverId: "test-mcp" }
```

## 总结

| 特性 | test MCP | kubernetes-mcp-server MCP |
|------|---------|--------------------------|
| **引用字段** | `mcpServerId: "test-mcp"` | `mcpServerId: "kubernetes-mcp-server"` |
| **匹配方式** | 按 ServerID 匹配 | 按 ServerID 匹配 |
| **代码逻辑** | 完全相同 | 完全相同 |
| **唯一区别** | Name 和 ServerID 不同 | Name 和 ServerID 相同 |

## 结论

✅ **引用方式完全一样**：
- 都是通过 `ServerID` 引用
- 都使用相同的代码逻辑
- 都遵循相同的匹配规则

❌ **唯一区别**：
- `test` 的 Name 和 ServerID 不同（Name="test", ServerID="test-mcp"）
- `kubernetes-mcp-server` 的 Name 和 ServerID 相同（都是 "kubernetes-mcp-server"）

**重要提示：**
- Agent 配置中的 `mcpServerId` 字段**必须**是 MCP 的 `ServerID`，不能是 `Name`
- 即使界面上显示的是 Name（如 "test"），实际引用的是 ServerID（如 "test-mcp"）

