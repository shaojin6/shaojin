# 获取 k8s-mcp-server-sa ServiceAccount 的 Token
# 在 dify 命名空间下

Write-Host "获取 k8s-mcp-server-sa ServiceAccount Token..." -ForegroundColor Green
Write-Host "命名空间: dify" -ForegroundColor Yellow
Write-Host ""

# 检查 kubectl 是否可用
if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
    Write-Host "错误: 未找到 kubectl 命令" -ForegroundColor Red
    Write-Host "请确保已安装 kubectl 并配置好 kubeconfig" -ForegroundColor Red
    exit 1
}

# 获取 ServiceAccount 的 Secret 名称
$secretName = kubectl get sa k8s-mcp-server-sa -n dify -o jsonpath='{.secrets[0].name}' 2>$null

if (-not $secretName) {
    Write-Host "错误: 未找到 k8s-mcp-server-sa ServiceAccount 的 Secret" -ForegroundColor Red
    Write-Host "请确保 ServiceAccount 已创建并且有关联的 Secret" -ForegroundColor Red
    exit 1
}

Write-Host "找到 Secret: $secretName" -ForegroundColor Green
Write-Host ""

# 获取 Token
$token = kubectl get secret $secretName -n dify -o jsonpath='{.data.token}' | ForEach-Object { [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($_)) }

if (-not $token) {
    Write-Host "错误: 无法从 Secret 中提取 Token" -ForegroundColor Red
    exit 1
}

Write-Host "Token 获取成功!" -ForegroundColor Green
Write-Host ""
Write-Host "请将以下内容添加到 .env 文件中:" -ForegroundColor Yellow
Write-Host "----------------------------------------" -ForegroundColor Gray
Write-Host "K8S_API_SERVER=https://11.0.1.110:6443" -ForegroundColor Cyan
Write-Host "K8S_API_TOKEN=$token" -ForegroundColor Cyan
Write-Host "K8S_API_INSECURE=true" -ForegroundColor Cyan
Write-Host "----------------------------------------" -ForegroundColor Gray
Write-Host ""
Write-Host "注意: Kubernetes API 使用 HTTPS（端口 6443）" -ForegroundColor Yellow
Write-Host "如果配置为 HTTP，代码会自动转换为 HTTPS" -ForegroundColor Gray
Write-Host ""
Write-Host "或者直接复制以下命令设置环境变量:" -ForegroundColor Yellow
Write-Host "`$env:K8S_API_SERVER='https://11.0.1.110:6443'" -ForegroundColor Cyan
Write-Host "`$env:K8S_API_TOKEN='$token'" -ForegroundColor Cyan
Write-Host "`$env:K8S_API_INSECURE='true'" -ForegroundColor Cyan
Write-Host ""

