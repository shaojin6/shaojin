# 工具刷新后数据库更新机制

## 问题

当通过 SQL 清空 `ansible-mcp-server` 的工具后，在 Web UI 中点击"刷新远程工具"时，获取到的正确工具会更新到数据库中吗？

## 答案

**是的，会更新到数据库中。**

## 更新流程

### 1. 用户操作
在 Web UI 中点击"刷新远程工具"按钮

### 2. API 调用
前端调用：`GET /api/config/remote-mcp/ansible-mcp-server/tools?refresh=true`

### 3. 后端处理流程

#### 步骤1：从远程 MCP 服务器获取工具
```go
// internal/web/api/router.go:892-922
remoteClient, err := mcpclient.NewRemoteClient(...)
tools := remoteClient.ListTools()
```

#### 步骤2：更新缓存
```go
// internal/web/api/router.go:924-928
if toolsCache != nil && len(tools) > 0 {
    toolsCache.SetTools(ctx, identifier, tools, 24*time.Hour)
}
```

#### 步骤3：DBCache 保存到数据库
```go
// internal/web/cache/db_cache.go:68-105
func (d *DBCache) SetTools(ctx context.Context, identifier string, tools []mcp.Tool, ttl time.Duration) error {
    // 1. 获取 MCP 配置
    mcpConfig := d.store.GetRemoteMCP(identifier)
    
    // 2. 转换工具格式
    cachedTools := make([]types.Tool, 0, len(tools))
    for _, tool := range tools {
        // 转换逻辑...
        cachedTools = append(cachedTools, types.Tool{...})
    }
    
    // 3. 更新配置对象
    mcpConfig.Tools = cachedTools
    mcpConfig.ToolsLastUpdate = time.Now().Unix()
    
    // 4. 保存到数据库（调用 SetRemoteMCP）
    d.store.SetRemoteMCP(*mcpConfig)
    
    return nil
}
```

#### 步骤4：PersistentStore 更新 MySQL
```go
// internal/web/store/persistent.go:538-578
func (ps *PersistentStore) SetRemoteMCP(config types.RemoteMCPConfig) {
    // 1. 保存到内存
    ps.store.SetRemoteMCP(config)
    
    // 2. 检查是否已存在
    existing, err := ps.mysqlStore.GetRemoteMCPConfig(ctx, config.ServerID)
    
    if existing != nil {
        // 3. 已存在，使用更新方法
        ps.mysqlStore.UpdateRemoteMCPConfig(ctx, config)
    } else {
        // 4. 不存在，使用新增方法
        ps.mysqlStore.SetRemoteMCPConfig(ctx, config)
    }
}
```

#### 步骤5：MySQLStore 执行 SQL UPDATE
```go
// internal/web/store/mysql_store.go:288-352
func (m *MySQLStore) UpdateRemoteMCPConfig(ctx context.Context, config types.RemoteMCPConfig) error {
    // 序列化 tools 为 JSON
    toolsJSON := sql.NullString{}
    if config.Tools != nil && len(config.Tools) > 0 {
        toolsBytes, err := json.Marshal(config.Tools)
        toolsJSON = sql.NullString{String: string(toolsBytes), Valid: true}
    }
    
    // 执行 UPDATE
    _, err := m.db.ExecContext(queryCtx, `
        UPDATE remote_mcp_configs SET
            ...
            tools = ?,
            tools_last_update = ?,
            ...
        WHERE server_id = ?
    `, ..., toolsJSON, config.ToolsLastUpdate, ..., config.ServerID)
}
```

## 数据库更新内容

更新 `remote_mcp_configs` 表中的以下字段：
- `tools`: JSON 格式的工具列表
- `tools_last_update`: 工具最后更新时间戳

## 验证方法

### 方法1：通过 SQL 查询
```sql
-- 查看更新后的工具
SELECT 
    server_id, 
    name,
    JSON_LENGTH(tools) as tool_count,
    tools_last_update,
    FROM_UNIXTIME(tools_last_update) as last_update_time
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';
```

### 方法2：通过 API 查询
```bash
# 获取工具列表（会从数据库读取）
curl http://localhost:9090/api/config/remote-mcp/ansible-mcp-server/tools
```

### 方法3：查看服务日志
刷新工具时，应该看到：
```
[MySQLStore] Remote MCP config updated: ServerID=ansible-mcp-server, Name=ansible-mcp-server, Headers count=X
```

## 总结

1. **SQL 清空工具**：`UPDATE remote_mcp_configs SET tools = NULL WHERE server_id = 'ansible-mcp-server'`
2. **Web UI 刷新工具**：点击"刷新远程工具"按钮
3. **系统自动更新数据库**：
   - 从远程 MCP 服务器获取正确的工具（8 个 Ansible 工具）
   - 通过 `DBCache.SetTools()` 保存到 `remote_mcp_configs` 表的 `tools` 字段
   - 更新 `tools_last_update` 时间戳

**因此，不需要手动更新数据库，系统会自动将刷新后的工具保存到数据库中。**

