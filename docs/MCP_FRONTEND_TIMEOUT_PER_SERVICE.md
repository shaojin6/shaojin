# MCP 服务前端超时配置（按服务）

## 功能概述

现在支持为每个 MCP 服务单独配置前端超时时间，实现更灵活的配置管理。

## 配置方式

### 在 Web UI 中配置

1. 进入"配置管理" -> "MCP 配置"
2. 点击"编辑"按钮编辑 MCP 服务
3. 在表单中找到"前端超时时间"字段
4. 设置超时时间（毫秒）：
   - **0**：使用全局配置（默认值）
   - **> 0**：使用该服务的特定超时时间

### 配置说明

- **前端超时时间**：前端工具列表刷新时的超时时间（毫秒）
- **默认值**：0（使用全局配置，默认 300000 毫秒，即 5 分钟）
- **建议值**：
  - 快速响应的服务：120000（2 分钟）
  - 正常服务：300000（5 分钟）
  - 慢速服务：600000（10 分钟）

## 优先级

前端超时时间的优先级：

1. **MCP 服务配置的 frontendTimeout**（如果 > 0）
2. **全局配置**（环境变量或本地存储）
3. **默认值**（300000 毫秒，5 分钟）

## 使用场景

### 场景1：不同响应速度的服务

- **ansible-mcp-server**：响应较慢，设置 `frontendTimeout: 300000`（5 分钟）
- **kubernetes-mcp-server**：响应较快，设置 `frontendTimeout: 120000`（2 分钟）

### 场景2：使用全局配置

如果所有服务都使用相同的超时时间，可以：
- 将所有服务的 `frontendTimeout` 设置为 `0`
- 通过环境变量或本地存储配置全局超时时间

### 场景3：混合配置

- 大部分服务使用全局配置（`frontendTimeout: 0`）
- 个别慢速服务使用特定配置（`frontendTimeout: 600000`）

## 技术实现

### 后端

1. **数据结构**：
   ```go
   type RemoteMCPConfig struct {
       // ... 其他字段
       FrontendTimeout int `json:"frontendTimeout,omitempty"` // 前端超时时间（毫秒），0 表示使用全局配置
   }
   ```

2. **数据库字段**：
   ```sql
   frontend_timeout INT DEFAULT 0
   ```

3. **API 响应**：
   - 在响应头中返回 `X-MCP-Frontend-Timeout`（可选）

### 前端

1. **表单字段**：
   ```vue
   <el-form-item v-if="form.type === 'http'" label="前端超时时间">
     <el-input-number
       v-model="form.frontendTimeout"
       :min="0"
       :max="600000"
       :step="1000"
       placeholder="0（使用全局配置）"
     />
   </el-form-item>
   ```

2. **API 调用逻辑**：
   ```javascript
   // 优先使用 MCP 服务的 frontendTimeout
   if (mcpConfig && mcpConfig.frontendTimeout && mcpConfig.frontendTimeout > 0) {
     timeout = mcpConfig.frontendTimeout
   } else {
     timeout = getFrontendConfig('toolsRefreshTimeout')  // 全局配置
   }
   ```

## 配置示例

### 示例1：ansible-mcp-server（慢速服务）

```json
{
  "name": "ansible-mcp-server",
  "serverId": "ansible-mcp-server",
  "baseUrl": "http://11.0.1.110:30091",
  "timeout": 60,
  "sseReadTimeout": 600,
  "frontendTimeout": 300000,  // 5 分钟
  "enabled": true
}
```

### 示例2：kubernetes-mcp-server（快速服务）

```json
{
  "name": "kubernetes-mcp-server",
  "serverId": "kubernetes-mcp-server",
  "baseUrl": "http://11.0.1.110:30080/mcp",
  "timeout": 30,
  "sseReadTimeout": 300,
  "frontendTimeout": 120000,  // 2 分钟
  "enabled": true
}
```

### 示例3：使用全局配置

```json
{
  "name": "my-mcp-server",
  "serverId": "my-mcp-server",
  "baseUrl": "http://example.com/mcp",
  "timeout": 30,
  "sseReadTimeout": 300,
  "frontendTimeout": 0,  // 使用全局配置
  "enabled": true
}
```

## 相关配置

### 全局配置

如果 MCP 服务的 `frontendTimeout` 为 0，将使用全局配置：

- **环境变量**：`VITE_TOOLS_REFRESH_TIMEOUT`（默认 300000）
- **本地存储**：`localStorage.getItem('frontendConfig')`
- **默认值**：300000 毫秒（5 分钟）

详细说明请参考：`docs/FRONTEND_TIMEOUT_CONFIG.md`

## 注意事项

1. **单位**：前端超时时间使用**毫秒**，后端超时时间使用**秒**
2. **范围**：建议设置在 120000（2 分钟）到 600000（10 分钟）之间
3. **兼容性**：如果 MCP 服务没有配置 `frontendTimeout`，将使用全局配置
4. **数据库迁移**：新字段会自动添加到现有表中，默认值为 0

## 总结

通过为每个 MCP 服务单独配置前端超时时间，可以实现：

- ✅ **灵活性**：不同服务可以使用不同的超时时间
- ✅ **兼容性**：未配置的服务使用全局配置
- ✅ **可维护性**：通过 Web UI 轻松配置和修改
- ✅ **向后兼容**：现有服务不受影响，默认使用全局配置

