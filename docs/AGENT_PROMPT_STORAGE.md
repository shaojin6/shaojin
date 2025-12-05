# Agent 提示词（SystemPrompt）存储设计

## 概述

Agent 配置中的 `SystemPrompt`（系统提示词）现在支持保存到 MySQL 数据库中，实现持久化存储和更好的数据管理。

## 存储架构

### 双重存储机制

系统采用 **MySQL 优先，文件备份** 的双重存储机制：

1. **MySQL 存储（主要）**
   - 如果 MySQL 连接可用，优先使用 MySQL 存储
   - 支持完整的 CRUD 操作
   - 支持长文本（LONGTEXT 类型）

2. **文件存储（备份）**
   - MySQL 不可用时，自动回退到文件存储
   - 文件位置：`.config/web-config.json`
   - 保持向后兼容

### 数据同步

- **读取时**：优先从 MySQL 读取，同步到内存
- **写入时**：优先写入 MySQL，失败时回退到文件
- **启动时**：从 MySQL 加载所有 Agent 配置

## 数据库设计

### agents 表结构

```sql
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mcp_server_id VARCHAR(255) NOT NULL,
    llm_id VARCHAR(255),
    system_prompt LONGTEXT,              -- 系统提示词（支持长文本）
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_mcp_server_id (mcp_server_id),
    INDEX idx_enabled (enabled),
    INDEX idx_is_default (is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

### 字段说明

- `id`: Agent 唯一标识符（主键）
- `name`: Agent 名称
- `description`: Agent 描述（可选）
- `mcp_server_id`: 关联的 MCP 服务 ID
- `llm_id`: 关联的 LLM 配置 ID（可选）
- `system_prompt`: **系统提示词（LONGTEXT，支持长文本）**
- `enabled`: 是否启用
- `is_default`: 是否为默认 Agent
- `created_at`: 创建时间（Unix 时间戳）
- `updated_at`: 更新时间（Unix 时间戳）

## 代码实现

### 1. MySQL 存储层 (`internal/web/store/mysql_store.go`)

提供 Agent 配置的 MySQL 存储实现：

- `NewMySQLStore(dsn string)`: 创建 MySQL 存储实例
- `GetAllAgents(ctx)`: 获取所有 Agent
- `GetAgent(ctx, id)`: 获取指定 Agent
- `SetAgent(ctx, agent)`: 保存或更新 Agent（包括 SystemPrompt）
- `DeleteAgent(ctx, id)`: 删除 Agent
- `GetDefaultAgent(ctx)`: 获取默认 Agent

### 2. 持久化存储层 (`internal/web/store/persistent.go`)

包装内存存储，添加 MySQL 和文件双重存储：

- `SetMySQLStore(mysqlStore)`: 设置 MySQL 存储
- `SetAgent(config)`: 保存 Agent（优先 MySQL，失败回退文件）
- `GetAgent(id)`: 获取 Agent（优先 MySQL）
- `GetAllAgents()`: 获取所有 Agent（优先 MySQL）

### 3. 路由层 (`internal/web/api/router.go`)

在服务启动时初始化 MySQL 存储：

```go
// 初始化 MySQL 存储（用于 Agent 配置，包括 SystemPrompt）
mysqlStore, err := store.NewMySQLStore(mysqlDSN)
if err != nil {
    log.Printf("Warning: Failed to connect to MySQL store for agents: %v", err)
} else {
    log.Printf("MySQL store for agents connected.")
    cfgStore.SetMySQLStore(mysqlStore)
}
```

## 使用方式

### 1. 配置 MySQL 连接

通过环境变量配置 MySQL 连接信息：

```bash
MYSQL_HOST=11.0.1.110
MYSQL_PORT=30306
MYSQL_USER=root
MYSQL_PASSWORD=canxixi
MYSQL_DB=mcp
```

或使用完整的 DSN：

```bash
MYSQL_DSN=root:canxixi@tcp(11.0.1.110:30306)/mcp?parseTime=true&charset=utf8mb4&loc=Local
```

### 2. 保存提示词

通过 Web 界面或 API 保存 Agent 配置时，`SystemPrompt` 会自动保存到 MySQL：

**Web 界面：**
1. 打开"配置管理" → "智能体配置"
2. 编辑 Agent
3. 在"系统提示词（编排）"中输入提示词
4. 点击"保存"

**API：**
```bash
POST /api/config/agents
Content-Type: application/json

{
  "id": "agent-xxx",
  "name": "My Agent",
  "systemPrompt": "你的系统提示词内容...",
  ...
}
```

### 3. 读取提示词

系统会自动从 MySQL 读取提示词：

- 启动时：从 MySQL 加载所有 Agent 配置
- 查询时：优先从 MySQL 读取
- 使用时会自动应用到 LLM 调用

## 数据迁移

### 从文件迁移到 MySQL

如果需要将现有的文件存储数据迁移到 MySQL，可以调用迁移方法：

```go
// 在 router.go 中取消注释以下代码
mysqlStore.MigrateFromFileStore(cfgStore)
```

**注意**：这是一次性操作，迁移完成后应注释掉，避免重复迁移。

## 优势

1. **持久化存储**：提示词保存在数据库中，不会丢失
2. **支持长文本**：LONGTEXT 类型支持超长提示词
3. **高可用性**：MySQL 不可用时自动回退到文件存储
4. **易于管理**：可以通过 SQL 直接查询和管理提示词
5. **性能优化**：数据库索引提高查询效率
6. **向后兼容**：保持与文件存储的兼容性

## 注意事项

1. **MySQL 连接**：确保 MySQL 服务正常运行且可访问
2. **字符编码**：使用 `utf8mb4` 编码支持完整的 Unicode 字符
3. **数据备份**：建议定期备份 MySQL 数据库
4. **权限控制**：确保数据库用户有足够的权限（CREATE TABLE, INSERT, UPDATE, SELECT, DELETE）
5. **连接池**：系统使用连接池管理数据库连接，避免连接泄漏

## 故障排查

### 问题1：提示词没有保存到 MySQL

**检查：**
1. MySQL 连接是否正常（查看日志）
2. 数据库表是否创建成功
3. 是否有写入权限

**日志示例：**
```
[MySQLStore] Agents table ensured
[MySQLStore] Agent saved: ID=agent-xxx, Name=My Agent, SystemPrompt length=1234
```

### 问题2：启动时没有从 MySQL 加载

**检查：**
1. MySQL 连接是否成功
2. 查看日志中的加载信息

**日志示例：**
```
[PersistentStore] Loaded 2 agents from MySQL
```

### 问题3：MySQL 不可用时系统无法工作

**解决：**
- 系统会自动回退到文件存储
- 检查 MySQL 连接配置
- 查看日志中的警告信息

## 相关文档

- [Agent 配置指南](../docs/INTEGRATION_GUIDE.md)
- [MySQL 配置说明](../README.md)
- [API 文档](../docs/WEB_UI_DEVELOPMENT.md)

