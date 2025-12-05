# Kubernetes MCP Agent

一个基于 Go 语言开发的 Kubernetes Model Context Protocol (MCP) 智能体，允许大语言模型（LLM）通过标准化的 MCP 协议与 Kubernetes 集群交互。

## 功能特性

- ✅ 符合 MCP (Model Context Protocol) 规范
- ✅ 基于 JSON-RPC 2.0 协议通信
- ✅ 支持多种 Kubernetes 资源操作
- ✅ 安全的 RBAC 权限控制
- ✅ 支持集群内和集群外部署
- ✅ 完整的工具注册和调用机制
- ✅ Web API 接口（REST）
- ✅ 支持手动指定 Kubernetes API 地址

## 已实现的工具

1. **list_pods** - 列出指定命名空间中的所有 Pods
2. **get_deployment_status** - 获取 Deployment 的详细状态
3. **restart_pod** - 重启指定的 Pod
4. **get_pod_logs** - 获取 Pod 的日志
5. **list_deployments** - 列出指定命名空间中的所有 Deployments
6. **get_service_info** - 获取 Service 的详细信息

## 快速开始

### 前置要求

- Go 1.21 或更高版本
- Kubernetes 集群访问权限（kubeconfig 文件或集群内运行）
- kubectl 已配置（用于测试）

### 方式一：CLI 模式（MCP Agent）

1. **安装依赖并构建**
```bash
cd k8s-mcp
go mod download
go build -o k8s-mcp-agent ./cmd/main.go
```

2. **配置 Kubernetes 连接**

**选项 A：使用 kubeconfig（默认）**
```bash
# 确保 ~/.kube/config 已配置
kubectl get nodes
```

**选项 B：使用环境变量（手动指定 API 地址）**
```powershell
setx K8S_API_SERVER http://11.0.1.110:6443
setx K8S_API_TOKEN <your-token>
setx K8S_API_INSECURE true
```

3. **运行 Agent**
```bash
./k8s-mcp-agent
```

### 方式二：Web 服务模式（推荐）

1. **构建 Web 服务**
```bash
go build -o k8s-mcp-web.exe ./cmd/web/main.go
```

2. **配置环境变量（如需要）**
```powershell
# 创建 .env 文件或设置环境变量
K8S_API_SERVER=https://11.0.1.110:6443
K8S_API_TOKEN=<your-token>
K8S_API_INSECURE=true
MCP_WEB_ADDR=:9090
```

3. **构建前端（可选，用于 Web UI）**
```powershell
# 首次运行需要安装依赖
.\scripts\setup-frontend.ps1

# 构建前端
.\scripts\build-frontend.ps1
```

4. **启动 Web 服务**
```bash
.\bin\k8s-mcp-web.exe
```

5. **访问 Web UI**
- 打开浏览器访问：`http://localhost:9090`
- 在配置面板中设置 LLM API Key（百炼平台）
- 开始对话式 Kubernetes 管理

**或使用 API**：
```bash
# 健康检查
curl http://localhost:9090/healthz

# 配置 LLM
curl -X POST http://localhost:9090/api/config/llm \
  -H "Content-Type: application/json" \
  -d '{"provider":"bailian","model":"qwen-plus","apiKey":"your-key"}'

# 开始对话
curl -X POST http://localhost:9090/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"列出 default 命名空间的所有 Pods"}'
```

## API 接口

### 配置管理

- `GET /api/config` - 获取当前配置
- `POST /api/config/k8s` - 保存 Kubernetes 配置
- `POST /api/config/llm` - 保存 LLM 配置
- `POST /api/test-k8s` - 测试 Kubernetes 连接
- `POST /api/test-llm` - 测试 LLM 连接

### 工具相关

- `GET /api/tools` - 列出所有可用工具
- `POST /api/tools/call` - 调用指定工具

### 状态查询

- `GET /api/status` - 获取系统状态
- `GET /healthz` - 健康检查

## 项目结构

```
k8s-mcp/
├── cmd/
│   ├── main.go          # CLI 入口（MCP Agent）
│   └── web/
│       └── main.go      # Web 服务入口
├── internal/
│   ├── k8s/
│   │   ├── client.go    # Kubernetes 客户端封装
│   │   └── tools.go     # Kubernetes 操作工具函数
│   ├── server/
│   │   └── handlers.go  # MCP 工具注册和处理
│   └── web/
│       ├── api/
│       │   └── router.go    # HTTP 路由
│       ├── mcpclient/
│       │   └── client.go     # MCP 客户端封装
│       ├── store/
│       │   └── store.go      # 配置存储
│       └── types/
│           └── types.go      # 类型定义
├── pkg/
│   └── mcp/
│       ├── types.go     # MCP 协议类型定义
│       └── server.go    # MCP 服务器实现
├── deploy/
│   ├── rbac.yaml        # RBAC 配置
│   └── deployment.yaml  # Kubernetes 部署配置
├── docs/
│   └── WEB_UI_DEVELOPMENT.md  # 开发指南
├── go.mod
├── Dockerfile
├── Makefile
└── README.md
```

## LLM 配置

项目支持通过公网 LLM API（如通义千问、OpenAI）进行对话式 Kubernetes 管理。

### 快速配置通义千问

1. **获取 API Key**
   - 访问 [阿里云 DashScope 控制台](https://dashscope.console.aliyun.com/)
   - 开通服务并创建 API Key

2. **配置 LLM**
   ```powershell
   # 使用配置脚本
   .\scripts\configure-llm.ps1
   
   # 或手动配置
   POST http://localhost:9090/api/config/llm
   {
     "provider": "dashscope",
     "baseUrl": "https://dashscope.aliyuncs.com/api/v1",
     "model": "qwen-plus",
     "apiKey": "your-api-key"
   }
   ```

3. **测试连接**
   ```bash
   POST http://localhost:9090/api/test-llm
   ```

4. **开始对话**
   ```bash
   POST http://localhost:9090/api/chat
   {
     "message": "列出 default 命名空间的所有 Pods"
   }
   ```

详细配置说明请参考 [docs/LLM_CONFIG.md](docs/LLM_CONFIG.md)。

## 开发指南

详细的开发指南、API 文档和扩展说明请参考 [docs/WEB_UI_DEVELOPMENT.md](docs/WEB_UI_DEVELOPMENT.md)。

## 部署到 Kubernetes

### 1. 创建 ServiceAccount 和 RBAC

```bash
kubectl apply -f deploy/rbac.yaml
```

### 2. 构建 Docker 镜像

```bash
docker build -t k8s-mcp-agent:latest .
```

### 3. 部署到集群

```bash
kubectl apply -f deploy/deployment.yaml
```

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

