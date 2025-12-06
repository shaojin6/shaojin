# 启动 Ansible MCP Server 脚本
# 快速启动 ansible-mcp-server 服务

param(
    [int]$Port = 8080,
    [string]$Host = "0.0.0.0"
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  启动 Ansible MCP Server" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查目录
if (-not (Test-Path "ansible-mcp-main")) {
    Write-Host "错误: 未找到 ansible-mcp-main 目录" -ForegroundColor Red
    exit 1
}

# 检查 Python
$pythonVersion = python --version 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "错误: 未找到 Python" -ForegroundColor Red
    exit 1
}

# 检查依赖
Set-Location ansible-mcp-main
if (-not (Test-Path "requirements.txt")) {
    Write-Host "错误: 未找到 requirements.txt" -ForegroundColor Red
    exit 1
}

# 检查端口
$portInUse = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
if ($portInUse) {
    Write-Host "警告: 端口 $Port 已被占用" -ForegroundColor Yellow
    Write-Host "正在尝试停止占用该端口的进程..." -ForegroundColor Yellow
    $process = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique
    if ($process) {
        Stop-Process -Id $process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
    }
}

Write-Host "启动参数:" -ForegroundColor Yellow
Write-Host "  主机: $Host" -ForegroundColor White
Write-Host "  端口: $Port" -ForegroundColor White
Write-Host "  工作目录: $(Get-Location)" -ForegroundColor White
Write-Host ""

Write-Host "正在启动服务..." -ForegroundColor Yellow
Write-Host "访问 http://localhost:$Port/docs 查看 API 文档" -ForegroundColor Cyan
Write-Host "按 Ctrl+C 停止服务" -ForegroundColor Yellow
Write-Host ""

# 启动服务
uvicorn main:app --host $Host --port $Port

Set-Location ..

