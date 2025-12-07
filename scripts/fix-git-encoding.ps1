# Git 编码修复脚本
# 用于修复 Git 提交历史中的中文乱码问题

Write-Host "=== Git 编码修复脚本 ===" -ForegroundColor Cyan

# 1. 设置 Git 编码配置
Write-Host "`n1. 配置 Git 编码设置..." -ForegroundColor Yellow
git config --global core.quotepath false
git config --global i18n.commitencoding utf-8
git config --global i18n.logoutputencoding utf-8
git config --global core.precomposeunicode true

Write-Host "   ✓ Git 编码配置完成" -ForegroundColor Green

# 2. 检查当前配置
Write-Host "`n2. 当前 Git 配置:" -ForegroundColor Yellow
git config --get core.quotepath
git config --get i18n.commitencoding
git config --get i18n.logoutputencoding
git config --get core.precomposeunicode

# 3. 设置环境变量
Write-Host "`n3. 设置环境变量..." -ForegroundColor Yellow
$env:LANG = "zh_CN.UTF-8"
$env:LC_ALL = "zh_CN.UTF-8"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

Write-Host "   ✓ 环境变量设置完成" -ForegroundColor Green

# 4. 显示最近提交（用于验证）
Write-Host "`n4. 最近 5 个提交:" -ForegroundColor Yellow
git log --oneline -5 --encoding=utf-8

Write-Host "`n=== 完成 ===" -ForegroundColor Cyan
Write-Host "`n注意:" -ForegroundColor Yellow
Write-Host "  - 已存在的乱码提交需要重写历史才能修复" -ForegroundColor White
Write-Host "  - 未来提交将自动使用 UTF-8 编码" -ForegroundColor White
Write-Host "  - 如果重写历史，需要使用: git push --force" -ForegroundColor White

