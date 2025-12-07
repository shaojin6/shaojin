# Git 中文编码问题修复指南

## 问题描述

在 GitHub 上查看提交历史时，中文提交信息显示为乱码（如：`浼樺寲鍒锋柊鎸夐挳鍔熻兘鍖哄垎`）。

## 原因分析

1. Git 提交时使用了错误的字符编码
2. Windows 系统默认编码可能不是 UTF-8
3. Git 配置未正确设置 UTF-8 编码

## 解决方案

### 1. 配置 Git 使用 UTF-8 编码

```powershell
# 设置 Git 编码配置
git config --global core.quotepath false
git config --global i18n.commitencoding utf-8
git config --global i18n.logoutputencoding utf-8
git config --global core.precomposeunicode true
```

### 2. 设置 PowerShell 环境变量

```powershell
$env:LANG = "zh_CN.UTF-8"
$env:LC_ALL = "zh_CN.UTF-8"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
```

### 3. 验证配置

```powershell
git config --get core.quotepath
git config --get i18n.commitencoding
git config --get i18n.logoutputencoding
git config --get core.precomposeunicode
```

应该显示：
```
false
utf-8
utf-8
true
```

## 修复已存在的乱码提交

### 选项 A：重写历史（推荐）

**注意：这会改变提交历史，需要强制推送**

1. 使用交互式 rebase 修复提交信息：

```bash
# 找到第一个乱码提交的父提交
git log --oneline

# 从第一个乱码提交之前开始 rebase
git rebase -i 5e8d437  # 使用 Initial commit 的 hash

# 在编辑器中，将要修改的提交前的 'pick' 改为 'reword'
# 保存并关闭编辑器

# Git 会逐个打开编辑器让你修改提交信息
# 将乱码的中文改为正确的 UTF-8 编码的中文
```

2. 强制推送（谨慎使用）：

```bash
git push --force-with-lease origin master
```

### 选项 B：保留现状，只修复未来提交

如果不想重写历史，可以：
1. 保留已存在的乱码提交
2. 确保未来所有提交使用 UTF-8 编码
3. 在提交信息中使用英文（避免编码问题）

## 未来提交最佳实践

### 1. 使用 UTF-8 编码提交

```bash
# 提交时确保使用 UTF-8
git commit -m "你的中文提交信息"
```

### 2. 使用英文提交信息（推荐）

```bash
# 使用英文提交信息，避免编码问题
git commit -m "feat: optimize refresh button functionality"
```

### 3. 检查文件编码

确保所有源代码文件使用 UTF-8 编码保存：
- Go 文件：UTF-8
- Vue/JavaScript 文件：UTF-8
- Markdown 文件：UTF-8

## 快速修复脚本

运行 `scripts/fix-git-encoding.ps1` 脚本自动配置：

```powershell
.\scripts\fix-git-encoding.ps1
```

## 验证修复

1. 查看最近提交：

```bash
git log --oneline -5 --encoding=utf-8
```

2. 在 GitHub 上查看提交历史，确认中文显示正常

## 注意事项

1. **重写历史的影响**：
   - 如果已经推送到远程仓库，重写历史需要强制推送
   - 其他协作者需要重新克隆仓库或重置本地分支
   - 建议在个人仓库或团队同意的情况下进行

2. **文件编码**：
   - 确保所有文件编辑器设置为 UTF-8 编码
   - VS Code: 右下角显示编码，点击可更改
   - 确保 `.gitattributes` 文件配置正确（如果需要）

3. **提交信息规范**：
   - 推荐使用英文提交信息，避免编码问题
   - 如果必须使用中文，确保使用 UTF-8 编码
   - 遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范

## 当前状态

- ✅ Git 编码配置已设置
- ✅ 未来提交将使用 UTF-8 编码
- ⚠️ 已存在的乱码提交需要手动修复（可选）

