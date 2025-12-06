# MySQL 连接管理机制

## 概述

系统实现了统一的 MySQL 连接管理器，所有模块（配置存储、缓存等）共享同一个连接池，实现了：

- ✅ **连接复用**：所有模块共享同一个连接池，避免重复连接
- ✅ **自动重连**：MySQL 挂掉后自动重连，最多重试 3 次
- ✅ **健康检查**：每分钟探测一次连接状态
- ✅ **长连接保持**：连接成功后保持长连接（30分钟）
- ✅ **故障恢复**：连接恢复后自动恢复服务

## 架构设计

### 连接管理器（单例模式）

所有 MySQL 操作都通过 `MySQLConnectionManager` 单例进行：

```go
// 获取连接管理器（单例）
connManager := store.GetMySQLConnectionManager(dsn)

// 获取数据库连接（带重连机制）
db, err := connManager.GetDB()
```

### 连接池配置

- **连接最大生存时间**：30 分钟（保持长连接）
- **最大空闲连接数**：5
- **最大打开连接数**：20

### 重连机制

1. **重连触发条件**：
   - 每次操作前检查连接状态
   - 每分钟自动探测一次连接
   - 连接失效时自动触发重连

2. **重连策略**：
   - **最大重连次数**：3 次
   - **重连间隔**：每分钟探测一次
   - **重连超时**：每次重连尝试 5 秒超时

3. **重连流程**：
   ```
   连接失效 → 关闭旧连接 → 尝试重连（最多3次）→ 连接成功 → 保持长连接
   ```

## 使用方式

### 1. 配置存储（MySQLStore）

```go
// 创建 MySQL 存储（自动使用共享连接管理器）
mysqlStore, err := store.NewMySQLStore(mysqlDSN)
if err != nil {
    log.Printf("Warning: Failed to connect to MySQL store: %v", err)
} else {
    log.Printf("MySQL store connected.")
    cfgStore.SetMySQLStore(mysqlStore)
    // 重连循环已自动启动
}
```

### 2. 缓存（MySQLCache）

```go
// 创建 MySQL 缓存（自动使用共享连接管理器）
mysqlCache, err := cache.NewMySQLCache(mysqlDSN, fileCache)
if err != nil {
    log.Printf("Warning: Failed to connect to MySQL cache: %v", err)
} else {
    log.Printf("MySQL cache connected.")
    // 使用共享连接管理器，不会创建新连接
}
```

### 3. 所有模块共享连接

由于使用单例模式，即使多个模块都调用 `GetMySQLConnectionManager(dsn)`，也只会创建一个连接管理器实例，所有模块共享同一个连接池。

## 重连机制详解

### 自动重连循环

启动服务后，连接管理器会在后台启动一个重连循环：

```go
// 在 NewMySQLStore 中自动启动
connManager.StartReconnectLoop()
```

重连循环每分钟执行一次：

1. **检查连接状态**：Ping 数据库
2. **连接正常**：继续等待下一分钟
3. **连接失效**：触发重连流程

### 重连流程

```
1. 检测到连接失效
   ↓
2. 关闭旧连接
   ↓
3. 尝试重连（最多3次）
   ├─ 第1次尝试 → 失败 → 等待5秒
   ├─ 第2次尝试 → 失败 → 等待5秒
   └─ 第3次尝试 → 成功/失败
   ↓
4. 连接成功 → 更新状态 → 保持长连接
   连接失败 → 等待下一分钟再次尝试
```

### 操作时重连

每次数据库操作前，都会检查连接状态：

```go
func (m *MySQLStore) getDB() (*sql.DB, error) {
    return m.connManager.GetDB() // 自动检查并重连
}
```

如果连接失效，`GetDB()` 会自动尝试重连。

## 配置参数

### 重连配置（可在 `mysql_conn.go` 中调整）

```go
const (
    maxReconnectAttempts = 3              // 最大重连次数
    reconnectInterval    = 1 * time.Minute // 重连间隔（每分钟探测一次）
    pingTimeout          = 5 * time.Second // Ping 超时时间
    initialPingTimeout   = 5 * time.Second // 初始连接 Ping 超时
)
```

### 连接池配置

```go
db.SetConnMaxLifetime(30 * time.Minute) // 连接最大生存时间
db.SetMaxIdleConns(5)                   // 最大空闲连接数
db.SetMaxOpenConns(20)                  // 最大打开连接数
```

## 日志监控

### 连接成功日志

```
[MySQLManager] Attempting to connect to MySQL...
[MySQLManager] MySQL connection established successfully
[MySQLStore] MySQL store initialized with connection manager
```

### 重连日志

```
[MySQLManager] Connection check failed: <error>, attempting to reconnect...
[MySQLManager] Reconnect attempt 1/3
[MySQLManager] Reconnected successfully on attempt 1
```

### 重连失败日志

```
[MySQLManager] Reconnect attempt 1/3
[MySQLManager] Reconnect attempt 1 failed: <error>
[MySQLManager] Reconnect attempt 2/3
[MySQLManager] Reconnect attempt 2 failed: <error>
[MySQLManager] Reconnect attempt 3/3
[MySQLManager] Reconnect attempt 3 failed: <error>
[MySQLManager] Failed to reconnect after 3 attempts, will retry in next cycle
```

## 故障处理

### MySQL 挂掉时的行为

1. **操作时检测**：每次数据库操作前检查连接
2. **自动重连**：连接失效时自动尝试重连（最多3次）
3. **回退机制**：重连失败时，操作会失败，系统自动回退到文件存储
4. **后台恢复**：每分钟后台探测一次，连接恢复后自动恢复服务

### 多实例部署

- 所有实例共享同一个 MySQL 数据库
- 每个实例有独立的连接管理器（但连接到同一个数据库）
- 连接池配置确保不会创建过多连接

## 最佳实践

1. **监控连接状态**：
   ```go
   if mysqlStore.IsConnected() {
       // MySQL 可用
   } else {
       // MySQL 不可用，使用文件存储
   }
   ```

2. **错误处理**：
   ```go
   db, err := m.getDB()
   if err != nil {
       // 连接失败，回退到文件存储
       return fallbackToFile()
   }
   ```

3. **避免重复启动重连循环**：
   - 重连循环在 `NewMySQLStore` 中自动启动
   - 不需要手动调用 `StartReconnectLoop()`

## 注意事项

1. **单例模式**：连接管理器是单例，所有模块共享
2. **连接复用**：不要直接创建新的 `sql.DB`，始终通过连接管理器获取
3. **长连接保持**：连接成功后保持 30 分钟，避免频繁创建连接
4. **重连限制**：每次重连最多尝试 3 次，避免过度重试
5. **资源清理**：应用退出时，连接管理器会自动关闭连接

## 相关文档

- [统一 MySQL 存储架构](./UNIFIED_MYSQL_STORAGE.md)
- [配置管理指南](./CONFIG.md)

