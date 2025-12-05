# 配置文件说明

## 概述

项目支持使用 `.env` 文件来存储配置信息，避免每次启动时都需要设置环境变量。

## 创建配置文件

### 方法 1: 使用脚本（推荐）

```powershell
.\scripts\create-config.bat
```

然后编辑生成的 `.env` 文件，填入你的配置信息。

### 方法 2: 手动创建

1. 复制示例配置文件：
   ```powershell
   copy config.env.example .env
   ```

2. 编辑 `.env` 文件，填入你的配置：
   ```env
   K8S_API_SERVER=http://11.0.1.110:6443
   K8S_API_TOKEN=your-token-here
   K8S_API_INSECURE=true
   MCP_WEB_ADDR=:8080
   ```

## 配置项说明

### Kubernetes API 配置

- `K8S_API_SERVER`: Kubernetes API 服务器地址（必需）
- `K8S_API_TOKEN`: 认证 Token（推荐使用）
- `K8S_API_USERNAME`: 用户名（可选，与 Token 二选一）
- `K8S_API_PASSWORD`: 密码（可选，与 Token 二选一）
- `K8S_API_INSECURE`: 是否跳过 TLS 验证（true/false，仅用于开发环境）
- `K8S_API_CA_FILE`: CA 证书文件路径（可选）
- `K8S_API_CA_DATA`: CA 证书内容 Base64 编码（可选）

### Web 服务配置

- `MCP_WEB_ADDR`: Web 服务监听地址（默认 :8080）
- `GIN_MODE`: Gin 框架模式（debug/release/test，默认 release）

## 配置文件优先级

1. **环境变量**（最高优先级）
2. **`.env` 文件**（当前目录）
3. **项目根目录的 `.env`**
4. **用户主目录的 `.k8s-mcp.env`**

如果环境变量已设置，则不会从 `.env` 文件读取该变量。

## 使用方式

启动服务时，程序会自动加载 `.env` 文件：

```powershell
.\scripts\start-now.bat
```

或者直接运行：

```powershell
go run .\cmd\web\main.go
```

## 安全提示

⚠️ **重要**: `.env` 文件包含敏感信息（如 Token），已被添加到 `.gitignore` 中，不会被提交到版本控制系统。

请妥善保管你的 `.env` 文件，不要分享给他人。

