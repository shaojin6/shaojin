# ServiceAccount 配置说明

## 当前使用的 ServiceAccount

项目使用 `dify` 命名空间下的 `k8s-mcp-server-sa` ServiceAccount，该账户具有管理员权限（cluster-admin）。

## 获取 Token

### 方法 1: 使用 PowerShell 脚本（推荐，Windows）

```powershell
.\scripts\get-token.ps1
```

脚本会自动获取 Token 并显示如何配置。

### 方法 2: 使用 Bash 脚本（Linux/Mac）

```bash
chmod +x scripts/get-token.sh
./scripts/get-token.sh
```

### 方法 3: 手动获取

#### Windows PowerShell

```powershell
# 获取 Secret 名称
$secretName = kubectl get sa k8s-mcp-server-sa -n dify -o jsonpath='{.secrets[0].name}'

# 获取 Token
$token = kubectl get secret $secretName -n dify -o jsonpath='{.data.token}' | ForEach-Object { [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($_)) }

# 显示 Token
Write-Host $token
```

#### Linux/Mac Bash

```bash
# 获取 Secret 名称
SECRET_NAME=$(kubectl get sa k8s-mcp-server-sa -n dify -o jsonpath='{.secrets[0].name}')

# 获取 Token
TOKEN=$(kubectl get secret $SECRET_NAME -n dify -o jsonpath='{.data.token}' | base64 -d)

# 显示 Token
echo $TOKEN
```

## 配置 Token

获取 Token 后，将其添加到 `.env` 文件中：

```env
K8S_API_SERVER=https://11.0.1.110:6443
K8S_API_TOKEN=<your-token-here>
K8S_API_INSECURE=true
```

**注意**: 
- Kubernetes API 服务器通常使用 HTTPS（端口 6443）
- 如果配置为 HTTP，代码会自动转换为 HTTPS
- 如果 API 服务器使用自签名证书，需要设置 `K8S_API_INSECURE=true`

## 验证配置

启动服务后，访问 `http://localhost:9090/api/test-k8s` 测试连接。

**注意**: 默认服务端口是 9090（可通过 `MCP_WEB_ADDR` 环境变量修改）

如果返回成功，说明 Token 配置正确。

## 权限说明

`k8s-mcp-server-sa` ServiceAccount 具有 `cluster-admin` 权限，可以：

- 查看所有命名空间的资源
- 执行所有操作（get, list, create, update, delete, patch）
- 访问所有 API 组和资源

## 安全提示

⚠️ **重要**: 
- Token 具有管理员权限，请妥善保管
- 不要将 `.env` 文件提交到版本控制系统
- 定期轮换 Token（如果可能）

## 故障排查

### Token 过期

如果遇到认证错误，可能是 Token 过期。重新获取 Token 并更新 `.env` 文件。

### 权限不足

如果遇到权限错误，检查 ServiceAccount 是否具有正确的权限：

```bash
kubectl get clusterrolebinding -o wide | grep k8s-mcp-server-sa
kubectl get rolebinding -n dify -o wide | grep k8s-mcp-server-sa
```

### 连接问题

如果无法连接到 API 服务器，检查：

1. API 服务器地址是否正确（应使用 HTTPS）
2. 网络连接是否正常
3. 防火墙规则是否允许连接
4. 如果使用自签名证书，确保设置了 `K8S_API_INSECURE=true`

### HTTP vs HTTPS

- Kubernetes API 服务器默认使用 HTTPS（端口 6443）
- 如果 `.env` 文件中配置为 `http://`，代码会自动转换为 `https://`
- 建议直接配置为 `https://` 以避免警告信息

