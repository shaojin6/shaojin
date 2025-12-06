# Ansible MCP Server 配置指南

## 概述

`ansible-mcp-server` 是一个基于 Python FastAPI 的 MCP 服务器，提供 Ansible 自动化工具。本文档说明如何将其集成到现有的 k8s-mcp 系统中。

## 项目结构

```
ansible-mcp-main/
├── main.py              # FastAPI 应用入口
├── mcp_server.py        # MCP 服务器实现
├── requirements.txt     # Python 依赖
├── Dockerfile          # Docker 构建文件
├── inventory.ini        # Ansible inventory 配置
└── README.md           # 项目说明
```

## 运行方式

### 方式1：本地 Python 运行（推荐用于开发测试）

#### 前置条件
1. Python 3.11+
2. 安装依赖：
   ```bash
   cd ansible-mcp-main
   pip install -r requirements.txt
   ```

#### 启动服务
```bash
cd ansible-mcp-main
uvicorn main:app --reload --host 0.0.0.0 --port 8080
```

**注意**：默认端口是 8080，如果与现有服务冲突，可以修改为其他端口（如 8081）。

### 方式2：Docker 运行（推荐用于生产环境）

#### 构建镜像
```bash
cd ansible-mcp-main
docker build -t ansible-mcp:latest .
```

#### 运行容器
```bash
docker run -d \
  --name ansible-mcp \
  -p 8080:8080 \
  -v $(pwd)/inventory.ini:/app/inventory.ini \
  ansible-mcp:latest
```

**端口映射**：`-p 8080:8080` 将容器内的 8080 端口映射到主机的 8080 端口。

**如果需要修改端口**：
```bash
docker run -d \
  --name ansible-mcp \
  -p 30091:8080 \
  -v $(pwd)/inventory.ini:/app/inventory.ini \
  ansible-mcp:latest
```
这样容器内的 8080 端口会映射到主机的 30091 端口。

### 方式3：Kubernetes 部署（推荐用于 K8s 环境）

可以创建 Kubernetes Deployment 和 Service，将服务暴露为 NodePort 或 LoadBalancer。

## MCP 协议支持

根据 `mcp_server.py` 代码，ansible-mcp-server 支持：
- **HTTP 传输**：通过 `mcp.mount_http()` 挂载
- **SSE 传输**：通过 `mcp.mount_sse()` 挂载

### SSE 端点
根据 README.md，SSE 端点为：`http://your-host-ip:8080/sse`

## 系统集成配置

### 1. 在 Web UI 中配置

在"配置管理-MCP配置"中添加新的 MCP 服务：

#### 配置参数
- **服务名称**：`ansible-mcp-server`
- **服务器标识符**：`ansible-mcp-server`（小写字母、数字、下划线、连字符，最多24字符）
- **类型**：`HTTP`
- **访问地址（BaseURL）**：
  - 本地运行：`http://localhost:8080/sse`
  - 远程运行：`http://11.0.1.110:30091/sse`（根据实际部署地址和端口）
- **超时时间**：`30` 秒
- **SSE 读取超时**：`300` 秒（5分钟）
- **工具端点**：留空（系统会自动发现）
- **请求头**：根据实际需要添加（如认证头）

#### 配置示例

**本地运行（同一台机器）**：
```json
{
  "name": "ansible-mcp-server",
  "serverId": "ansible-mcp-server",
  "type": "http",
  "baseUrl": "http://localhost:8080/sse",
  "timeout": 30,
  "sseReadTimeout": 300,
  "enabled": true
}
```

**远程运行（K8s 环境）**：
```json
{
  "name": "ansible-mcp-server",
  "serverId": "ansible-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30091/sse",
  "timeout": 30,
  "sseReadTimeout": 300,
  "enabled": true
}
```

### 2. 工具发现机制

系统会自动尝试以下方式发现工具：

1. **Dify SSE 模式**：
   - GET 请求到 `http://host:port/sse`
   - 等待 `event: endpoint` 事件
   - 获取实际端点 URL
   - POST JSON-RPC 请求到该端点

2. **StreamableHTTP 模式**：
   - 直接 POST JSON-RPC 请求到 BaseURL
   - 支持 JSON 和 SSE 两种响应格式

3. **普通 HTTP 模式**（如果 SSE 失败）：
   - 尝试常见路径：`/mcp/tools`, `/api/tools`, `/tools`, `/v1/tools`

### 3. 提供的工具

根据 `mcp_server.py`，ansible-mcp-server 提供以下工具：

