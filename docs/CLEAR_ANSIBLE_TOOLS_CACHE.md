# 清空 ansible-mcp-server 工具缓存

## 问题

`ansible-mcp-server` 从缓存中加载了 `kubernetes-mcp-server` 的 21 个工具，这是因为数据库中的 `tools` 字段仍然包含错误的 Kubernetes 工具。

## 根本原因

`DBCache.GetTools()` 从数据库的 `remote_mcp_configs.tools` 字段读取工具。如果该字段包含错误的工具，就会返回错误的工具列表。

## 清理步骤

### 步骤1：清空 remote_mcp_configs.tools 字段

```sql
UPDATE remote_mcp_configs 
SET tools = NULL, tools_last_update = NULL 
WHERE server_id = 'ansible-mcp-server';
```

### 步骤2：清空 mcp_tools_cache 表

```sql
DELETE FROM mcp_tools_cache 
WHERE identifier = 'ansible-mcp-server';
```

### 步骤3：清空 Redis 缓存

使用 Redis CLI：
```bash
redis-cli -h 11.0.1.110 -p 31202 -a difyai123456 -n 1 DEL mcp:tools:ansible-mcp-server
```

或者使用 Redis 客户端：
```
DEL mcp:tools:ansible-mcp-server
```

### 步骤4：重启服务

重启服务以确保所有内存缓存被清空。

### 步骤5：在 Web UI 中刷新工具

1. 进入"配置管理" -> "MCP 配置"
2. 找到 `ansible-mcp-server`
3. 点击"刷新远程工具"按钮
4. 系统会从远程 MCP 服务器获取正确的 8 个 Ansible 工具

## 验证

清理后，验证缓存是否已清空：

```sql
-- 检查 remote_mcp_configs
SELECT server_id, name, tools IS NULL as tools_is_null 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';

-- 应该返回 tools_is_null = 1

-- 检查 mcp_tools_cache
SELECT identifier, LENGTH(tools) as tools_length 
FROM mcp_tools_cache 
WHERE identifier = 'ansible-mcp-server';

-- 应该返回空结果（没有记录）
```

## 完整的 SQL 脚本

```sql
-- 清空 ansible-mcp-server 的所有工具缓存

-- 1. 清空 remote_mcp_configs.tools
UPDATE remote_mcp_configs 
SET tools = NULL, tools_last_update = NULL 
WHERE server_id = 'ansible-mcp-server';

-- 2. 清空 mcp_tools_cache
DELETE FROM mcp_tools_cache 
WHERE identifier = 'ansible-mcp-server';

-- 3. 验证清理结果
SELECT 
    'remote_mcp_configs' as table_name,
    server_id, 
    name, 
    tools IS NULL as tools_is_null 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server'
UNION ALL
SELECT 
    'mcp_tools_cache' as table_name,
    identifier as server_id,
    '' as name,
    CASE WHEN identifier IS NULL THEN 1 ELSE 0 END as tools_is_null
FROM mcp_tools_cache 
WHERE identifier = 'ansible-mcp-server';
```

## 连接信息

- **MySQL Host**: 11.0.1.110
- **MySQL Port**: 30306
- **MySQL User**: root
- **MySQL Password**: canxixi
- **MySQL Database**: mcp

- **Redis Host**: 11.0.1.110
- **Redis Port**: 31202
- **Redis Password**: difyai123456
- **Redis DB**: 1

