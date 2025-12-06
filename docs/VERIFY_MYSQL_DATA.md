# 验证 MySQL 数据指南

## 数据库连接信息

根据代码配置，数据库连接信息如下：

- **主机**: 11.0.1.110 (或环境变量 `MYSQL_HOST`)
- **端口**: 30306 (或环境变量 `MYSQL_PORT`)
- **用户名**: root (或环境变量 `MYSQL_USER`)
- **密码**: canxixi (或环境变量 `MYSQL_PASSWORD`)
- **数据库名**: mcp (或环境变量 `MYSQL_DB`)

## 连接数据库

### 方式1：使用 MySQL 客户端命令行

```bash
mysql -h 11.0.1.110 -P 30306 -u root -pcanxixi mcp
```

### 方式2：使用 MySQL Workbench 或其他图形化工具

- Host: 11.0.1.110
- Port: 30306
- Username: root
- Password: canxixi
- Database: mcp

## 验证查询语句

### 1. 查看所有表

```sql
SHOW TABLES;
```

应该看到以下表：
- `k8s_configs` - K8s 配置
- `llm_configs` - LLM 配置
- `remote_mcp_configs` - 远程 MCP 配置
- `agents` - Agent 配置

### 2. 检查 K8s 配置

```sql
-- 查看所有 K8s 配置
SELECT id, name, mode, server, enabled, last_update 
FROM k8s_configs 
ORDER BY last_update DESC;

-- 查看 K8s 配置总数
SELECT COUNT(*) as total FROM k8s_configs;

-- 查看详细信息（不显示敏感信息）
SELECT id, name, mode, namespace, server, insecure, enabled, 
       FROM_UNIXTIME(last_update) as update_time
FROM k8s_configs;
```

### 3. 检查 LLM 配置

```sql
-- 查看所有 LLM 配置
SELECT id, name, provider, model, base_url, enabled, is_default, last_update 
FROM llm_configs 
ORDER BY last_update DESC;

-- 查看 LLM 配置总数
SELECT COUNT(*) as total FROM llm_configs;

-- 查看详细信息
SELECT id, name, provider, model, base_url, enabled, is_default,
       FROM_UNIXTIME(last_update) as update_time
FROM llm_configs;
```

### 4. 检查远程 MCP 配置

```sql
-- 查看所有远程 MCP 配置
SELECT server_id, name, type, base_url, enabled, last_update 
FROM remote_mcp_configs 
ORDER BY last_update DESC;

-- 查看远程 MCP 配置总数
SELECT COUNT(*) as total FROM remote_mcp_configs;

-- 查看详细信息
SELECT server_id, name, type, base_url, timeout, sse_read_timeout, 
       enabled, FROM_UNIXTIME(last_update) as update_time,
       FROM_UNIXTIME(tools_last_update) as tools_update_time
FROM remote_mcp_configs;
```

### 5. 检查 Agent 配置

```sql
-- 查看所有 Agent 配置
SELECT id, name, mcp_server_id, llm_id, enabled, is_default, 
       created_at, updated_at 
FROM agents 
ORDER BY updated_at DESC;

-- 查看 Agent 配置总数
SELECT COUNT(*) as total FROM agents;

-- 查看详细信息（包括 SystemPrompt 长度）
SELECT id, name, description, mcp_server_id, llm_id, enabled, is_default,
       LENGTH(system_prompt) as prompt_length,
       FROM_UNIXTIME(created_at) as created_time,
       FROM_UNIXTIME(updated_at) as updated_time
FROM agents;
```

### 6. 综合检查 - 查看所有配置的统计信息

```sql
SELECT 
    'K8s Configs' as config_type,
    COUNT(*) as count,
    MAX(FROM_UNIXTIME(last_update)) as last_update
FROM k8s_configs
UNION ALL
SELECT 
    'LLM Configs' as config_type,
    COUNT(*) as count,
    MAX(FROM_UNIXTIME(last_update)) as last_update
FROM llm_configs
UNION ALL
SELECT 
    'Remote MCP Configs' as config_type,
    COUNT(*) as count,
    MAX(FROM_UNIXTIME(last_update)) as last_update
FROM remote_mcp_configs
UNION ALL
SELECT 
    'Agents' as config_type,
    COUNT(*) as count,
    MAX(FROM_UNIXTIME(updated_at)) as last_update
FROM agents;
```

