# 统一 MySQL 存储架构

## 概述

系统现在采用 **统一的 MySQL 存储架构**，所有配置类型（K8s、LLM、Remote MCP、Agent）都存储在 MySQL 数据库中，实现了：

- ✅ **架构统一**：所有配置使用相同的存储机制
- ✅ **数据一致性**：支持事务，保证数据一致性
- ✅ **易于扩展**：支持多实例部署，配置共享
- ✅ **高性能**：数据库索引提升查询效率
- ✅ **向后兼容**：MySQL 不可用时自动回退到文件存储

## 存储架构

### 双重存储机制

系统采用 **MySQL 优先，文件备份** 的双重存储机制：

1. **MySQL 存储（主要）**
   - 如果 MySQL 连接可用，优先使用 MySQL 存储
   - 支持完整的 CRUD 操作
   - 支持事务和复杂查询
   - 支持多实例部署

2. **文件存储（备份）**
   - MySQL 不可用时，自动回退到文件存储
   - 文件位置：`.config/web-config.json`
   - 保持向后兼容

### 数据同步

- **读取时**：优先从 MySQL 读取，同步到内存
- **写入时**：优先写入 MySQL，失败时回退到文件
- **启动时**：智能迁移和合并流程（见下方详细说明）

## 数据库设计

### 1. k8s_configs 表

存储 Kubernetes 集群配置：

