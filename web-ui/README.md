# Kubernetes MCP Web UI

基于 Vue3 + Element Plus 的前端界面。

## 开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev
```

开发服务器会在 `http://localhost:5173` 启动，并自动代理 API 请求到后端。

## 构建

```bash
# 构建生产版本
npm run build
```

构建产物会输出到 `../static` 目录，后端会自动提供静态文件服务。

## 功能

- ✅ LLM 配置（支持百炼平台、OpenAI、Ollama）
- ✅ K8s 连接测试
- ✅ 系统状态监控
- ✅ 对话式 Kubernetes 管理
- ✅ 工具调用过程可视化

