# 配置删除同步说明

## 问题

之前删除 MCP 配置时，只从内存中删除了，但没有从 MySQL 数据库中删除。这导致：
- 配置在 UI 中消失了
- 但数据库中仍然存在
- 服务重启后，配置会重新出现

## 解决方案

### 1. 实现 MySQL 删除方法

在 `internal/web/store/mysql_store.go` 中实现了 `DeleteRemoteMCPConfig` 方法：

```go
func (m *MySQLStore) DeleteRemoteMCPConfig(ctx context.Context, serverID string) error {
    // 先查询是否存在，以便在日志中记录
    // 执行 DELETE 操作
    // 记录删除的行数
}
```

### 2. 更新删除逻辑

在 `internal/web/store/persistent.go` 中更新了 `DeleteRemoteMCP` 方法：

```go
func (ps *PersistentStore) DeleteRemoteMCP(identifier string) {
    // 1. 获取配置信息（用于日志）
    // 2. 从内存删除
    // 3. 从 MySQL 删除
    // 4. 记录日志
}
```

### 3. 添加详细日志

删除操作现在会打印以下日志：
- `[PersistentStore] Deleting Remote MCP config: ServerID=xxx, Name=xxx`
- `[MySQLStore] Remote MCP config deleted: ServerID=xxx, Name=xxx, RowsAffected=1`
- `[PersistentStore] Successfully deleted Remote MCP config 'xxx' (Name: xxx) from MySQL`

## 检查数据库

### 方法1：使用 MySQL 客户端

```sql
-- 检查所有 MCP 配置
SELECT server_id, name, base_url, timeout, sse_read_timeout, enabled 
FROM remote_mcp_configs 
ORDER BY server_id;

-- 检查特定配置（例如 ansible-mcp-server）
SELECT server_id, name, base_url, timeout, sse_read_timeout, enabled 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';
```

### 方法2：使用 API

```bash
# 获取所有 MCP 配置
curl http://localhost:9090/api/config/remote-mcp

# 检查特定配置是否存在
curl http://localhost:9090/api/config/remote-mcp/ansible-mcp-server
```

## 测试步骤

1. **删除配置**：
   - 在 Web UI 中删除 `ansible-mcp-server`
   - 查看服务日志，应该看到删除相关的日志

2. **检查数据库**：
   - 运行上面的 SQL 查询
   - 确认 `ansible-mcp-server` 已从数据库中删除

3. **重启服务**：
   - 重启服务
   - 确认 `ansible-mcp-server` 不会重新出现

## 注意事项

- 删除操作是**不可逆**的
- 删除前会先检查配置是否存在
- 如果 MySQL 删除失败，会在日志中记录错误，但内存中的配置仍会被删除
- 删除操作**不会**删除文件备份（因为现在只使用 MySQL）

