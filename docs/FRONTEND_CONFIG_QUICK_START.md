# 前端超时配置快速开始

## 快速配置

### 方式1：使用环境变量（推荐）

在 `web-ui` 目录下创建 `.env` 文件：

```bash
# 工具列表刷新超时时间（毫秒）- 5分钟
VITE_TOOLS_REFRESH_TIMEOUT=300000

# 工具列表加载超时时间（毫秒）- 10秒
VITE_TOOLS_LOAD_TIMEOUT=10000
```

然后重新构建前端：
```bash
cd web-ui
npm run build
```

### 方式2：使用浏览器控制台（临时测试）

打开浏览器控制台（F12），执行：

```javascript
// 设置刷新超时为 5 分钟
localStorage.setItem('frontendConfig', JSON.stringify({
  toolsRefreshTimeout: 300000
}))

// 刷新页面生效
location.reload()
```

## 配置值说明

| 配置项 | 默认值 | 说明 | 建议值 |
|--------|--------|------|--------|
| `toolsRefreshTimeout` | 300000 (5分钟) | 强制刷新工具列表的超时时间 | 300000-600000 |
| `toolsLoadTimeout` | 10000 (10秒) | 普通加载工具列表的超时时间 | 5000-30000 |

## 验证配置

在浏览器控制台执行：

```javascript
// 检查当前配置
import { getAllConfig } from './src/config/frontend-config'
console.log('当前配置:', getAllConfig())
```

## 常见场景

### 场景1：ansible-mcp-server 刷新超时

**问题**：点击"刷新"按钮时，60秒超时

**解决方案**：
```bash
# 在 web-ui/.env 中设置
VITE_TOOLS_REFRESH_TIMEOUT=300000  # 5分钟
```

### 场景2：快速测试（开发环境）

**需求**：开发时快速发现超时问题

**解决方案**：
```bash
# 在 web-ui/.env.development 中设置
VITE_TOOLS_REFRESH_TIMEOUT=120000  # 2分钟
```

### 场景3：生产环境稳定性

**需求**：生产环境需要更长的超时时间

**解决方案**：
```bash
# 在 web-ui/.env.production 中设置
VITE_TOOLS_REFRESH_TIMEOUT=600000  # 10分钟
```

## 相关文档

- `docs/FRONTEND_TIMEOUT_CONFIG.md` - 详细配置说明
- `web-ui/src/config/frontend-config.js` - 配置管理模块源码

