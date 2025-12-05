# LLM 配置指南

## 支持的 LLM 提供商

本项目支持以下 LLM 提供商：

1. **通义千问（百炼平台/Model Studio）** - 阿里云（推荐）
2. **通义千问（DashScope）** - 阿里云（旧版，已升级到百炼）
3. **OpenAI** - OpenAI API
4. **Ollama** - 本地部署的模型

## 通义千问（百炼平台）配置

> **重要提示**：DashScope 已升级为百炼平台（Model Studio），请使用百炼平台获取 API Key。

### 1. 获取 API Key

1. 访问 [阿里云百炼平台控制台](https://modelstudio.aliyun.com/)
2. 开通百炼平台服务（免费开通）
3. 在控制台中找到 "API Key" 管理页面
4. 创建并复制 API Key

**注意**：
- 百炼平台使用与 DashScope 相同的 API 端点
- 新用户通常有免费额度
- API Key 格式与 DashScope 相同

### 2. 配置参数

通过 API 配置：

```bash
POST http://localhost:9090/api/config/llm
Content-Type: application/json

{
  "provider": "bailian",
  "baseUrl": "https://dashscope.aliyuncs.com/api/v1",
  "model": "qwen3-max",
  "apiKey": "your-api-key-here"
}
```

**参数说明：**
- `provider`: 支持以下值（都使用相同的 API）：
  - `"bailian"` - 百炼平台（推荐，新用户使用）
  - `"sfm"` - 百炼平台别名
  - `"modelstudio"` - Model Studio 别名
  - `"dashscope"` - DashScope（旧版，已升级）
  - `"qwen"` - 通义千问别名
  - `"tongyi"` - 通义千问别名
- `baseUrl`: API 基础地址（可选，默认：`https://dashscope.aliyuncs.com/api/v1`）
  - 百炼平台和 DashScope 使用相同的 API 端点
- `model`: 模型名称，可选值：
  - `qwen3-max` - 通义千问 3 Max（最新最强，推荐）
    - 支持 8k tokens 上下文（API 限定输入为 6k tokens）
    - 最新版本，性能最强
    - 适合复杂推理和高质量回答
  - `qwen-plus` - 通义千问 Plus（性价比高）
    - 平衡性能和成本
    - 适合大多数场景
  - `qwen-turbo` - 通义千问 Turbo（更快，适合简单任务）
    - 响应速度快
    - 适合简单查询
  - `qwen-max` - 通义千问 Max（旧版最强性能）
    - 旧版本最强模型
    - 建议升级到 qwen3-max
  - `qwen-max-longcontext` - 支持长上下文
    - 支持更长的上下文
- `apiKey`: 你的百炼平台 API Key（从百炼控制台获取）

### 3. 测试连接

```bash
POST http://localhost:9090/api/test-llm
```

## OpenAI 配置

```json
{
  "provider": "openai",
  "baseUrl": "https://api.openai.com/v1",
  "model": "gpt-3.5-turbo",
  "apiKey": "your-openai-api-key"
}
```

## Ollama 配置（本地模型）

```json
{
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "model": "qwen2:7b",
  "apiKey": ""
}
```

## 使用对话接口

配置完成后，可以使用对话接口：

```bash
POST http://localhost:9090/api/chat
Content-Type: application/json

{
  "sessionId": "session_123",
  "message": "列出 default 命名空间的所有 Pods"
}
```

系统会自动：
1. 理解你的问题
2. 调用相应的 Kubernetes 工具
3. 基于结果生成自然语言回答

## 示例

### 示例 1: 查询 Pods

**请求：**
```json
{
  "message": "default 命名空间有多少个 Pod？"
}
```

**响应：**
```json
{
  "sessionId": "session_123",
  "reply": "default 命名空间共有 3 个 Pods：nginx-deployment-xxx, redis-xxx, app-xxx",
  "steps": [
    {
      "type": "llm",
      "text": "需要查询 default 命名空间的 Pods"
    },
    {
      "type": "tool",
      "tool": "list_pods",
      "arguments": {"namespace": "default"},
      "result": {...}
    }
  ]
}
```

### 示例 2: 查看 Deployment 状态

**请求：**
```json
{
  "message": "nginx deployment 的状态如何？"
}
```

## 注意事项

1. **API Key 安全**：请妥善保管 API Key，不要提交到版本控制系统
2. **费用**：使用公网 LLM 可能产生费用，请查看各提供商的计费说明
3. **网络**：确保服务器可以访问 LLM API 地址
4. **超时**：LLM 调用默认超时时间为 60 秒

## 故障排查

### 连接失败

1. 检查 API Key 是否正确
2. 检查网络连接
3. 检查 API 地址是否正确

### 模型不存在

1. 确认模型名称正确
2. 检查你的账户是否有权限使用该模型

### 响应超时

1. 增加超时时间（需要修改代码）
2. 使用更快的模型（如 qwen-turbo）

