# 配置通义千问 LLM 的 PowerShell 脚本（支持百炼平台）

Write-Host "=== 配置通义千问 LLM（百炼平台）===" -ForegroundColor Green
Write-Host ""
Write-Host "注意: DashScope 已升级为百炼平台（Model Studio）" -ForegroundColor Yellow
Write-Host "请从百炼平台获取 API Key: https://modelstudio.aliyun.com/" -ForegroundColor Cyan
Write-Host ""

# 提示用户输入信息
$apiKey = Read-Host "请输入你的百炼平台 API Key"
if ([string]::IsNullOrWhiteSpace($apiKey)) {
    Write-Host "错误: API Key 不能为空" -ForegroundColor Red
    exit 1
}

$model = Read-Host "请输入模型名称 (默认: qwen3-max) [qwen3-max/qwen-plus/qwen-turbo/qwen-max]"
if ([string]::IsNullOrWhiteSpace($model)) {
    $model = "qwen3-max"
}

$baseUrl = Read-Host "请输入 API 地址 (默认: https://dashscope.aliyuncs.com/api/v1)"
if ([string]::IsNullOrWhiteSpace($baseUrl)) {
    $baseUrl = "https://dashscope.aliyuncs.com/api/v1"
}

# 构建配置 JSON（使用百炼平台）
$config = @{
    provider = "bailian"
    baseUrl = $baseUrl
    model = $model
    apiKey = $apiKey
} | ConvertTo-Json

Write-Host ""
Write-Host "配置信息:" -ForegroundColor Yellow
Write-Host $config -ForegroundColor Cyan
Write-Host ""

# 发送配置请求
$baseUrl = "http://localhost:9090"
Write-Host "正在配置 LLM..." -ForegroundColor Yellow

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/config/llm" -Method Post -Body $config -ContentType "application/json"
    Write-Host "✓ LLM 配置成功!" -ForegroundColor Green
    Write-Host ""
    
    # 测试连接
    Write-Host "正在测试 LLM 连接..." -ForegroundColor Yellow
    $testResponse = Invoke-RestMethod -Uri "$baseUrl/api/test-llm" -Method Post
    if ($testResponse.status -eq "ok") {
        Write-Host "✓ LLM 连接测试成功!" -ForegroundColor Green
        Write-Host "消息: $($testResponse.message)" -ForegroundColor Cyan
    } else {
        Write-Host "✗ LLM 连接测试失败" -ForegroundColor Red
        Write-Host "消息: $($testResponse.message)" -ForegroundColor Red
    }
} catch {
    Write-Host "✗ 配置失败: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "=== 配置完成 ===" -ForegroundColor Green
Write-Host ""
Write-Host "现在可以使用对话接口了:" -ForegroundColor Yellow
Write-Host "POST $baseUrl/api/chat" -ForegroundColor Cyan
Write-Host 'Body: {"message": "列出 default 命名空间的所有 Pods"}' -ForegroundColor Cyan

