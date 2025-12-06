# 前端超时配置说明

## 概述

前端工具列表加载的超时时间已配置化，支持通过多种方式配置，避免硬编码。

## 配置方式

### 方式1：环境变量（推荐用于生产环境）

在 `web-ui` 目录下创建 `.env` 或 `.env.local` 文件：

```bash
# 工具列表刷新超时时间（毫秒）
# 强制刷新工具列表时的超时时间，默认 300000（5分钟）
VITE_TOOLS_REFRESH_TIMEOUT=300000

# 工具列表加载超时时间（毫秒）
# 普通加载工具列表时的超时时间（使用缓存），默认 10000（10秒）
VITE_TOOLS_LOAD_TIMEOUT=10000
```

**优先级**：环境变量 > 本地存储 > 默认值

### 方式2：本地存储（推荐用于开发测试）

在浏览器控制台执行：

```javascript
// 设置刷新超时时间（5分钟）
localStorage.setItem('frontendConfig', JSON.stringify({
  toolsRefreshTimeout: 300000
}))

// 设置加载超时时间（10秒）
localStorage.setItem('frontendConfig', JSON.stringify({
  toolsLoadTimeout: 10000
}))

// 同时设置两个
localStorage.setItem('frontendConfig', JSON.stringify({
  toolsRefreshTimeout: 300000,
  toolsLoadTimeout: 10000
}))
```

**优先级**：环境变量 > 本地存储 > 默认值

### 方式3：使用默认值

如果不配置，将使用默认值：
- `toolsRefreshTimeout`: 300000 毫秒（5分钟）
- `toolsLoadTimeout`: 10000 毫秒（10秒）

## 配置说明

### toolsRefreshTimeout（刷新超时）

**用途**：强制刷新工具列表时的超时时间

**默认值**：300000 毫秒（5分钟）

**为什么需要这么长**：
- 需要连接远程 MCP 服务
- 响应中包含 ping 消息（心跳），每 15 秒一次
- 从测试结果看，完整响应可能需要 60-90 秒
- 考虑到网络延迟，5 分钟更安全

**建议值**：
- 最小：120000 毫秒（2分钟）
- 推荐：300000 毫秒（5分钟）
- 最大：600000 毫秒（10分钟）

### toolsLoadTimeout（加载超时）

**用途**：普通加载工具列表时的超时时间（使用缓存）

**默认值**：10000 毫秒（10秒）

**为什么这么快**：
- 使用缓存，不需要连接远程服务
- 通常很快就能返回结果

**建议值**：
- 最小：5000 毫秒（5秒）
- 推荐：10000 毫秒（10秒）
- 最大：30000 毫秒（30秒）

## 配置示例

### 示例1：生产环境配置

`.env.production`:
```bash
# 生产环境：使用较长的超时时间，确保稳定性
VITE_TOOLS_REFRESH_TIMEOUT=300000  # 5分钟
VITE_TOOLS_LOAD_TIMEOUT=10000      # 10秒
```

### 示例2：开发环境配置

`.env.development`:
```bash
# 开发环境：可以使用较短的超时时间，快速发现问题
VITE_TOOLS_REFRESH_TIMEOUT=120000  # 2分钟
VITE_TOOLS_LOAD_TIMEOUT=5000       # 5秒
```

### 示例3：测试环境配置

`.env.test`:
```bash
# 测试环境：使用中等超时时间
VITE_TOOLS_REFRESH_TIMEOUT=180000  # 3分钟
VITE_TOOLS_LOAD_TIMEOUT=8000       # 8秒
```

## 配置优先级

配置的优先级顺序：

1. **环境变量**（最高优先级）
   - `.env.local`（本地覆盖）
   - `.env.production` / `.env.development`（根据模式）
   - `.env`（通用配置）

2. **本地存储**（中等优先级）
   - `localStorage.getItem('frontendConfig')`

3. **默认值**（最低优先级）
   - `DEFAULT_CONFIG` 中定义的值

## 使用方式

### 在代码中使用

```javascript
import { getConfig, getAllConfig } from '@/config/frontend-config'

// 获取单个配置值
const refreshTimeout = getConfig('toolsRefreshTimeout')  // 300000

// 获取所有配置
const config = getAllConfig()
// {
//   toolsRefreshTimeout: 300000,
//   toolsLoadTimeout: 10000
// }
```

### 动态修改配置

```javascript
import { saveConfig, resetConfig } from '@/config/frontend-config'

// 保存配置到本地存储
saveConfig({
  toolsRefreshTimeout: 300000,
  toolsLoadTimeout: 10000
})

// 重置为默认值
resetConfig()
```

## 验证配置

### 检查当前配置

在浏览器控制台执行：

```javascript
// 检查环境变量（需要在代码中访问）
console.log('Refresh Timeout:', import.meta.env.VITE_TOOLS_REFRESH_TIMEOUT)
console.log('Load Timeout:', import.meta.env.VITE_TOOLS_LOAD_TIMEOUT)

// 检查本地存储
console.log('Local Config:', localStorage.getItem('frontendConfig'))

// 检查实际使用的配置（需要导入模块）
import { getAllConfig } from './src/config/frontend-config'
console.log('Current Config:', getAllConfig())
```

## 常见问题

### Q1: 如何知道当前使用的是哪个配置？

**A**: 配置优先级是：环境变量 > 本地存储 > 默认值。可以通过浏览器控制台检查。

### Q2: 修改配置后需要重启服务吗？

**A**: 
- **环境变量**：需要重新构建前端（`npm run build`）
- **本地存储**：不需要重启，刷新页面即可生效

### Q3: 如何为不同的 MCP 服务设置不同的超时时间？

**A**: 当前设计是全局配置。如果需要为不同服务设置不同超时，可以考虑：
1. 在 MCP 服务配置中添加 `frontendTimeout` 字段
2. 在调用 `getRemoteMCPTools` 时传入该值

### Q4: 配置不生效怎么办？

**A**: 
1. 检查环境变量名称是否正确（必须以 `VITE_` 开头）
2. 检查本地存储中是否有配置
3. 清除浏览器缓存和本地存储
4. 重新构建前端

## 相关文件

- `web-ui/src/config/frontend-config.js` - 配置管理模块
- `web-ui/src/api/config.js` - 使用配置的 API 调用
- `web-ui/.env.example` - 环境变量示例文件

## 总结

通过配置化的方式管理超时时间，具有以下优点：

1. ✅ **灵活性**：可以根据不同环境设置不同的超时时间
2. ✅ **可维护性**：不需要修改代码，只需修改配置
3. ✅ **可扩展性**：可以轻松添加其他配置项
4. ✅ **优先级清晰**：环境变量 > 本地存储 > 默认值

**推荐使用环境变量方式配置生产环境，使用本地存储方式配置开发测试环境。**

