# Ansible MCP Server 快速配置脚本
# 用于在 Windows 环境下快速启动和配置 ansible-mcp-server

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Ansible MCP Server 配置向导" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查 Python 环境
Write-Host "[1/5] 检查 Python 环境..." -ForegroundColor Yellow
$pythonVersion = python --version 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "错误: 未找到 Python，请先安装 Python 3.11+" -ForegroundColor Red
    exit 1
}
Write-Host "  Python 版本: $pythonVersion" -ForegroundColor Green

# 检查 ansible-mcp-main 目录
Write-Host "[2/5] 检查 ansible-mcp-main 目录..." -ForegroundColor Yellow
if (-not (Test-Path "ansible-mcp-main")) {
    Write-Host "错误: 未找到 ansible-mcp-main 目录" -ForegroundColor Red
    exit 1
}
Write-Host "  ansible-mcp-main 目录存在" -ForegroundColor Green

# 检查依赖
Write-Host "[3/5] 检查 Python 依赖..." -ForegroundColor Yellow
Set-Location ansible-mcp-main
if (-not (Test-Path "requirements.txt")) {
    Write-Host "错误: 未找到 requirements.txt" -ForegroundColor Red
    exit 1
}

# 询问是否安装依赖
$installDeps = Read-Host "是否安装 Python 依赖? (y/n)"
if ($installDeps -eq "y" -or $installDeps -eq "Y") {
    Write-Host "  正在安装依赖..." -ForegroundColor Yellow
    pip install -r requirements.txt
    if ($LASTEXITCODE -ne 0) {
        Write-Host "错误: 依赖安装失败" -ForegroundColor Red
        exit 1
    }
    Write-Host "  依赖安装完成" -ForegroundColor Green
}

# 检查 Ansible
Write-Host "[4/5] 检查 Ansible..." -ForegroundColor Yellow
$ansibleVersion = ansible --version 2>&1 | Select-Object -First 1
if ($LASTEXITCODE -ne 0) {
    Write-Host "警告: 未找到 Ansible，请确保已安装 ansible-core" -ForegroundColor Yellow
} else {
    Write-Host "  Ansible: $ansibleVersion" -ForegroundColor Green
}

# 配置端口
Write-Host "[5/5] 配置服务端口..." -ForegroundColor Yellow
$defaultPort = 8080
$port = Read-Host "请输入服务端口 (默认: $defaultPort)"
if ([string]::IsNullOrWhiteSpace($port)) {
    $port = $defaultPort
}

# 检查端口是否被占用
$portInUse = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
if ($portInUse) {
    Write-Host "警告: 端口 $port 已被占用" -ForegroundColor Yellow
    $continue = Read-Host "是否继续? (y/n)"
    if ($continue -ne "y" -and $continue -ne "Y") {
        exit 0
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  配置完成！" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "启动方式:" -ForegroundColor Yellow
Write-Host "  1. 本地运行:" -ForegroundColor White
Write-Host "     cd ansible-mcp-main" -ForegroundColor Gray
Write-Host "     uvicorn main:app --host 0.0.0.0 --port $port" -ForegroundColor Gray
Write-Host ""
Write-Host "  2. 后台运行 (PowerShell):" -ForegroundColor White
Write-Host "     Start-Process -FilePath 'uvicorn' -ArgumentList 'main:app','--host','0.0.0.0','--port','$port' -WorkingDirectory 'ansible-mcp-main' -WindowStyle Hidden" -ForegroundColor Gray
Write-Host ""
Write-Host "Web UI 配置:" -ForegroundColor Yellow
Write-Host "  服务名称: ansible-mcp-server" -ForegroundColor White
Write-Host "  服务器标识符: ansible-mcp-server" -ForegroundColor White
Write-Host "  类型: HTTP" -ForegroundColor White
Write-Host "  访问地址: http://localhost:$port/sse" -ForegroundColor White
Write-Host "  超时时间: 30" -ForegroundColor White
Write-Host "  SSE 读取超时: 300" -ForegroundColor White
Write-Host ""
Write-Host "测试连接:" -ForegroundColor Yellow
Write-Host "  curl http://localhost:$port/docs" -ForegroundColor Gray
Write-Host "  curl http://localhost:$port/sse" -ForegroundColor Gray
Write-Host ""

# 询问是否立即启动
$startNow = Read-Host "是否立即启动服务? (y/n)"
if ($startNow -eq "y" -or $startNow -eq "Y") {
    Write-Host "正在启动服务..." -ForegroundColor Yellow
    Set-Location ..
    Start-Process -FilePath "uvicorn" -ArgumentList "main:app","--host","0.0.0.0","--port","$port" -WorkingDirectory "ansible-mcp-main" -WindowStyle Normal
    Write-Host "服务已启动，请查看新窗口" -ForegroundColor Green
    Write-Host "访问 http://localhost:$port/docs 查看 API 文档" -ForegroundColor Cyan
}

Set-Location ..

