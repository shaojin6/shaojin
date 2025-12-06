# 修复 ansible-mcp-server 工具列表

## 问题描述

由于之前的 MySQL 数据库存储覆盖问题，`ansible-mcp-server` 从 `kubernetes-mcp-server` 中获取了工具值。这些 Kubernetes 工具不属于 `ansible-mcp-server`。

## 问题表现

从日志中可以看到：
```
Warning: Tool configuration_view already exists in client kubernetes-mcp-server, overwriting with ansible-mcp-server
Warning: Tool events_list already exists in client kubernetes-mcp-server, overwriting with ansible-mcp-server
...
```

`ansible-mcp-server` 显示了 21 个工具，但其中大部分是 Kubernetes 工具，不应该属于它。

## 正确的工具列表

### ansible-mcp-server 应该有的工具（8个）：
1. `list_inventory` - List Inventory
2. `list_hosts` - List Hosts
3. `validate_playbook` - Validate Playbook
4. `ping_hosts` - Ping Hosts
5. `run_ad_hoc` - Run Ad Hoc
6. `get_ansible_version` - Get Ansible Version
7. `generate_playbook` - Generate Playbook
8. `run_playbook` - Run Playbook

### kubernetes-mcp-server 应该有的工具（21个）：
1. `configuration_view` - Get Kubernetes configuration
2. `events_list` - List Kubernetes events
3. `helm_install` - Install Helm chart
4. `helm_list` - List Helm releases
5. `helm_uninstall` - Uninstall Helm chart
6. `namespaces_list` - List namespaces
7. `nodes_log` - Get node logs
8. `nodes_stats_summary` - Get node stats
9. `nodes_top` - Get node top
10. `pods_delete` - Delete pod
11. `pods_exec` - Execute command in pod
12. `pods_get` - Get pod
13. `pods_list` - List pods
14. `pods_list_in_namespace` - List pods in namespace
15. `pods_log` - Get pod logs
16. `pods_run` - Run pod
17. `pods_top` - Get pod top
18. `resources_create_or_update` - Create or update resource
19. `resources_delete` - Delete resource
20. `resources_get` - Get resource
21. `resources_list` - List resources

## 解决方案

### 方法1：通过 API 清理（推荐）

使用提供的 PowerShell 脚本：

```powershell
.\scripts\clear-ansible-tools-via-api.ps1
```

这个脚本会：
1. 获取 `ansible-mcp-server` 的当前配置
2. 过滤掉 Kubernetes 工具
3. 只保留 Ansible 工具
4. 更新数据库

### 方法2：通过 SQL 直接更新

如果 API 方法不可用，可以直接通过 SQL 更新：

#### 步骤1：查看当前工具

```sql
-- 查看 ansible-mcp-server 的工具
SELECT server_id, name, LENGTH(tools) as tools_length, LEFT(tools, 1000) as tools_preview 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';
```

#### 步骤2：清空工具（让系统重新获取）

```sql
-- 清空 ansible-mcp-server 的工具，让系统重新获取
UPDATE remote_mcp_configs 
SET tools = NULL, tools_last_update = NULL 
WHERE server_id = 'ansible-mcp-server';
```

#### 步骤3：验证

```sql
-- 验证工具已清空
SELECT server_id, name, tools IS NULL as tools_is_null 
FROM remote_mcp_configs 
WHERE server_id = 'ansible-mcp-server';
```

#### 步骤4：在 Web UI 中刷新工具

1. 进入"配置管理" -> "MCP 配置"
2. 找到 `ansible-mcp-server`
3. 点击"刷新远程工具"
4. 系统会从远程 MCP 服务器重新获取正确的工具列表

### 方法3：通过 Web UI 手动清理

1. 进入"配置管理" -> "MCP 配置"
2. 找到 `ansible-mcp-server`
3. 点击"编辑"
4. 在工具列表中，手动删除不属于 Ansible 的工具
5. 保存

**注意**：这种方法不推荐，因为工具列表可能很长，手动操作容易出错。

## 验证修复

修复后，验证以下内容：

1. **ansible-mcp-server 应该只有 8 个工具**：
   - 在 Web UI 中查看，应该显示 "8个工具"
   - 工具列表应该只包含 Ansible 相关工具

2. **kubernetes-mcp-server 应该仍有 21 个工具**：
   - 在 Web UI 中查看，应该显示 "21个工具"
   - 工具列表应该包含所有 Kubernetes 工具

3. **检查数据库**：
   ```sql
   -- 检查 ansible-mcp-server 的工具数量
   SELECT 
       server_id, 
       name,
       JSON_LENGTH(tools) as tool_count
   FROM remote_mcp_configs 
   WHERE server_id = 'ansible-mcp-server';
   
   -- 应该返回 tool_count = 8
   ```

## 预防措施

为了避免将来再次出现这个问题：

1. **确保每个 MCP 服务有唯一的 serverId**
2. **工具缓存时使用正确的 serverId 作为标识符**
3. **定期检查工具列表，确保没有交叉污染**

## 相关文件

- `scripts/clear-ansible-tools-via-api.ps1` - API 清理脚本
- `scripts/check-mcp-tools.ps1` - 检查工具脚本
- `scripts/fix-ansible-mcp-tools.ps1` - SQL 修复脚本（需要 MySQL 客户端）