```sql
CREATE TABLE IF NOT EXISTS k8s_configs (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mode VARCHAR(50) NOT NULL,              -- kubeconfig / manual
    content TEXT,                            -- base64 kubeconfig
    namespace VARCHAR(255),
    server VARCHAR(500),                     -- API server
    token TEXT,                              -- Token
    username VARCHAR(255),
    password VARCHAR(255),
    insecure BOOLEAN DEFAULT FALSE,
    ca_file VARCHAR(500),
    ca_data TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    last_update BIGINT NOT NULL,
    INDEX idx_enabled (enabled),
    INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

### 2. llm_configs 表

存储 LLM 配置：

```sql
CREATE TABLE IF NOT EXISTS llm_configs (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,           -- dashscope, openai, ollama
    base_url VARCHAR(500),
    model VARCHAR(255) NOT NULL,
    api_key VARCHAR(500),
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    last_update BIGINT NOT NULL,
    INDEX idx_enabled (enabled),
    INDEX idx_is_default (is_default),
    INDEX idx_provider (provider)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

### 3. remote_mcp_configs 表

存储远程 MCP 服务配置：

```sql
CREATE TABLE IF NOT EXISTS remote_mcp_configs (
    server_id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,               -- http, websocket, stdio
    base_url VARCHAR(500) NOT NULL,
    icon VARCHAR(255),
    timeout INT DEFAULT 30,
    sse_read_timeout INT DEFAULT 300,
    headers TEXT,                           -- JSON 格式
    tools_endpoint VARCHAR(500),
    enabled BOOLEAN DEFAULT TRUE,
    tools TEXT,                              -- JSON 格式，缓存的工具列表
    tools_last_update BIGINT,
    last_update BIGINT NOT NULL,
    INDEX idx_enabled (enabled),
    INDEX idx_name (name),
    INDEX idx_type (type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

### 4. agents 表

存储智能 Agent 配置（包括 SystemPrompt）：

```sql
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mcp_server_id VARCHAR(255) NOT NULL,
    llm_id VARCHAR(255),
    system_prompt LONGTEXT,                  -- 系统提示词（支持长文本）
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    INDEX idx_mcp_server_id (mcp_server_id),
    INDEX idx_enabled (enabled),
    INDEX idx_is_default (is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

## 智能迁移与合并机制（最新优化）

### 启动流程

系统实现了**智能的启动流程**，自动处理数据迁移和合并：

```
启动时：
├─ NewPersistentStore() 
│  └─ loadFromFile() → 从文件加载到内存（初始数据）

连接MySQL后：
├─ SetMySQLStore(mysqlStore)
│  ├─ HasAnyData() → 检查MySQL是否有数据
│  │
│  ├─ 如果MySQL为空：
│  │  └─ MigrateFromFileStore() → 自动迁移文件数据到MySQL
│  │
│  ├─ 如果MySQL有数据：
│  │  └─ mergeFileDataWithMySQL() → 智能合并（MySQL优先，文件补充）
│  │
│  └─ loadFromMySQL() → 从MySQL加载所有配置到内存
│
└─ 设置 mysqlStore 引用

运行时：
├─ 优先使用MySQL（通过 mysqlStore）
└─ MySQL失败时回退到文件（已有实现）
```

### 核心特性

#### 1. 自动检测数据状态

系统启动时会自动检查 MySQL 中是否有数据：

```go
hasData, err := mysqlStore.HasAnyData(ctx)
```

- 如果 MySQL 为空（首次使用），自动执行迁移
- 如果 MySQL 有数据，执行智能合并

#### 2. 自动迁移（首次使用）

当 MySQL 为空时，系统会自动将文件中的数据迁移到 MySQL：

- **迁移内容**：K8s 配置、LLM 配置、Remote MCP 配置、Agent 配置
- **迁移时机**：服务启动时自动执行
- **无需手动操作**：完全自动化

**日志示例**：
```
[PersistentStore] MySQL is empty, migrating data from file store...
[MySQLStore] Starting migration from file store to MySQL...
[MySQLStore] Migrating 1 K8s configs from file store to MySQL
[MySQLStore] Migrated K8s config: k8s-xxx
[MySQLStore] Migration completed successfully
[PersistentStore] Migration from file store to MySQL completed
```

#### 3. 智能合并（MySQL 有数据时）

当 MySQL 已有数据时，系统会智能合并文件数据和 MySQL 数据：

- **MySQL 优先**：如果 MySQL 中已有某个 ID 的配置，使用 MySQL 的
- **文件补充**：如果 MySQL 中没有但文件中有，则添加到 MySQL
- **避免重复**：通过 ID 集合快速判断，避免重复添加

**合并策略**：
```
对于每个配置类型（K8s、LLM、RemoteMCP、Agent）：
1. 获取MySQL中已有的配置ID集合
2. 遍历文件中的配置
3. 如果文件中的配置ID不在MySQL中 → 添加到MySQL
4. 如果文件中的配置ID已在MySQL中 → 跳过（MySQL优先）
```

**日志示例**：
```
[PersistentStore] MySQL has data, merging with file store data (MySQL priority)...
[PersistentStore] Merged 2 K8s configs from file to MySQL
[PersistentStore] Merged 1 LLM configs from file to MySQL
[PersistentStore] Merged 1 Remote MCP configs from file to MySQL
```

#### 4. 数据加载

合并完成后，系统从 MySQL 加载所有配置到内存（MySQL 是权威数据源）：

```
[PersistentStore] Loaded 3 K8s configs from MySQL
[PersistentStore] Loaded 2 LLM configs from MySQL
[PersistentStore] Loaded 2 Remote MCP configs from MySQL
[PersistentStore] Loaded 1 agents from MySQL
```

### 优势

1. **零配置迁移**：首次使用 MySQL 时自动迁移，无需手动操作
2. **数据不丢失**：智能合并确保文件中的新配置不会丢失
3. **MySQL 优先**：MySQL 作为主要存储，保证数据一致性
4. **向后兼容**：MySQL 不可用时自动回退到文件存储
5. **完全自动化**：迁移和合并完全自动化，无需人工干预

## 代码实现

### 1. MySQL 存储层 (`internal/web/store/mysql_store.go`)

提供所有配置类型的 MySQL 存储实现：

- **K8s 配置**：
  - `GetAllK8sConfigs(ctx)`: 获取所有 K8s 配置
  - `GetK8sConfig(ctx, id)`: 获取指定 K8s 配置
  - `GetEnabledK8sConfigs(ctx)`: 获取所有启用的 K8s 配置
  - `GetDefaultK8sConfig(ctx)`: 获取默认 K8s 配置
  - `SetK8sConfig(ctx, config)`: 保存或更新 K8s 配置
  - `DeleteK8sConfig(ctx, id)`: 删除 K8s 配置

- **LLM 配置**：
  - `GetAllLLMConfigs(ctx)`: 获取所有 LLM 配置
  - `GetLLMConfig(ctx, id)`: 获取指定 LLM 配置
  - `GetDefaultLLMConfig(ctx)`: 获取默认 LLM 配置
  - `SetLLMConfig(ctx, config)`: 保存或更新 LLM 配置
  - `DeleteLLMConfig(ctx, id)`: 删除 LLM 配置

- **Remote MCP 配置**：
  - `GetAllRemoteMCPConfigs(ctx)`: 获取所有远程 MCP 配置
  - `GetRemoteMCPConfig(ctx, serverID)`: 获取指定远程 MCP 配置
  - `SetRemoteMCPConfig(ctx, config)`: 保存或更新远程 MCP 配置
  - `DeleteRemoteMCPConfig(ctx, serverID)`: 删除远程 MCP 配置

- **Agent 配置**：
  - `GetAllAgents(ctx)`: 获取所有 Agent
  - `GetAgent(ctx, id)`: 获取指定 Agent
  - `GetDefaultAgent(ctx)`: 获取默认 Agent
  - `SetAgent(ctx, agent)`: 保存或更新 Agent（包括 SystemPrompt）
  - `DeleteAgent(ctx, id)`: 删除 Agent

- **数据迁移与检查**：
  - `HasAnyData(ctx)`: 检查 MySQL 中是否有任何配置数据
  - `MigrateFromFileStore(fileStore)`: 从文件存储迁移所有配置到 MySQL

### 2. 持久化存储层 (`internal/web/store/persistent.go`)

包装内存存储，添加 MySQL 和文件双重存储：

- **SetMySQLStore(mysqlStore)**: 智能设置 MySQL 存储
  - 自动检查 MySQL 数据状态
  - 自动迁移或合并数据
  - 从 MySQL 加载配置到内存
- **loadFromMySQL()**: 从 MySQL 加载所有配置到内存
- **mergeFileDataWithMySQL()**: 智能合并文件数据和 MySQL 数据
- 所有配置类型的 Get/Set/Delete 方法都优先使用 MySQL
- MySQL 不可用时自动回退到文件存储

### 3. 路由层 (`internal/web/api/router.go`)

在服务启动时初始化 MySQL 存储：

```go
// 初始化 MySQL 存储（统一存储所有配置）
mysqlStore, err := store.NewMySQLStore(mysqlDSN)
if err != nil {
    log.Printf("Warning: Failed to connect to MySQL store: %v (will use file storage)", err)
} else {
    log.Printf("MySQL store connected (unified storage for all configs).")
    // SetMySQLStore 会自动处理：
    // 1. 检查MySQL是否有数据
    // 2. 如果MySQL为空 → 自动迁移文件数据到MySQL
    // 3. 如果MySQL有数据 → 合并文件数据和MySQL数据（MySQL优先）
    // 4. 最后从MySQL加载所有配置到内存
    cfgStore.SetMySQLStore(mysqlStore)
}
```

**注意**：迁移和合并逻辑已内置在 `SetMySQLStore()` 方法中，无需手动调用。

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

### 2. 数据迁移（自动）

**系统已实现自动迁移**，无需手动操作：

- **首次使用 MySQL**：系统启动时自动检测 MySQL 是否为空，如果为空则自动迁移文件数据
- **MySQL 已有数据**：系统会自动合并文件数据和 MySQL 数据（MySQL 优先）
- **完全自动化**：无需手动配置或执行迁移命令

**迁移流程**：
1. 服务启动时，`NewPersistentStore()` 从文件加载配置到内存
2. 连接 MySQL 后，`SetMySQLStore()` 自动检查 MySQL 数据状态
3. 如果 MySQL 为空 → 自动执行迁移
4. 如果 MySQL 有数据 → 自动执行合并
5. 最后从 MySQL 加载所有配置到内存

**无需任何手动操作**，系统会自动处理所有迁移和合并逻辑。

### 3. 配置管理

所有配置通过 Web 界面或 API 管理，系统会自动保存到 MySQL：

- **K8s 配置**：`/api/config/k8s`
- **LLM 配置**：`/api/config/llm`
- **Remote MCP 配置**：`/api/config/remote-mcp`
- **Agent 配置**：`/api/config/agents`

## 优势

### 1. 架构统一

- ✅ 所有配置使用相同的存储机制
- ✅ 代码逻辑统一，易于维护
- ✅ 减少代码复杂度

### 2. 数据一致性

- ✅ 支持数据库事务
- ✅ 避免并发写入冲突
- ✅ 数据完整性约束

### 3. 易于扩展

- ✅ 支持多实例部署
- ✅ 配置在多实例间共享
- ✅ 支持复杂查询和关联

### 4. 高性能

- ✅ 数据库索引提升查询效率
- ✅ 支持连接池
- ✅ 支持批量操作

### 5. 功能增强

- ✅ 支持配置历史（可扩展）
- ✅ 支持审计日志（可扩展）
- ✅ 支持权限控制（可扩展）
- ✅ 支持关联查询

### 6. 向后兼容

- ✅ MySQL 不可用时自动回退到文件存储
- ✅ 保持与文件存储的兼容性
- ✅ 平滑迁移

## 注意事项

1. **MySQL 连接**：确保 MySQL 服务正常运行且可访问
2. **字符编码**：使用 `utf8mb4` 编码支持完整的 Unicode 字符
3. **数据备份**：建议定期备份 MySQL 数据库
4. **权限控制**：确保数据库用户有足够的权限（CREATE TABLE, INSERT, UPDATE, SELECT, DELETE）
5. **连接池**：系统使用连接池管理数据库连接，避免连接泄漏
6. **敏感信息**：K8s Token、LLM API Key 等敏感信息存储在数据库中，建议：
   - 使用数据库加密功能
   - 限制数据库访问权限
   - 定期轮换密钥

## 故障排查

### 问题1：配置没有保存到 MySQL

**检查**：
1. MySQL 连接是否正常（查看日志）
2. 数据库表是否创建成功
3. 是否有写入权限

**日志示例**：
```
[MySQLStore] K8s configs table ensured
[MySQLStore] LLM configs table ensured
[MySQLStore] Remote MCP configs table ensured
[MySQLStore] Agents table ensured
```

### 问题2：启动时没有从 MySQL 加载

**检查**：
1. MySQL 连接是否成功
2. 查看日志中的加载信息

**日志示例**：
```
[PersistentStore] Loaded 2 K8s configs from MySQL
[PersistentStore] Loaded 1 LLM configs from MySQL
[PersistentStore] Loaded 3 Remote MCP configs from MySQL
[PersistentStore] Loaded 2 agents from MySQL
```

### 问题3：MySQL 不可用时系统无法工作

**解决**：
- 系统会自动回退到文件存储
- 检查 MySQL 连接配置
- 查看日志中的警告信息

### 问题4：多实例部署时配置不同步

**解决**：
- 确保所有实例连接到同一个 MySQL 数据库
- 配置会自动在多实例间共享
- 使用数据库事务保证一致性

## 迁移指南

### 从文件存储迁移到 MySQL（自动）

系统已实现**完全自动化的迁移流程**，无需手动操作：

1. **备份现有配置**（可选但推荐）：
   ```bash
   cp .config/web-config.json .config/web-config.json.backup
   ```

2. **配置 MySQL 连接**：
   确保环境变量中配置了正确的 MySQL 连接信息：
   ```bash
   MYSQL_HOST=11.0.1.110
   MYSQL_PORT=30306
   MYSQL_USER=root
   MYSQL_PASSWORD=canxixi
   MYSQL_DB=mcp
   ```

3. **启动服务**：
   - 服务会自动创建数据库表
   - 服务会自动检测 MySQL 数据状态
   - 如果 MySQL 为空，自动执行迁移
   - 如果 MySQL 有数据，自动执行合并

4. **验证迁移**：
   检查服务日志，确认迁移或合并成功：
   
   **迁移场景**：
   ```
   [PersistentStore] MySQL is empty, migrating data from file store...
   [MySQLStore] Migration completed successfully
   ```
   
   **合并场景**：
   ```
   [PersistentStore] MySQL has data, merging with file store data (MySQL priority)...
   [PersistentStore] Merged X configs from file to MySQL
   ```

5. **验证数据**：
   使用数据库客户端连接 MySQL，验证数据是否正确迁移：
   ```sql
   SELECT COUNT(*) FROM k8s_configs;
   SELECT COUNT(*) FROM llm_configs;
   SELECT COUNT(*) FROM remote_mcp_configs;
   SELECT COUNT(*) FROM agents;
   ```
   详细验证方法请参考：[验证 MySQL 数据指南](./VERIFY_MYSQL_DATA.md)

6. **测试功能**：
   验证所有配置功能正常，确保数据已正确迁移

### 迁移后的行为

迁移完成后，系统行为：

- **MySQL 是主要存储**：所有新配置会保存到 MySQL
- **文件作为备份**：MySQL 不可用时自动回退到文件
- **自动合并**：如果文件中有新配置，下次启动时会自动合并到 MySQL
- **数据一致性**：MySQL 优先，保证数据一致性

## 设计优化历史

### 最新优化（2024-12-05）

#### 问题
之前的实现存在数据丢失风险：
- `SetMySQLStore()` 会无条件从 MySQL 加载数据
- 如果 MySQL 为空，会覆盖内存中从文件加载的数据
- 迁移代码需要手动启用，容易遗漏

#### 解决方案
实现了**智能的启动流程**：

1. **自动检测**：启动时自动检查 MySQL 是否有数据
2. **自动迁移**：MySQL 为空时自动迁移文件数据
3. **智能合并**：MySQL 有数据时自动合并（MySQL 优先，文件补充）
4. **完全自动化**：无需手动配置或操作

#### 优势
- ✅ **零配置**：首次使用 MySQL 时自动迁移
- ✅ **数据安全**：智能合并确保数据不丢失
- ✅ **MySQL 优先**：保证数据一致性
- ✅ **向后兼容**：MySQL 不可用时自动回退

## 相关文档

- [验证 MySQL 数据指南](./VERIFY_MYSQL_DATA.md) - 如何验证数据是否正确迁移
- [MySQL 连接管理](./MYSQL_CONNECTION_MANAGEMENT.md) - MySQL 连接池和重连机制
- [Agent 提示词存储设计](./AGENT_PROMPT_STORAGE.md)
- [工具列表缓存设计](./TOOL_CACHE_DESIGN.md) - Redis+MySQL 工具列表缓存
- [API 文档](./WEB_UI_DEVELOPMENT.md)

