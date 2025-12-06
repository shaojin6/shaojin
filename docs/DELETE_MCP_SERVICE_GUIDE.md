# 删除 MCP 服务操作指南

## 操作流程

### 步骤1：检查是否有 Agent 引用该 MCP 服务

#### 方法1：通过 Web UI 检查

1. 进入"配置管理" -> "智能体配置"
2. 查看所有 Agent 配置
3. 找出 `MCP 服务器` 字段引用了要删除的 MCP 服务的 Agent

#### 方法2：通过 API 检查

```bash
# 获取所有 Agent 配置
curl http://localhost:9090/api/config/agents

# 或者使用 PowerShell
$agents = Invoke-RestMethod -Uri "http://localhost:9090/api/config/agents" -Method GET
$agents | Where-Object { $_.mcpServerId -eq "ansible-mcp-server" } | Format-Table id, name, mcpServerId
```

#### 方法3：通过数据库检查

```sql
-- 查找引用了 ansible-mcp-server 的 Agent
SELECT id, name, mcp_server_id 
FROM agents 
WHERE mcp_server_id = 'ansible-mcp-server';
```

### 步骤2：删除或修改相关的 Agent 配置

#### 选项A：删除 Agent（如果不再需要）

**通过 Web UI**：
1. 进入"配置管理" -> "智能体配置"
2. 找到引用了该 MCP 服务的 Agent
3. 点击"删除"按钮

**通过 API**：
```bash
# 删除 Agent
curl -X DELETE http://localhost:9090/api/config/agents/{agent-id}
```

#### 选项B：修改 Agent 使用其他 MCP 服务（如果需要保留 Agent）

**通过 Web UI**：
1. 进入"配置管理" -> "智能体配置"
2. 找到引用了该 MCP 服务的 Agent
3. 点击"编辑"按钮
4. 修改 `MCP 服务器` 字段，选择其他 MCP 服务（如 `kubernetes-mcp-server`）
5. 保存

**通过 API**：
```bash
# 获取 Agent 配置
curl http://localhost:9090/api/config/agents/{agent-id}

# 修改 Agent 配置（PUT 请求）
curl -X PUT http://localhost:9090/api/config/agents/{agent-id} \
  -H "Content-Type: application/json" \
  -d '{
    "id": "agent-id",
    "name": "Agent Name",
    "mcpServerId": "kubernetes-mcp-server",  # 修改为其他 MCP 服务
    ...
  }'
```

### 步骤3：删除 MCP 服务

**通过 Web UI**：
1. 进入"配置管理" -> "MCP 配置"
2. 找到要删除的 MCP 服务（如 `ansible-mcp-server`）
3. 点击"删除"按钮
4. 确认删除

**通过 API**：
```bash
# 删除 MCP 服务
curl -X DELETE http://localhost:9090/api/config/remote-mcp/ansible-mcp-server
```

### 步骤4：验证删除是否成功

#### 方法1：通过 API 验证

```bash
# 检查 MCP 服务列表
curl http://localhost:9090/api/config/remote-mcp

# 检查特定 MCP 服务是否存在（应该返回 404）
curl http://localhost:9090/api/config/remote-mcp/ansible-mcp-server
```

#### 方法2：通过数据库验证

```sql
-- 检查 MCP 服务是否已删除
SELECT server_id, name 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';

-- 如果查询返回空结果，说明删除成功
```

#### 方法3：检查服务日志

查看服务日志，应该看到：
```
[PersistentStore] Deleting Remote MCP config: ServerID=ansible-mcp-server, Name=ansible-mcp-server
[MySQLStore] Remote MCP config deleted: ServerID=ansible-mcp-server, Name=ansible-mcp-server, RowsAffected=1
[PersistentStore] Successfully deleted Remote MCP config 'ansible-mcp-server' (Name: ansible-mcp-server) from MySQL
```

### 步骤5：重启服务验证

1. 重启服务
2. 检查 MCP 服务列表，确认 `ansible-mcp-server` 不会重新出现
3. 如果仍然出现，说明：
   - 删除可能未成功（检查数据库）
   - 或者文件备份中仍然存在（检查 `.config/web-config.json`）

