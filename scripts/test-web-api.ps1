# 测试 Web API 的 PowerShell 脚本

Write-Host "=== Kubernetes MCP Web API 测试 ===" -ForegroundColor Green

$baseUrl = "http://localhost:8080"

# 1. 健康检查
Write-Host "`n1. 测试健康检查..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/healthz" -Method Get
    Write-Host "✓ 健康检查通过: $($response.status)" -ForegroundColor Green
} catch {
    Write-Host "✗ 健康检查失败: $_" -ForegroundColor Red
    exit 1
}

# 2. 测试 K8s 连接
Write-Host "`n2. 测试 Kubernetes 连接..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/test-k8s" -Method Post
    if ($response.status -eq "ok") {
        Write-Host "✓ Kubernetes 连接成功: $($response.message)" -ForegroundColor Green
    } else {
        Write-Host "✗ Kubernetes 连接失败: $($response.message)" -ForegroundColor Red
    }
} catch {
    Write-Host "✗ 测试失败: $_" -ForegroundColor Red
}

# 3. 列出工具
Write-Host "`n3. 列出可用工具..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/tools" -Method Get
    Write-Host "✓ 找到 $($response.tools.Count) 个工具:" -ForegroundColor Green
    foreach ($tool in $response.tools) {
        Write-Host "  - $($tool.name): $($tool.description)" -ForegroundColor Cyan
    }
} catch {
    Write-Host "✗ 获取工具列表失败: $_" -ForegroundColor Red
}

# 4. 调用工具 - 列出 Pods
Write-Host "`n4. 测试调用工具 (list_pods)..." -ForegroundColor Yellow
try {
    $body = @{
        name = "list_pods"
        arguments = @{
            namespace = "default"
        }
    } | ConvertTo-Json

    $response = Invoke-RestMethod -Uri "$baseUrl/api/tools/call" -Method Post -Body $body -ContentType "application/json"
    Write-Host "✓ 工具调用成功" -ForegroundColor Green
    Write-Host "结果:" -ForegroundColor Cyan
    $response.content | ForEach-Object {
        Write-Host $_.text -ForegroundColor White
    }
} catch {
    Write-Host "✗ 工具调用失败: $_" -ForegroundColor Red
}

# 5. 获取状态
Write-Host "`n5. 获取系统状态..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/status" -Method Get
    Write-Host "✓ 系统状态:" -ForegroundColor Green
    $response | ConvertTo-Json -Depth 3 | Write-Host
} catch {
    Write-Host "✗ 获取状态失败: $_" -ForegroundColor Red
}

Write-Host "`n=== 测试完成 ===" -ForegroundColor Green

