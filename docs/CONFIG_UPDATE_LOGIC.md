# 配置更新逻辑说明

## 用户需求

1. **修改配置后，要保存到数据库并更新**
2. **配置参数不能硬编码，前端设置的值要保存到数据库，以数据库中的值为准**

## 实现方案

### 1. 数据库更新逻辑

#### 后端实现

**`internal/web/store/persistent.go` - `SetRemoteMCP` 方法**：
- 自动检测配置是否存在
- 如果存在 → 调用 `UpdateRemoteMCPConfig`（更新）
- 如果不存在 → 调用 `SetRemoteMCPConfig`（新增）

**`internal/web/store/mysql_store.go`**：
- `SetRemoteMCPConfig`：用于新增，如果已存在会返回错误
- `UpdateRemoteMCPConfig`：用于更新，使用 `UPDATE` 语句更新所有字段

#### API 端点

**`PUT /api/config/remote-mcp/:identifier`**：
- 检查配置是否存在
- 如果前端传入的字段为 0 或空，保留数据库中的现有值
- 如果前端传入的值不为 0 或非空，使用前端传入的值
- 调用 `SetRemoteMCP`，自动判断是更新还是新增

### 2. 配置参数管理

#### 移除硬编码默认值

**前端 (`web-ui/src/components/RemoteMCPConfig.vue`)**：
- `handleEdit`：从数据库读取配置时，如果值为 0，则显示为 0（前端表单会显示 placeholder）
- `saveRemoteMCP`：保存时，如果前端传入的值为 0，则传递 0 到后端
- `resetForm`：新增时，使用 0 作为默认值（而不是硬编码的 30 或 300）

**后端 (`internal/web/api/router.go`)**：
- PUT 请求时，如果前端传入的 `timeout` 或 `sseReadTimeout` 为 0，则保留数据库中的现有值
- 这样确保前端设置的值会更新到数据库，未设置的字段保持原值

**后端 (`internal/web/store/mysql_store.go`)**：
- `SetRemoteMCPConfig`（新增）：如果前端传入的值为 0，使用默认值（30 或 300）作为最后的保障
- `UpdateRemoteMCPConfig`（更新）：直接使用前端传入的值，不设置默认值

### 3. 数据流程

#### 更新配置流程

```
前端表单 → PUT /api/config/remote-mcp/:identifier
  ↓
检查是否存在 → 存在
  ↓
如果 timeout == 0 → 使用数据库中的现有值
如果 timeout != 0 → 使用前端传入的值
  ↓
调用 SetRemoteMCP → 检测到已存在
  ↓
调用 UpdateRemoteMCPConfig → UPDATE MySQL
  ↓
更新内存中的配置
```

#### 新增配置流程

```
前端表单 → POST /api/config/remote-mcp
  ↓
检查是否重复 → 不重复
  ↓
调用 SetRemoteMCP → 检测到不存在
  ↓
调用 SetRemoteMCPConfig → INSERT MySQL
  ↓
如果 timeout == 0 → 使用默认值（30）
如果 timeout != 0 → 使用前端传入的值
```

### 4. 关键设计点

1. **前端传入的值优先**：
   - 如果前端明确设置了值（不为 0），使用前端传入的值
   - 如果前端没有设置值（为 0），在更新时保留数据库中的现有值

2. **数据库中的值为准**：
   - 所有配置参数都从数据库读取
   - 前端表单显示数据库中的值
   - 更新时，前端传入的值会更新到数据库

3. **默认值仅作为最后保障**：
   - 仅在新增配置且前端没有传入值时使用
   - 更新配置时，不会使用默认值覆盖数据库中的值

## 测试建议

1. **测试更新配置**：
   - 编辑现有 MCP 服务
   - 修改 `timeout` 为 60
   - 保存后，检查数据库中的值是否为 60
   - 重新加载页面，确认显示为 60

2. **测试保留现有值**：
   - 编辑现有 MCP 服务
   - 不修改 `timeout`（保持原值）
   - 保存后，检查数据库中的值是否保持不变

3. **测试新增配置**：
   - 添加新的 MCP 服务
   - 设置 `timeout` 为 45
   - 保存后，检查数据库中的值是否为 45

4. **测试默认值**：
   - 添加新的 MCP 服务
   - 不设置 `timeout`（为 0）
   - 保存后，检查数据库中的值是否为默认值（30）

