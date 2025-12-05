# 收集 MCP 服务信息的脚本

Write-Host "=== MCP 服务信息收集 ===" -ForegroundColor Green
Write-Host ""

# 1. 部署信息
Write-Host "1. 部署信息" -ForegroundColor Yellow
$namespace = Read-Host "命名空间 (例如: dify)"
$serviceName = Read-Host "Service 名称 (例如: k8s-mcp-service)"
$port = Read-Host "端口号 (例如: 8080)"

# 2. 访问方式
Write-Host "`n2. 访问方式" -ForegroundColor Yellow
Write-Host "请选择访问方式:"
Write-Host "1) ClusterIP (集群内)"
Write-Host "2) NodePort"
Write-Host "3) LoadBalancer"
Write-Host "4) Ingress"
$accessType = Read-Host "选择 (1-4)"

# 3. 协议信息
Write-Host "`n3. 通信协议" -ForegroundColor Yellow
Write-Host "请选择通信协议:"
Write-Host "1) HTTP/REST API"
Write-Host "2) WebSocket"
Write-Host "3) stdio"
Write-Host "4) 其他"
$protocol = Read-Host "选择 (1-4)"

# 4. API 端点
$baseUrl = ""
if ($protocol -eq "1" -or $protocol -eq "2") {
    Write-Host "`n4. API 端点" -ForegroundColor Yellow
    if ($accessType -eq "1") {
        $baseUrl = "http://${serviceName}.${namespace}.svc.cluster.local:${port}"
        Write-Host "建议的集群内地址: $baseUrl" -ForegroundColor Cyan
    } else {
        $baseUrl = Read-Host "请输入完整的访问地址 (例如: http://your-domain.com)"
    }
}

# 5. 认证方式
Write-Host "`n5. 认证方式" -ForegroundColor Yellow
Write-Host "请选择认证方式:"
Write-Host "1) 无认证"
Write-Host "2) Bearer Token"
Write-Host "3) API Key"
Write-Host "4) 其他"
$authType = Read-Host "选择 (1-4)"

$authInfo = @{}
if ($authType -eq "2") {
    $token = Read-Host "请输入 Token"
    $authInfo = @{ type = "bearer_token"; token = $token }
} elseif ($authType -eq "3") {
    $apiKey = Read-Host "请输入 API Key"
    $authInfo = @{ type = "api_key"; api_key = $apiKey }
}

# 6. 工具信息
Write-Host "`n6. 工具信息" -ForegroundColor Yellow
$toolsEndpoint = Read-Host "工具列表 API 端点 (如果支持自动发现，留空则跳过)"
if ([string]::IsNullOrWhiteSpace($toolsEndpoint)) {
    $toolsEndpoint = "/api/tools"  # 默认端点
}

# 生成配置
$config = @{
    deployment = @{
        namespace = $namespace
        service_name = $serviceName
        port = $port
        access_type = @("ClusterIP", "NodePort", "LoadBalancer", "Ingress")[[int]$accessType - 1]
    }
    protocol = @("http", "websocket", "stdio", "other")[[int]$protocol - 1]
    base_url = $baseUrl
    auth = $authInfo
    tools_endpoint = $toolsEndpoint
}

# 保存配置
$configJson = $config | ConvertTo-Json -Depth 10
$outputFile = "mcp-service-config.json"
$configJson | Out-File -FilePath $outputFile -Encoding UTF8

Write-Host "`n=== 配置已保存 ===" -ForegroundColor Green
Write-Host "配置文件: $outputFile" -ForegroundColor Cyan
Write-Host ""
Write-Host "配置内容:" -ForegroundColor Yellow
Write-Host $configJson -ForegroundColor White

Write-Host "`n下一步:" -ForegroundColor Yellow
Write-Host "1. 检查配置文件是否正确" -ForegroundColor White
Write-Host "2. 将配置文件发送给开发人员" -ForegroundColor White
Write-Host "3. 或直接使用此配置进行集成开发" -ForegroundColor White

