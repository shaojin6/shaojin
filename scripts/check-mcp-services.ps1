# 检查 MCP 服务配置脚本
# 用于验证数据库中的 MCP 服务配置

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   检查 MCP 服务配置" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查 API 返回的 MCP 服务列表
try {
    $response = Invoke-RestMethod -Uri "http://localhost:9090/api/config/remote-mcp" -Method GET
    Write-Host "从 API 获取的 MCP 服务列表:" -ForegroundColor Green
    Write-Host ""
    
    if ($response -is [Array]) {
        Write-Host "共找到 $($response.Count) 个服务:" -ForegroundColor Yellow
        foreach ($service in $response) {
            Write-Host "  - 名称: $($service.name)" -ForegroundColor White
            Write-Host "    标识符: $($service.serverId)" -ForegroundColor Gray
            Write-Host "    地址: $($service.baseUrl)" -ForegroundColor Gray
            Write-Host "    启用: $($service.enabled)" -ForegroundColor Gray
            Write-Host ""
        }
    } else {
        Write-Host "响应格式异常: $($response | ConvertTo-Json -Depth 3)" -ForegroundColor Red
    }
} catch {
    Write-Host "无法连接到服务: $_" -ForegroundColor Red
}

Write-Host ""
Write-Host "提示: 如果 kubernetes-mcp-server 不在列表中，可能需要重新添加" -ForegroundColor Yellow

