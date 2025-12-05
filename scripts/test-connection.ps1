# 测试 Kubernetes API 连接

Write-Host "=== 测试 Kubernetes API 连接 ===" -ForegroundColor Green

# 读取配置
$envFile = ".env"
if (-not (Test-Path $envFile)) {
    Write-Host "错误: 找不到 .env 文件" -ForegroundColor Red
    exit 1
}

$content = Get-Content $envFile
$apiServer = ($content | Select-String "K8S_API_SERVER").ToString().Split("=")[1].Trim()
$token = ($content | Select-String "K8S_API_TOKEN").ToString().Split("=")[1].Trim()
$insecure = ($content | Select-String "K8S_API_INSECURE").ToString().Split("=")[1].Trim()

Write-Host "`n配置信息:" -ForegroundColor Yellow
Write-Host "  API Server: $apiServer" -ForegroundColor Cyan
Write-Host "  Token 长度: $($token.Length)" -ForegroundColor Cyan
Write-Host "  Insecure: $insecure" -ForegroundColor Cyan

# 测试连接（使用 curl）
Write-Host "`n测试 API 连接..." -ForegroundColor Yellow

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# 如果使用 HTTPS 且需要跳过证书验证
if ($apiServer -like "https://*" -and $insecure -eq "true") {
    # 忽略 SSL 证书错误
    [System.Net.ServicePointManager]::ServerCertificateValidationCallback = {$true}
}

try {
    # 测试获取 API 版本
    $url = "$apiServer/version"
    Write-Host "  请求: $url" -ForegroundColor Gray
    
    $response = Invoke-RestMethod -Uri $url -Method Get -Headers $headers -ErrorAction Stop
    
    Write-Host "✓ 连接成功!" -ForegroundColor Green
    Write-Host "  Kubernetes 版本: $($response.gitVersion)" -ForegroundColor Cyan
    Write-Host "  平台: $($response.platform)" -ForegroundColor Cyan
    
    # 测试获取 Pods
    Write-Host "`n测试获取 Pods..." -ForegroundColor Yellow
    $podsUrl = "$apiServer/api/v1/namespaces/default/pods"
    Write-Host "  请求: $podsUrl" -ForegroundColor Gray
    
    $podsResponse = Invoke-RestMethod -Uri $podsUrl -Method Get -Headers $headers -ErrorAction Stop
    
    Write-Host "✓ 获取 Pods 成功!" -ForegroundColor Green
    Write-Host "  Pod 数量: $($podsResponse.items.Count)" -ForegroundColor Cyan
    
    if ($podsResponse.items.Count -gt 0) {
        Write-Host "`n前 5 个 Pods:" -ForegroundColor Yellow
        $podsResponse.items | Select-Object -First 5 | ForEach-Object {
            Write-Host "  - $($_.metadata.name) ($($_.status.phase))" -ForegroundColor White
        }
    }
    
} catch {
    Write-Host "✗ 连接失败!" -ForegroundColor Red
    Write-Host "  错误: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.ErrorDetails.Message) {
        Write-Host "  详细信息: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
    
    Write-Host "`n可能的原因:" -ForegroundColor Yellow
    Write-Host "  1. Token 已过期或无效" -ForegroundColor White
    Write-Host "  2. 网络连接问题" -ForegroundColor White
    Write-Host "  3. API 服务器配置问题" -ForegroundColor White
    Write-Host "  4. 防火墙阻止连接" -ForegroundColor White
    
    exit 1
}

Write-Host "`n=== 测试完成 ===" -ForegroundColor Green

