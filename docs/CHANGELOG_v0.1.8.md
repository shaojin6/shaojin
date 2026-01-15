# 版本更新日志 v0.1.8

## 🔧 优化和修复

### ✨ 主要改进

#### 1. MySQL 连接超时优化
- ✅ 增加 MySQL DSN 连接超时时间（10s → 15s）
- ✅ 增加 MySQL 读取超时时间（30s → 60s）
- ✅ 增加 MySQL 写入超时时间（30s → 60s）
- ✅ 增加 `PingContext` 超时时间（10s → 15s）
- ✅ 增加 `ensureTables` 超时时间（30s → 60s）
- ✅ 增加 `GetAllAgents` 查询超时时间（5s → 10s）
- ✅ 优化 MySQL 缓存连接超时设置

**解决的问题**：
- 修复 MySQL 连接超时错误：`context deadline exceeded`
- 改善慢速网络环境下的数据库连接稳定性
- 提高数据库表创建和迁移的成功率

#### 2. 前端 UI 更新
- ✅ 将 "MCP 智能体" 更名为 "ChatOps 智能体"
- ✅ 更新浏览器标签页标题
- ✅ 更新主页面标题显示

#### 3. 文档更新
- ✅ 更新 `WEB_UI_DEVELOPMENT.md`，完善 Function Calling 功能说明
- ✅ 更新整体架构图，反映双模式工具调用机制
- ✅ 新增模型能力检测和自动选择策略说明

### 📝 技术细节

#### MySQL 超时配置变更

**`internal/web/api/router.go`**:
```go
// DSN 超时参数优化
timeout=15s&readTimeout=60s&writeTimeout=60s
```

**`internal/web/store/mysql_store.go`**:
- `PingContext`: 10s → 15s
- `ensureTables`: 30s → 60s
- `GetAllAgents`: 5s → 10s

**`internal/web/cache/mysql_cache.go`**:
- `PingContext`: 5s → 10s
- `ensureTable`: 5s → 10s

### 🔄 从 v0.1.7 升级

1. **停止服务**
2. **部署新代码**
3. **启动服务**（自动应用新的超时设置）
4. **验证 MySQL 连接**（检查日志确认无超时错误）

### ⚠️ 注意事项

1. **MySQL 超时设置**：如果仍然遇到超时问题，可能需要：
   - 检查 MySQL 服务器性能
   - 检查网络连接质量
   - 考虑进一步增加超时时间

2. **前端更新**：前端更新后需要：
   - 重新构建前端（`npm run build`）
   - 重启后端服务

### 🐛 修复的问题

- ✅ 修复 MySQL 连接超时导致的 `context deadline exceeded` 错误
- ✅ 修复 `ensure frontend_timeout column` 超时问题
- ✅ 修复 `create agents table` 超时问题

### 📦 包含的提交

- `cf4d89d` - docs: update README.md for v0.1.7 - add Function Calling features
- `f9f56a9` - Merge branch 'master'
- `aaa0f45` - docs: add script to fix commit 39f7aa7 encoding issue
- MySQL 超时优化相关修改
- 前端 UI 更新相关修改

---

**发布日期**：2025-01-14  
**版本号**：v0.1.8  
**主要变更**：MySQL 连接超时优化、前端 UI 更新

