# 重启服务说明 - MCP 可扩展性功能

## 需要重启的原因

本次修改涉及：

### 后端代码修改
1. ✅ **结构体定义**：在 `RemoteMCPConfig` 中添加了：
   - `ConnectionStrategy` 字段（连接策略）
   - `SessionPathPattern` 字段（Session 路径模式）

2. ✅ **核心逻辑**：实现了自动探测机制
   - `discoverToolsWithStrategy` 函数
   - 支持多种连接策略（auto/dify-sse/streamable-http/http）

3. ✅ **配置传递**：更新了所有使用 `RemoteMCPConfig` 的地方

## 重启步骤

### 1. 重新编译后端

```powershell
# 编译后端
go build -o .\bin\k8s-mcp-web.exe .\cmd\web\main.go
```

### 2. 重启后端服务

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

✅ **自动兼容**：新增字段都是可选的（`omitempty`），现有数据不受影响

- 如果数据库表已存在，新字段会自动添加（如果支持自动迁移）
- 如果数据库不支持自动迁移，新字段会使用默认值（空字符串）

**无需手动执行 SQL**，系统会自动处理。

## 验证

重启后，可以通过以下方式验证：

1. **检查日志**：
   - 启动服务后，查看日志中是否有自动探测相关的输出
   - 例如：`[RemoteClient] Auto-detecting connection strategy for ...`

2. **测试新功能**：
   - 打开 Web UI
   - 进入"配置管理" -> "MCP 配置"
   - 添加或编辑一个 MCP 服务
   - 在配置中可以看到新的字段（如果前端已更新）
   - 或者通过 API 直接设置 `connectionStrategy` 字段

3. **测试自动探测**：
   - 配置一个新 MCP 服务，`connectionStrategy` 设置为 `auto`（或留空）
   - 系统会自动尝试多种连接方式
   - 查看日志确认使用了哪种连接策略

## 配置变更 vs 代码变更

### 配置变更（不需要重启）
- ✅ 在 Web UI 中添加新的 MCP 服务
- ✅ 修改现有 MCP 服务的配置（BaseURL、超时时间等）
- ✅ 启用/禁用 MCP 服务

系统会自动调用 `RefreshRemoteTools()` 刷新工具列表。

### 代码变更（需要重启）
- ⚠️ 新增 `ConnectionStrategy` 字段支持
- ⚠️ 新增自动探测机制
- ⚠️ 修改连接逻辑

这些需要重新编译和重启服务。

## 快速重启命令

```powershell
# 一键重新编译
go build -o .\bin\k8s-mcp-web.exe .\cmd\web\main.go

# 然后重启服务
.\bin\k8s-mcp-web.exe
```

## 注意事项

1. ⚠️ **后端必须重新编译**，否则无法识别新的字段和逻辑
2. ✅ **现有配置不受影响**，新字段默认为空（使用默认行为）
3. ✅ **向后兼容**，即使不设置新字段，系统也能正常工作（使用默认的 auto 模式）
4. 📝 **前端更新（可选）**：如果要在 Web UI 中显示新字段，需要更新前端代码并重新构建

## 使用新功能

重启后，您可以通过以下方式使用新功能：

### 方式1：通过 API 直接配置

```json
POST /api/config/remote-mcp
{
  "name": "my-new-mcp",
  "serverId": "my-new-mcp",
  "type": "http",
  "baseUrl": "http://example.com/mcp",
  "connectionStrategy": "auto",  // 新增字段
  "timeout": 30,
  "sseReadTimeout": 300,
  "enabled": true
}
```

### 方式2：通过 Web UI（如果前端已更新）

在 MCP 配置表单中，会看到新的"连接策略"选项。

### 方式3：使用默认行为

即使不设置 `connectionStrategy`，系统也会使用 `auto` 模式（自动探测）。

