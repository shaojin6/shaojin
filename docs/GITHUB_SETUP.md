# GitHub 仓库配置指南

## 前置准备

✅ 已完成：
- [x] Git 仓库初始化
- [x] Git 用户信息配置（renshaojin <renshaojin@gmail.com>）
- [x] 初始提交创建
- [x] .gitignore 配置（已排除敏感文件和构建产物）

## 步骤 1：在 GitHub 上创建仓库

1. 登录 GitHub：https://github.com
2. 点击右上角的 **"+"** → **"New repository"**
3. 填写仓库信息：
   - **Repository name**: `k8s-mcp`（或你喜欢的名称）
   - **Description**: `Kubernetes MCP Agent with Web UI`
   - **Visibility**: 选择 Public 或 Private
   - ⚠️ **不要**勾选 "Initialize this repository with a README"（因为本地已有代码）
4. 点击 **"Create repository"**

## 步骤 2：在 Cursor 中配置远程仓库

### 方法 1：使用 HTTPS（推荐，简单）

在 Cursor 的终端中执行：

```powershell
# 替换 YOUR_USERNAME 为你的 GitHub 用户名
# 替换 YOUR_REPO_NAME 为你的仓库名称（通常是 k8s-mcp）

git remote add origin https://github.com/YOUR_USERNAME/YOUR_REPO_NAME.git
```

**示例**：
```powershell
git remote add origin https://github.com/renshaojin/k8s-mcp.git
```

### 方法 2：使用 SSH（需要配置 SSH 密钥）

如果你已经配置了 SSH 密钥：

```powershell
git remote add origin git@github.com:YOUR_USERNAME/YOUR_REPO_NAME.git
```

## 步骤 3：验证远程仓库配置

```powershell
git remote -v
```

应该看到类似输出：
```
origin  https://github.com/YOUR_USERNAME/YOUR_REPO_NAME.git (fetch)
origin  https://github.com/YOUR_USERNAME/YOUR_REPO_NAME.git (push)
```

## 步骤 4：推送到 GitHub

### 首次推送

```powershell
# 推送主分支到 GitHub
git push -u origin master
```

如果 GitHub 默认分支是 `main`，使用：
```powershell
git push -u origin master:main
```

### 身份验证

**如果使用 HTTPS**：
- 首次推送时，GitHub 会要求输入用户名和密码
- **密码**：需要使用 **Personal Access Token (PAT)**，而不是 GitHub 密码
- 如何创建 PAT：
  1. GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)
  2. 点击 "Generate new token (classic)"
  3. 设置权限：至少勾选 `repo` 权限
  4. 生成后复制 token（只显示一次）
  5. 推送时，用户名输入 GitHub 用户名，密码输入 PAT

**如果使用 SSH**：
- 确保已配置 SSH 密钥并添加到 GitHub
- 无需输入密码

## 步骤 5：验证推送结果

1. 在浏览器中打开你的 GitHub 仓库
2. 应该能看到所有代码文件
3. 检查提交历史，应该看到 "Initial commit: K8s MCP Agent with Web UI"

## 后续操作

### 日常推送代码

```powershell
# 1. 查看修改状态
git status

# 2. 添加修改的文件
git add .

# 3. 提交修改
git commit -m "描述你的修改"

# 4. 推送到 GitHub
git push
```

### 创建新分支

```powershell
# 创建并切换到新分支
git checkout -b feature/your-feature-name

# 推送新分支到 GitHub
git push -u origin feature/your-feature-name
```

### 查看分支

```powershell
# 查看本地分支
git branch

# 查看远程分支
git branch -r

# 查看所有分支
git branch -a
```

## 常见问题

### 问题 1：推送时提示 "remote: Support for password authentication was removed"

**原因**：GitHub 已禁用密码认证，需要使用 Personal Access Token

**解决**：
1. 创建 Personal Access Token（见步骤 4）
2. 使用 token 作为密码
3. 或者配置 SSH 密钥使用 SSH 方式

### 问题 2：推送时提示 "Permission denied"

**原因**：没有权限访问该仓库

**解决**：
1. 确认仓库名称和用户名正确
2. 确认你有该仓库的写入权限
3. 检查是否使用了正确的认证方式

### 问题 3：推送时提示 "Updates were rejected"

**原因**：远程仓库有本地没有的提交

**解决**：
```powershell
# 先拉取远程更改
git pull origin master --rebase

# 然后再推送
git push
```

### 问题 4：想修改远程仓库地址

```powershell
# 查看当前远程地址
git remote -v

# 修改远程地址
git remote set-url origin https://github.com/NEW_USERNAME/NEW_REPO_NAME.git

# 验证修改
git remote -v
```

## 在 Cursor 中使用 Git

### 使用 Cursor 的 Git 界面

1. 点击左侧边栏的 **源代码管理** 图标（或按 `Ctrl+Shift+G`）
2. 可以看到所有修改的文件
3. 点击文件旁边的 **"+"** 添加到暂存区
4. 在消息框中输入提交信息
5. 点击 **"提交"** 按钮
6. 点击 **"同步更改"** 或 **"推送"** 推送到 GitHub

### 使用终端命令

在 Cursor 底部打开终端（`Ctrl+``），然后使用 Git 命令。

## 安全建议

1. ✅ **不要提交敏感信息**：
   - `.env` 文件（已在 .gitignore 中）
   - 密码、Token、密钥文件
   - 数据库连接信息

2. ✅ **定期备份**：
   - 推送到 GitHub 本身就是一种备份
   - 考虑定期创建 release/tag

3. ✅ **使用分支**：
   - 主分支（master/main）保持稳定
   - 新功能在 feature 分支开发
   - 使用 Pull Request 合并代码

## 下一步

- [ ] 在 GitHub 上添加 README.md 的详细说明
- [ ] 添加 LICENSE 文件
- [ ] 配置 GitHub Actions 进行 CI/CD
- [ ] 添加 issue 模板和 PR 模板
- [ ] 创建第一个 release


