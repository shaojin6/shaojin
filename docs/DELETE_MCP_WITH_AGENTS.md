# 删除 MCP 服务时的注意事项

## 问题描述

当删除一个 MCP 服务时，如果存在 Agent 配置引用了该服务，服务重启后该 MCP 服务会被自动恢复。

## 原因分析

`restoreMissingMCPServicesFromAgents` 函数会在服务启动时：
1. 检查所有 Agent 配置
2. 找出被引用的 MCP serverId
3. 如果某个 MCP 服务不存在，会尝试恢复它

恢复优先级：
1. **从 MySQL 恢复**（如果 MySQL 中还存在）
2. **从文件备份恢复**（如果文件备份中还存在）
3. **创建默认配置**（如果都没有）

## 解决方案

### 方案1：删除 MCP 服务前，先删除或修改相关的 Agent 配置

**步骤**：
1. 在 Web UI 中查看所有 Agent 配置
2. 找出引用了要删除的 MCP 服务的 Agent
3. 删除这些 Agent，或者修改它们使用其他 MCP 服务
4. 然后再删除 MCP 服务

**优点**：
- 数据一致性更好
- 不会出现"僵尸"Agent 配置

**缺点**：
- 需要手动操作
- 如果 Agent 配置很多，操作繁琐

### 方案2：删除 MCP 服务时，自动处理相关的 Agent 配置

**实现**：
- 在 `DeleteRemoteMCP` 方法中，检查是否有 Agent 引用了该 MCP 服务
- 如果有，可以选择：
  - 删除这些 Agent（可能不合适）
  - 修改这些 Agent 使用其他 MCP 服务（需要指定默认值）
  - 提示用户先处理这些 Agent

**优点**：
- 自动化处理
- 避免数据不一致

**缺点**：
- 实现复杂
- 需要决定如何处理 Agent（删除还是修改）

### 方案3：修改恢复逻辑，不恢复已删除的 MCP 服务

**实现**：
- 在恢复逻辑中，如果 MySQL 中不存在（返回 nil），说明已被删除
- 不恢复已删除的 MCP 服务，即使有 Agent 引用

**优点**：
- 简单直接
- 尊重用户的删除操作

**缺点**：
- Agent 配置会引用不存在的 MCP 服务
- 可能导致 Agent 功能异常

## 当前状态

当前实现采用**方案3**的改进版本：
- 如果 MySQL 中不存在（返回 nil），说明已被删除
- 不会从 MySQL 恢复已删除的 MCP 服务
- 但会尝试从文件备份恢复（可能是误删，需要恢复）
- 如果文件备份也没有，会创建默认配置（确保 Agent 可以工作）

## 建议

**最佳实践**：
1. 删除 MCP 服务前，先检查是否有 Agent 引用
2. 如果有，先删除或修改这些 Agent
3. 然后再删除 MCP 服务

**检查方法**：
```bash
# 通过 API 检查
curl http://localhost:9090/api/config/agents | jq '.[] | select(.mcpServerId == "ansible-mcp-server")'
```

## 验证删除

删除 MCP 服务后，验证是否成功：

1. **检查 API 响应**：
   ```bash
   curl http://localhost:9090/api/config/remote-mcp
   ```

2. **检查数据库**：
   ```sql
   SELECT server_id, name FROM remote_mcp_configs WHERE server_id = 'ansible-mcp-server';
   ```

3. **检查日志**：
   - 应该看到 `[PersistentStore] Deleting Remote MCP config: ServerID=xxx, Name=xxx`
   - 应该看到 `[MySQLStore] Remote MCP config deleted: ServerID=xxx, Name=xxx, RowsAffected=1`
   - 应该看到 `[PersistentStore] Successfully deleted Remote MCP config 'xxx' from MySQL`

4. **重启服务**：
   - 如果 MySQL 中已删除，重启后不应该恢复
   - 如果仍有 Agent 引用，可能会从文件备份或创建默认配置恢复

