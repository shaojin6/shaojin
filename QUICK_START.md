# 快速开始指南

## 完整流程（包含前端 Web UI）

### 1. 准备环境

- Go 1.21+
- Node.js 18+ 和 npm
- Kubernetes 集群访问权限

### 2. 配置 Kubernetes 连接

创建 `.env` 文件：

```env
K8S_API_SERVER=https://11.0.1.110:6443
K8S_API_TOKEN=your-k8s-token
K8S_API_INSECURE=true
MCP_WEB_ADDR=:9090
```

### 3. 构建后端

```powershell
go build -o .\bin\k8s-mcp-web.exe .\cmd\web
```

### 4. 构建前端（首次）

```powershell
# 安装依赖
.\scripts\setup-frontend.ps1

# 构建前端
.\scripts\build-frontend.ps1
```

### 5. 启动服务

```powershell
.\bin\k8s-mcp-web.exe
```

### 6. 访问 Web UI

打开浏览器访问：`http://localhost:9090`

### 7. 配置 LLM（在 Web UI 中）

1. 在左侧"配置管理"面板中，选择"LLM 配置"标签
2. 选择提供商：**百炼平台（推荐）**
3. 输入你的百炼平台 API Key：`sk-8a66058f2bb84ab69047bf3037f0e5ac`
4. 选择模型：`qwen-plus`
5. 点击"保存配置"
6. 点击"测试连接"验证配置

### 8. 开始对话

在右侧对话窗口中，输入问题，例如：
- "列出 default 命名空间的所有 Pods"
- "查看 nginx deployment 的状态"
- "重启某个 Pod"

系统会自动：
1. 理解你的问题
2. 调用相应的 Kubernetes 工具
3. 返回自然语言回答

## 仅使用 API（不构建前端）

如果不需要 Web UI，可以直接使用 API：

```powershell
# 配置 LLM
$config = @{
    provider = "bailian"
    baseUrl = "https://dashscope.aliyuncs.com/api/v1"
    model = "qwen-plus"
    apiKey = "sk-8a66058f2bb84ab69047bf3037f0e5ac"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:9090/api/config/llm" -Method Post -Body $config -ContentType "application/json"

# 开始对话
$chat = @{
    message = "列出 default 命名空间的所有 Pods"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:9090/api/chat" -Method Post -Body $chat -ContentType "application/json"
```

## 获取百炼平台 API Key

1. 访问 [阿里云百炼平台](https://modelstudio.aliyun.com/)
2. 登录/注册账号
3. 开通服务（免费）
4. 在控制台找到"API Key"管理
5. 创建并复制 API Key

## 故障排查

### 前端无法访问

- 确保已运行 `.\scripts\build-frontend.ps1` 构建前端
- 检查 `static` 目录是否存在

### LLM 连接失败

- 检查 API Key 是否正确
- 确认网络可以访问 `dashscope.aliyuncs.com`
- 查看后端日志获取详细错误信息

### K8s 连接失败

- 检查 `.env` 文件中的配置
- 确认 Token 未过期
- 验证网络连接

