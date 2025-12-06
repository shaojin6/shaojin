# 重启服务说明

## 需要重启的原因

本次修改涉及：

### 后端修改
1. ✅ **结构体定义**：添加了 `FrontendTimeout` 字段到 `RemoteMCPConfig`
2. ✅ **数据库表结构**：添加了 `frontend_timeout` 字段（带自动迁移）
3. ✅ **API 路由**：修改了工具列表获取接口

### 前端修改
1. ✅ **Vue 组件**：添加了前端超时配置字段
2. ✅ **API 调用**：修改了 `getRemoteMCPTools` 函数逻辑

## 重启步骤

### 1. 重新编译后端

```powershell
# 编译后端
go build -o .\bin\k8s-mcp-web.exe .\cmd\web\main.go
```

### 2. 重新构建前端

```powershell
# 进入前端目录
cd web-ui

# 构建前端
npm run build

# 返回根目录
cd ..
```

### 3. 重启后端服务

**方式1：如果使用脚本启动**
```powershell
# 停止当前服务（Ctrl+C）
# 然后重新启动
.\scripts\start-with-console.bat
```

**方式2：如果直接运行可执行文件**
```powershell
# 停止当前服务（Ctrl+C）
# 然后重新启动
.\bin\k8s-mcp-web.exe
```

## 数据库迁移

✅ **自动迁移**：数据库表结构会在服务启动时自动迁移

- 如果表不存在，会创建包含 `frontend_timeout` 字段的新表
- 如果表已存在，会自动添加 `frontend_timeout` 字段（如果不存在）

**无需手动执行 SQL**，服务启动时会自动处理。

## 验证

重启后，可以通过以下方式验证：

1. **检查数据库**：
   ```sql
   DESCRIBE remote_mcp_configs;
   -- 应该看到 frontend_timeout 字段
   ```

2. **检查前端**：
   - 打开 Web UI
   - 进入"配置管理" -> "MCP 配置"
   - 编辑任意 MCP 服务
   - 应该能看到"前端超时时间"字段

3. **测试功能**：
   - 设置某个 MCP 服务的 `frontendTimeout` 为 300000
   - 点击"刷新远程工具"
   - 应该使用 5 分钟的超时时间（而不是全局配置）

## 注意事项

1. ⚠️ **数据库迁移是自动的**，但建议在重启前备份数据库
2. ⚠️ **前端必须重新构建**，否则看不到新的配置字段
3. ⚠️ **后端必须重新编译**，否则无法识别新的字段
4. ✅ **现有配置不受影响**，`frontend_timeout` 默认为 0（使用全局配置）

## 快速重启命令

```powershell
# 一键重新编译和构建
go build -o .\bin\k8s-mcp-web.exe .\cmd\web\main.go
cd web-ui && npm run build && cd ..

# 然后重启服务
.\bin\k8s-mcp-web.exe
```

