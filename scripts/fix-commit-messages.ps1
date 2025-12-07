# 修复 Git 提交历史中的乱码问题
# 使用方法: .\scripts\fix-commit-messages.ps1

Write-Host "=== Git 提交信息乱码修复脚本 ===" -ForegroundColor Cyan
Write-Host "`n警告: 此操作将重写 Git 历史，需要强制推送" -ForegroundColor Red
Write-Host "`n请确保:" -ForegroundColor Yellow
Write-Host "  1. 已备份当前代码" -ForegroundColor White
Write-Host "  2. 没有其他人在使用这个仓库" -ForegroundColor White
Write-Host "  3. 已了解强制推送的风险" -ForegroundColor White

$confirm = Read-Host "`n是否继续? (yes/no)"
if ($confirm -ne "yes") {
    Write-Host "已取消操作" -ForegroundColor Yellow
    exit
}

Write-Host "`n开始修复..." -ForegroundColor Cyan

# 1. 确保使用 UTF-8 编码
$env:LANG = "zh_CN.UTF-8"
$env:LC_ALL = "zh_CN.UTF-8"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

# 2. 显示需要修复的提交
Write-Host "`n需要修复的提交:" -ForegroundColor Yellow
Write-Host "  - 39f7aa7: feat: 集成 Worker 模块并优化工具列表超时配置" -ForegroundColor White
Write-Host "  - abfbb68: feat: 优化刷新按钮功能区分" -ForegroundColor White

Write-Host "`n修复步骤:" -ForegroundColor Yellow
Write-Host "  1. 运行: git rebase -i 5e8d437" -ForegroundColor White
Write-Host "  2. 在编辑器中，将以下提交的 'pick' 改为 'reword':" -ForegroundColor White
Write-Host "     - 39f7aa7" -ForegroundColor Gray
Write-Host "     - abfbb68" -ForegroundColor Gray
Write-Host "  3. 保存并关闭编辑器" -ForegroundColor White
Write-Host "  4. Git 会逐个打开编辑器，修改提交信息为:" -ForegroundColor White
Write-Host "     - feat: integrate Worker module and optimize tool list timeout config" -ForegroundColor Gray
Write-Host "     - feat: optimize refresh button functionality distinction" -ForegroundColor Gray
Write-Host "  5. 完成后运行: git push --force-with-lease origin master" -ForegroundColor White

Write-Host "`n或者使用英文提交信息（推荐）:" -ForegroundColor Yellow
Write-Host "  - feat: integrate Worker module and optimize tool list timeout config" -ForegroundColor Gray
Write-Host "  - feat: optimize refresh button functionality distinction" -ForegroundColor Gray

Write-Host "`n注意: 建议使用英文提交信息，避免编码问题" -ForegroundColor Yellow

