# 前端开发指南

## 快速开始

### 1. 安装 Node.js

确保已安装 Node.js 18+ 和 npm。

### 2. 初始化前端项目

```powershell
.\scripts\setup-frontend.ps1
```

或手动执行：

```bash
cd web-ui
npm install
```

### 3. 启动开发服务器

```bash
cd web-ui
npm run dev
```

开发服务器会在 `http://localhost:5173` 启动。

### 4. 构建生产版本

```powershell
.\scripts\build-frontend.ps1
```

或手动执行：

```bash
cd web-ui
npm run build
```

构建产物会输出到 `static` 目录，后端会自动提供静态文件服务。

## 项目结构

```
web-ui/
├── src/
│   ├── components/      # Vue 组件
│   │   ├── ConfigPanel.vue   # 配置面板
│   │   ├── StatusCard.vue    # 状态卡片
│   │   └── ChatWindow.vue   # 对话窗口
│   ├── api/             # API 调用
│   │   ├── config.js    # 配置相关 API
│   │   ├── chat.js      # 对话相关 API
│   │   └── status.js    # 状态相关 API
│   ├── App.vue          # 根组件
│   └── main.js          # 入口文件
├── index.html
├── package.json
└── vite.config.js
```

## 功能说明

### 配置面板

- **LLM 配置**：
  - 选择提供商（百炼平台、OpenAI、Ollama）
  - 配置 API 地址和模型
  - 输入 API Key（支持密码显示/隐藏）
  - 保存配置和测试连接

- **K8s 配置**：
  - 显示当前 K8s 配置（从环境变量读取）
  - 测试 K8s 连接

### 状态卡片

实时显示：
- Kubernetes 连接状态
- LLM 配置状态
- MCP 工具数量

### 对话窗口

- 发送消息与 LLM 对话
- 自动调用 Kubernetes 工具
- 显示工具调用步骤和结果
- 支持 Ctrl+Enter 快速发送

## 开发提示

1. **API 代理**：开发模式下，Vite 会自动代理 `/api` 请求到后端（`http://localhost:9090`）

2. **热更新**：修改代码后会自动刷新页面

3. **构建优化**：生产构建会自动优化和压缩代码

## 部署

构建完成后，静态文件在 `static` 目录，后端会自动提供这些文件。

访问 `http://localhost:9090` 即可使用 Web UI。

