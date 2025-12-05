# 构建前端项目

Write-Host "=== 构建前端项目 ===" -ForegroundColor Green

if (-not (Test-Path "web-ui")) {
    Write-Host "错误: web-ui 目录不存在" -ForegroundColor Red
    exit 1
}

Set-Location web-ui

# 检查 node_modules
if (-not (Test-Path "node_modules")) {
    Write-Host "安装依赖..." -ForegroundColor Yellow
    npm install
    if ($LASTEXITCODE -ne 0) {
        Write-Host "依赖安装失败" -ForegroundColor Red
        Set-Location ..
        exit 1
    }
}

Write-Host "构建前端..." -ForegroundColor Yellow
npm run build

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ 前端构建成功!" -ForegroundColor Green
    Write-Host "构建产物已输出到 static 目录" -ForegroundColor Cyan
} else {
    Write-Host "✗ 构建失败" -ForegroundColor Red
    Set-Location ..
    exit 1
}

Set-Location ..

