# 工具缓存交叉污染问题

## 问题描述

`ansible-mcp-server` 从缓存中加载了 `kubernetes-mcp-server` 的 21 个工具，导致工具列表错误。

## 问题表现

日志显示：
```
[ToolManager] Loaded 21 tools from cache for ansible-mcp-server
Warning: Tool configuration_view already exists in client kubernetes-mcp-server, overwriting with ansible-mcp-server
Warning: Tool events_list already exists in client kubernetes-mcp-server, overwriting with ansible-mcp-server
...
```

## 根本原因

工具缓存系统使用 `identifier`（通常是 `serverId`）作为缓存键。问题可能出现在：

1. **数据库中的 tools 字段未清空**：
   - `remote_mcp_configs` 表的 `tools` 字段仍然包含错误的 Kubernetes 工具
   - `DBCache.GetTools()` 从数据库读取时返回了错误的工具

2. **Redis 缓存污染**：
   - Redis 中的 `mcp:tools:ansible-mcp-server` 键可能包含错误的工具

3. **MySQL 工具缓存表污染**：
   - `mcp_tools_cache` 表中的 `identifier = 'ansible-mcp-server'` 可能包含错误的工具

## 缓存系统架构

### 多级缓存结构

```
ToolManager
  ↓
MultiLevelCache
  ├── RedisCache (mcp:tools:{identifier})
  └── DBCache
      ├── remote_mcp_configs.tools (通过 GetRemoteMCP)
      └── MySQLCache (mcp_tools_cache 表)
```

### 缓存键生成

- **Redis**: `mcp:tools:{identifier}`
- **MySQL Cache Table**: `identifier` 字段
- **DBCache**: 使用 `identifier` 查找 `remote_mcp_configs` 配置，返回 `tools` 字段

### 问题定位

`DBCache.GetTools()` 的实现：
```go
func (d *DBCache) GetTools(ctx context.Context, identifier string) ([]mcp.Tool, error) {
    mcpConfig := d.store.GetRemoteMCP(identifier)  // 使用 identifier 查找配置
    if mcpConfig == nil {
        return nil, nil
    }
    // 返回 mcpConfig.Tools（从数据库的 tools 字段读取）
    return tools, nil
}
```

如果数据库中的 `remote_mcp_configs.tools` 字段包含错误的工具，`DBCache` 就会返回错误的工具。

## 解决方案

### 方案1：清空所有缓存（推荐）

使用提供的脚本清空所有缓存：

```powershell
.\scripts\clear-all-ansible-tools-cache.ps1
```

这会清空：
1. `remote_mcp_configs.tools` 字段
2. `mcp_tools_cache` 表中的缓存
3. Redis 缓存（需要手动执行）

### 方案2：手动 SQL 清空

```sql
-- 1. 清空 remote_mcp_configs 表的 tools 字段
UPDATE remote_mcp_configs 
SET tools = NULL, tools_last_update = NULL 
WHERE server_id = 'ansible-mcp-server';

-- 2. 清空 mcp_tools_cache 表的缓存
DELETE FROM mcp_tools_cache 
WHERE identifier = 'ansible-mcp-server';
```

### 方案3：清空 Redis 缓存

```bash
# 使用 Redis CLI
redis-cli -h 11.0.1.110 -p 31202 -a difyai123456 -n 1 DEL mcp:tools:ansible-mcp-server
```

或者使用 Redis 客户端：
```
DEL mcp:tools:ansible-mcp-server
```

## 完整清理步骤

1. **清空数据库 tools 字段**：
   ```sql
   UPDATE remote_mcp_configs 
   SET tools = NULL, tools_last_update = NULL 
   WHERE server_id = 'ansible-mcp-server';
   ```

2. **清空 MySQL 工具缓存表**：
   ```sql
   DELETE FROM mcp_tools_cache 
   WHERE identifier = 'ansible-mcp-server';
   ```

3. **清空 Redis 缓存**：
   ```bash
   redis-cli -h 11.0.1.110 -p 31202 -a difyai123456 -n 1 DEL mcp:tools:ansible-mcp-server
   ```

4. **重启服务**：
   - 确保所有内存缓存被清空

5. **在 Web UI 中刷新工具**：
   - 进入"配置管理" -> "MCP 配置"
   - 找到 `ansible-mcp-server`
   - 点击"刷新远程工具"
   - 系统会从远程 MCP 服务器获取正确的 8 个 Ansible 工具

## 验证

清理后，验证缓存是否已清空：

```sql
-- 检查 remote_mcp_configs
SELECT server_id, name, tools IS NULL as tools_is_null 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';

-- 检查 mcp_tools_cache
SELECT identifier, LENGTH(tools) as tools_length 
FROM mcp_tools_cache 
WHERE identifier = 'ansible-mcp-server';
```

两个查询都应该返回空结果或 `tools_is_null = 1`。

## 预防措施

1. **确保每个 MCP 服务有唯一的 serverId**
2. **工具缓存时使用正确的 identifier**
3. **定期检查工具列表，确保没有交叉污染**
4. **在更新工具时，确保清空旧缓存**

## 相关文件

- `scripts/clear-all-ansible-tools-cache.ps1` - 清理所有缓存的脚本
- `internal/web/cache/db_cache.go` - DBCache 实现
- `internal/web/cache/cache.go` - 多级缓存实现