### 7. 验证数据完整性

```sql
-- 检查是否有重复的 ID
SELECT id, COUNT(*) as count 
FROM k8s_configs 
GROUP BY id 
HAVING count > 1;

SELECT id, COUNT(*) as count 
FROM llm_configs 
GROUP BY id 
HAVING count > 1;

SELECT server_id, COUNT(*) as count 
FROM remote_mcp_configs 
GROUP BY server_id 
HAVING count > 1;

SELECT id, COUNT(*) as count 
FROM agents 
GROUP BY id 
HAVING count > 1;
```

### 8. 查看表结构（验证字段是否正确）

```sql
DESCRIBE k8s_configs;
DESCRIBE llm_configs;
DESCRIBE remote_mcp_configs;
DESCRIBE agents;
```

## 预期结果

如果迁移成功，你应该看到：

1. **k8s_configs**: 至少 1 条记录（从文件迁移的 K8s 配置）
2. **llm_configs**: 至少 1 条记录（从文件迁移的 LLM 配置）
3. **remote_mcp_configs**: 至少 1 条记录（从文件迁移的 Remote MCP 配置，如 kubernetes-mcp-server）
4. **agents**: 至少 1 条记录（从文件迁移的 Agent 配置）

## 验证迁移是否成功

### 方法1：检查记录数量

```sql
-- 快速检查所有表的记录数
SELECT 
    (SELECT COUNT(*) FROM k8s_configs) as k8s_count,
    (SELECT COUNT(*) FROM llm_configs) as llm_count,
    (SELECT COUNT(*) FROM remote_mcp_configs) as mcp_count,
    (SELECT COUNT(*) FROM agents) as agent_count;
```

### 方法2：检查特定配置是否存在

```sql
-- 检查是否有 kubernetes-mcp-server
SELECT server_id, name, base_url, enabled 
FROM remote_mcp_configs 
WHERE server_id = 'kubernetes-mcp-server' OR name = 'kubernetes-mcp-server';

-- 检查是否有默认 LLM 配置
SELECT id, name, provider, model, is_default 
FROM llm_configs 
WHERE is_default = TRUE;
```

### 方法3：查看服务日志

检查服务启动日志，应该看到类似以下信息：

```
[MySQLStore] Starting migration from file store to MySQL...
[MySQLStore] Migrating 1 K8s configs from file store to MySQL
[MySQLStore] Migrated K8s config: k8s-xxx
[MySQLStore] Migrating 1 LLM configs from file store to MySQL
[MySQLStore] Migrated LLM config: llm-default
[MySQLStore] Migrating 1 Remote MCP configs from file store to MySQL
[MySQLStore] Migrated Remote MCP config: kubernetes-mcp-server
[MySQLStore] Migration completed successfully
```

或者如果 MySQL 已有数据：

```
[PersistentStore] MySQL has data, merging with file store data (MySQL priority)...
[PersistentStore] Merged X configs from file to MySQL
[PersistentStore] Loaded X configs from MySQL
```

## 常见问题排查

### 问题1：表不存在

如果表不存在，说明表创建失败。检查服务日志中的错误信息。

### 问题2：数据为空

如果表存在但数据为空：
1. 检查文件 `.config/web-config.json` 是否有数据
2. 检查服务日志，看迁移是否执行
3. 检查是否有错误信息

### 问题3：数据不完整

如果只有部分数据：
1. 检查服务日志，看是否有迁移错误
2. 检查文件中的数据是否完整
3. 查看是否有重复 ID 冲突

## 快速验证脚本

```sql
-- 一键验证所有数据
SELECT 
    '=== 数据统计 ===' as info
UNION ALL
SELECT CONCAT('K8s 配置: ', COUNT(*)) FROM k8s_configs
UNION ALL
SELECT CONCAT('LLM 配置: ', COUNT(*)) FROM llm_configs
UNION ALL
SELECT CONCAT('Remote MCP 配置: ', COUNT(*)) FROM remote_mcp_configs
UNION ALL
SELECT CONCAT('Agent 配置: ', COUNT(*)) FROM agents;
```

