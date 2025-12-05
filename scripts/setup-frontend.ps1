# 初始化前端项目

Write-Host "=== 初始化前端项目 ===" -ForegroundColor Green

if (-not (Test-Path "web-ui")) {
    Write-Host "错误: web-ui 目录不存在" -ForegroundColor Red
    exit 1
}

Set-Location web-ui

# 检查 Node.js
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    Write-Host "错误: 未找到 Node.js" -ForegroundColor Red
    Write-Host "请先安装 Node.js: https://nodejs.org/" -ForegroundColor Yellow
    Set-Location ..
    exit 1
}

Write-Host "Node.js 版本: $(node -v)" -ForegroundColor Cyan
Write-Host "npm 版本: $(npm -v)" -ForegroundColor Cyan
Write-Host ""

Write-Host "安装依赖..." -ForegroundColor Yellow
npm install

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ 依赖安装成功!" -ForegroundColor Green
    Write-Host ""
    Write-Host "启动开发服务器:" -ForegroundColor Yellow
    Write-Host "  cd web-ui" -ForegroundColor Cyan
    Write-Host "  npm run dev" -ForegroundColor Cyan
} else {
    Write-Host "✗ 依赖安装失败" -ForegroundColor Red
    Set-Location ..
    exit 1
}

Set-Location ..