## 完整示例：删除 ansible-mcp-server

### 1. 检查 Agent 引用

```powershell
# PowerShell
$agents = Invoke-RestMethod -Uri "http://localhost:9090/api/config/agents" -Method GET
$ansibleAgents = $agents | Where-Object { $_.mcpServerId -eq "ansible-mcp-server" }
if ($ansibleAgents) {
    Write-Host "Found Agent(s) that reference ansible-mcp-server:"
    $ansibleAgents | ForEach-Object {
        Write-Host "  - Agent ID: $($_.id), Name: $($_.name)"
    }
} else {
    Write-Host "No Agent references ansible-mcp-server"
}
```

### 2. 删除或修改 Agent

```powershell
# 删除 Agent（示例）
# Invoke-RestMethod -Uri "http://localhost:9090/api/config/agents/agent-1764949387023840500" -Method DELETE

# 或者修改 Agent 使用其他 MCP 服务
# 先获取 Agent 配置
$agent = Invoke-RestMethod -Uri "http://localhost:9090/api/config/agents/agent-1764949387023840500" -Method GET
# 修改 mcpServerId
$agent.mcpServerId = "kubernetes-mcp-server"
# 更新 Agent
Invoke-RestMethod -Uri "http://localhost:9090/api/config/agents/agent-1764949387023840500" -Method PUT -Body ($agent | ConvertTo-Json) -ContentType "application/json"
```

### 3. 删除 MCP 服务

```powershell
# 删除 ansible-mcp-server
Invoke-RestMethod -Uri "http://localhost:9090/api/config/remote-mcp/ansible-mcp-server" -Method DELETE
```

### 4. 验证删除

```powershell
# 检查 MCP 服务列表
$mcps = Invoke-RestMethod -Uri "http://localhost:9090/api/config/remote-mcp" -Method GET
$ansibleExists = $mcps | Where-Object { $_.serverId -eq "ansible-mcp-server" }
if ($ansibleExists) {
    Write-Host "⚠️  ansible-mcp-server still exists!" -ForegroundColor Red
} else {
    Write-Host "✅ ansible-mcp-server has been deleted!" -ForegroundColor Green
}
```

### 5. 重启服务验证

```powershell
# 重启服务后再次检查
$mcps = Invoke-RestMethod -Uri "http://localhost:9090/api/config/remote-mcp" -Method GET
$ansibleExists = $mcps | Where-Object { $_.serverId -eq "ansible-mcp-server" }
if ($ansibleExists) {
    Write-Host "⚠️  ansible-mcp-server was restored after restart!" -ForegroundColor Yellow
    Write-Host "   This might be due to:" -ForegroundColor Yellow
    Write-Host "   - File backup still contains it (.config/web-config.json)" -ForegroundColor Yellow
    Write-Host "   - Or automatic restore logic created a default config" -ForegroundColor Yellow
} else {
    Write-Host "✅ ansible-mcp-server remains deleted after restart!" -ForegroundColor Green
}
```

## 注意事项

1. **删除顺序很重要**：先删除/修改 Agent，再删除 MCP 服务
2. **数据一致性**：确保没有其他配置引用该 MCP 服务
3. **备份**：删除前建议备份数据库或配置文件
4. **验证**：删除后务必验证，确保不会在重启后恢复

## 故障排查

### 问题1：删除后重启又出现了

**可能原因**：
- Agent 仍然引用该 MCP 服务
- 文件备份中仍然存在
- 自动恢复逻辑创建了默认配置

**解决方法**：
1. 检查是否还有 Agent 引用
2. 检查 `.config/web-config.json` 文件
3. 检查服务日志中的恢复信息

### 问题2：删除失败

**可能原因**：
- MySQL 连接问题
- 数据库权限问题
- 配置不存在

**解决方法**：
1. 检查服务日志
2. 检查 MySQL 连接状态
3. 直接查询数据库确认配置是否存在

