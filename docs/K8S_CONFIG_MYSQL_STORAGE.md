# K8s 配置 MySQL 存储实现

## ✅ 实现内容

### 1. 数据库表结构

在 `mysql_store.go` 的 `ensureTables()` 中添加了 `k8s_configs` 表：

```sql
CREATE TABLE IF NOT EXISTS k8s_configs (
    id VARCHAR(255) NOT NULL PRIMARY KEY,  -- 主键，保证唯一性
    name VARCHAR(255) NOT NULL,
    mode VARCHAR(50) NOT NULL,              -- kubeconfig / manual
    content TEXT,                           -- base64 kubeconfig
    namespace VARCHAR(255),
    server VARCHAR(500),                    -- API server
    token TEXT,                             -- Token
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

### 2. MySQL 存储方法

在 `mysql_store.go` 中实现了完整的 CRUD 方法：

- ✅ `GetAllK8sConfigs(ctx)` - 获取所有 K8s 配置
- ✅ `GetK8sConfig(ctx, id)` - 获取指定 K8s 配置
- ✅ `GetEnabledK8sConfigs(ctx)` - 获取所有启用的 K8s 配置
- ✅ `GetDefaultK8sConfig(ctx)` - 获取默认 K8s 配置
- ✅ `SetK8sConfig(ctx, config)` - 保存或更新 K8s 配置（使用 `INSERT ... ON DUPLICATE KEY UPDATE`）
- ✅ `DeleteK8sConfig(ctx, id)` - 删除 K8s 配置

### 3. 持久化存储层更新

在 `persistent.go` 中更新了方法：

#### SetK8sConfig
- ✅ 保存到内存
- ✅ **保存到 MySQL**（如果可用）
- ❌ **不再保存到文件**

#### DeleteK8sConfig
- ✅ 从内存删除
- ✅ **从 MySQL 删除**（如果可用）
- ❌ **不再保存到文件**

#### GetAllK8sConfigs / GetK8sConfig / GetEnabledK8sConfigs / GetDefaultK8sConfig
- ✅ **优先从 MySQL 读取**（如果可用）
- ✅ 同步到内存存储
- ✅ 回退到内存存储（如果 MySQL 不可用）

### 4. 启动时加载

在 `SetMySQLStore()` 中：
- ✅ 启动时从 MySQL 加载所有 K8s 配置
- ✅ 同步到内存存储

### 5. 测试逻辑修复

在 `router.go` 的 `testK8sHandler` 中：
- ✅ 从请求体读取 `id` 参数
- ✅ 根据 ID 获取对应的 K8s 配置
- ✅ 使用 `k8s.NewClientFromConfig()` 动态创建客户端
- ✅ 返回配置名称和测试结果

## 🔒 ID 唯一性保证

### 数据库层面
- ✅ **主键约束**：`id VARCHAR(255) NOT NULL PRIMARY KEY`
- ✅ **MySQL 自动保证唯一性**：重复 ID 会触发 `ON DUPLICATE KEY UPDATE`，执行更新而非插入

### 应用层面
- ✅ **ID 生成**：如果配置没有 ID，自动生成 `k8s-{timestamp}`
- ✅ **ID 验证**：`SetK8sConfig` 时检查 ID 不为空

## 📊 数据流程

### 保存流程
```
前端提交 → API 路由 → PersistentStore.SetK8sConfig()
  → 保存到内存 (Store)
  → 保存到 MySQL (MySQLStore)
  → 完成（不再保存到文件）
```

### 读取流程
```
API 请求 → PersistentStore.GetK8sConfig(id)
  → 优先从 MySQL 读取
  → 同步到内存
  → 返回配置
```

### 测试流程
```
前端点击测试 → API: POST /api/test-k8s {id: "xxx"}
  → testK8sHandler 读取 id
  → GetK8sConfig(id) 从 MySQL 获取配置
  → NewClientFromConfig(config) 创建客户端
  → 测试连接
  → 返回结果（包含配置名称）
```

## ✅ 解决的问题

1. **配置串了的问题** ✅
   - 测试时使用正确的配置 ID
   - 每个配置都有唯一的 ID（数据库主键保证）

2. **数据持久化** ✅
   - 配置保存到 MySQL 数据库
   - 重启后自动从 MySQL 加载

3. **不再保存到文件** ✅
   - 移除了 `ps.saveToFile()` 调用
   - 配置只保存在 MySQL 和内存中

## 🎯 使用说明

### 创建/更新配置
- 前端提交配置 → 自动保存到 MySQL
- ID 由前端生成或系统自动生成
- 如果 ID 已存在，执行更新操作

### 测试配置
- 点击"测试"按钮 → 传递配置 ID
- 后端根据 ID 获取配置 → 创建客户端 → 测试连接
- 返回结果包含配置名称，便于区分

### 删除配置
- 点击"删除"按钮 → 从 MySQL 和内存中删除
- 不会保存到文件

## ⚠️ 注意事项

1. **ID 唯一性**：确保每个配置都有唯一的 ID
2. **MySQL 连接**：如果 MySQL 不可用，会回退到内存存储
3. **数据迁移**：现有文件中的配置需要手动迁移到 MySQL（如果需要）

---

**实现完成时间**：2024-12-09
**版本**：v0.1.7+

