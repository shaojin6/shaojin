# 修复提交 39f7aa7 的乱码问题
# 使用方法: .\scripts\fix-commit-39f7aa7.ps1

Write-Host "=== 修复提交 39f7aa7 乱码 ===" -ForegroundColor Cyan

# 设置 UTF-8 编码
$env:LANG = "zh_CN.UTF-8"
$env:LC_ALL = "zh_CN.UTF-8"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

Write-Host "`n要修复的提交:" -ForegroundColor Yellow
Write-Host "  - 39f7aa7: feat: 集成 Worker 模块并优化工具列表超时配置" -ForegroundColor White
Write-Host "  - abfbb68: feat: 优化刷新按钮功能区分" -ForegroundColor White

Write-Host "`n步骤 1: 开始交互式 rebase" -ForegroundColor Cyan
Write-Host "  运行: git rebase -i 5e8d437" -ForegroundColor White
Write-Host "`n步骤 2: 在编辑器中修改" -ForegroundColor Cyan
Write-Host "  找到以下提交，将 'pick' 改为 'reword':" -ForegroundColor White
Write-Host "    pick 39f7aa7 feat: 集成 Worker 模块..." -ForegroundColor Gray
Write-Host "    pick abfbb68 feat: 优化刷新按钮..." -ForegroundColor Gray
Write-Host "  改为:" -ForegroundColor White
Write-Host "    reword 39f7aa7 feat: 集成 Worker 模块..." -ForegroundColor Green
Write-Host "    reword abfbb68 feat: 优化刷新按钮..." -ForegroundColor Green
Write-Host "`n步骤 3: 修改提交信息" -ForegroundColor Cyan
Write-Host "  Git 会逐个打开编辑器，修改为:" -ForegroundColor White
Write-Host "`n  提交 39f7aa7:" -ForegroundColor Yellow
Write-Host "    feat: integrate Worker module and optimize tool list timeout config" -ForegroundColor Gray
Write-Host "`n    - Integrate Worker module to router.go, implement async task management" -ForegroundColor Gray
Write-Host "    - Replace 3 go func() calls with worker.EnqueueTask()" -ForegroundColor Gray
Write-Host "    - Add Worker initialization logic (support env var config)" -ForegroundColor Gray
Write-Host "    - Add /api/worker/stats monitoring endpoint" -ForegroundColor Gray
Write-Host "    - Implement graceful shutdown, auto stop Worker when service stops" -ForegroundColor Gray
Write-Host "`n  提交 abfbb68:" -ForegroundColor Yellow
Write-Host "    feat: optimize refresh button functionality distinction" -ForegroundColor Gray
Write-Host "`n步骤 4: 强制推送" -ForegroundColor Cyan
Write-Host "  运行: git push --force-with-lease origin master" -ForegroundColor White

$confirm = Read-Host "`n是否现在开始修复? (yes/no)"
if ($confirm -eq "yes") {
    Write-Host "`n开始交互式 rebase..." -ForegroundColor Cyan
    git rebase -i 5e8d437
} else {
    Write-Host "`n已取消，请手动执行上述步骤" -ForegroundColor Yellow
}