1. **list_inventory** - 列出 Inventory 主机组/结构
2. **list_hosts** - 列出主机清单
3. **ping_hosts** - Ping 测试主机连通性
4. **run_ad_hoc** - 执行 Ansible ad-hoc 命令
5. **run_playbook** - 执行 Playbook（SSE 流式输出）
6. **validate_playbook** - 验证 Playbook 语法
7. **generate_playbook** - 生成 Playbook 文件
8. **get_ansible_version** - 获取 Ansible 版本

## 配置步骤

### 步骤1：启动 ansible-mcp-server

选择一种运行方式（本地、Docker 或 K8s），确保服务正常运行。

**验证服务是否启动**：
```bash
curl http://localhost:8080/docs  # 查看 FastAPI 文档
curl http://localhost:8080/sse  # 测试 SSE 端点
```

### 步骤2：在 Web UI 中添加配置

1. 打开 Web UI：`http://localhost:9090`
2. 进入"配置管理-MCP配置"
3. 点击"+ 添加 MCP 服务"
4. 填写配置信息：
   - 服务名称：`ansible-mcp-server`
   - 服务器标识符：`ansible-mcp-server`
   - 类型：`HTTP`
   - 访问地址：`http://11.0.1.110:30091/sse`（根据实际地址修改）
   - 超时时间：`30`
   - SSE 读取超时：`300`
5. 点击"保存"

### 步骤3：测试连接

1. 在 MCP 配置列表中，找到 `ansible-mcp-server`
2. 点击"测试"按钮
3. 查看测试结果：
   - 如果成功：显示"连接正常，已找到 X 个工具"
   - 如果失败：查看错误信息

### 步骤4：验证工具列表

1. 在 MCP 配置列表中，点击"刷新远程工具"按钮
2. 查看工具数量（应该显示 8 个工具）
3. 点击工具数量，查看工具列表

## 常见问题排查

### 问题1：工具发现失败（0 个工具）

**可能原因**：
1. SSE 端点不可访问
2. 端口配置错误
3. 网络连接问题

**排查步骤**：
1. 检查 ansible-mcp-server 是否正常运行：
   ```bash
   curl http://11.0.1.110:30091/sse
   ```
2. 检查防火墙规则，确保端口开放
3. 查看服务日志：`service.log`
4. 尝试手动测试 SSE 端点

### 问题2：SSE 连接超时

**可能原因**：
1. SSE 读取超时设置过短
2. 网络延迟较大

**解决方案**：
1. 增加 SSE 读取超时时间（如 600 秒）
2. 检查网络连接质量

### 问题3：工具调用失败

**可能原因**：
1. Ansible 环境未正确配置
2. Inventory 文件路径错误
3. SSH 连接问题

**排查步骤**：
1. 检查 ansible-mcp-server 日志
2. 验证 `inventory.ini` 文件是否存在
3. 测试 SSH 连接：
   ```bash
   ansible all -i inventory.ini -m ping
   ```

## 配置优化建议

### 1. 端口配置

如果 8080 端口被占用，可以：
- 修改 `main.py` 中的端口号
- 使用 Docker 端口映射
- 使用 Kubernetes Service 暴露不同端口

### 2. 超时配置

根据实际网络环境调整：
- **timeout**：HTTP 请求超时（建议 30-60 秒）
- **sseReadTimeout**：SSE 流读取超时（建议 300-600 秒）

### 3. Inventory 配置

确保 `inventory.ini` 文件正确配置：
- 主机列表
- SSH 连接参数
- 主机组定义

### 4. 安全配置

如果需要认证，可以在"请求头"中添加：
- `Authorization: Bearer <token>`
- 或其他自定义认证头

## 验证清单

- [ ] ansible-mcp-server 服务正常运行
- [ ] SSE 端点可访问（`http://host:port/sse`）
- [ ] 在 Web UI 中添加了 MCP 配置
- [ ] 测试连接成功
- [ ] 工具列表正确显示（8 个工具）
- [ ] 工具调用功能正常
- [ ] 服务重启后配置保留

## 相关文档

- `ansible-mcp-main/README.md` - Ansible MCP Server 项目说明
- `docs/MCP_CONFIG_PERSISTENCE_ISSUE.md` - MCP 配置持久化问题分析
- `docs/MCP_CONFIG_FIX_SUMMARY.md` - MCP 配置修复总结
- `docs/REMOTE_MCP_TROUBLESHOOTING.md` - 远程 MCP 故障排查

