# 从 MySQL 恢复请求头配置

## 问题描述

在自动恢复 `kubernetes-mcp-server` 时，请求头（headers）配置丢失了。

## 原因分析

1. **自动恢复逻辑**：恢复时创建的是默认配置，`Headers` 字段被初始化为空 `map[string]string{}`
2. **数据来源**：请求头数据之前存储在 MySQL 中，但恢复逻辑没有从 MySQL 读取

## 解决方案

### 方案1：从文件备份恢复（已实现）

在恢复 MCP 服务时，优先从文件备份（`.config/web-config.json`）中读取配置，如果存在则保留所有字段（包括 headers）。

**优点**：
- 简单快速
- 保留完整配置

**缺点**：
- 如果文件备份中也没有 headers，则无法恢复

### 方案2：从 MySQL 恢复（需要实现）

如果 MySQL 中有 `remote_mcp_configs` 表，可以从 MySQL 中读取配置。

**SQL 查询示例**：
```sql
SELECT server_id, name, type, base_url, icon, timeout, sse_read_timeout, 
       frontend_timeout, headers, tools_endpoint, enabled, tools, 
       tools_last_update, last_update
FROM remote_mcp_configs
WHERE server_id = 'kubernetes-mcp-server';
```

**实现步骤**：
1. 在 `MySQLStore` 中添加 `GetRemoteMCPConfig` 方法
2. 在恢复逻辑中，先尝试从 MySQL 读取
3. 如果 MySQL 中没有，再尝试从文件备份读取
4. 如果都没有，才创建默认配置

### 方案3：手动恢复

如果 MySQL 中有数据，可以手动查询并重新配置：

1. **查询 MySQL**：
```sql
SELECT headers FROM remote_mcp_configs WHERE server_id = 'kubernetes-mcp-server';
```

2. **在 Web UI 中重新配置**：
   - 打开"配置管理" -> "MCP 配置"
   - 编辑 `kubernetes-mcp-server`
   - 添加请求头
   - 保存

## 当前实现

已实现从文件备份恢复的逻辑：
- `tryRestoreFromFile` 函数会从文件备份中读取配置
- 如果找到匹配的配置，会保留所有字段（包括 headers）
- 如果文件备份中没有，才创建默认配置

## 验证

重启服务后，检查：
1. `kubernetes-mcp-server` 是否已恢复
2. 请求头是否已恢复（如果文件备份中有）

## 如果文件备份中也没有 headers

需要：
1. 查询 MySQL 数据库，获取 headers 数据
2. 在 Web UI 中手动重新配置
3. 或者实现从 MySQL 恢复的逻辑

