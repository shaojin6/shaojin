# Kubernetes MCP Server 丢失问题分析

## 问题描述

`kubernetes-mcp-server` 配置丢失，只剩下 `ansible-mcp-server`。

## 可能的原因

### 1. 数据库迁移问题

在服务重启时，如果数据库迁移逻辑有问题，可能导致数据丢失。

### 2. 保存操作覆盖

在保存 `ansible-mcp-server` 时，如果代码逻辑有问题，可能意外覆盖或删除了其他服务。

### 3. 数据加载问题

在服务启动时，如果数据加载逻辑有问题，可能只加载了部分数据。

## 解决方案

### 方案1：从 Agent 配置恢复

从文件备份中可以看到，Agent 配置中引用了 `kubernetes-mcp-server`，说明之前确实存在。

**kubernetes-mcp-server 的典型配置**：
```json
{
  "name": "kubernetes-mcp-server",
  "serverId": "kubernetes-mcp-server",
  "type": "http",
  "baseUrl": "http://11.0.1.110:30080/mcp",
  "icon": "M",
  "timeout": 30,
  "sseReadTimeout": 300,
  "headers": {},
  "toolsEndpoint": "",
  "enabled": true
}
```

### 方案2：手动重新添加

在 Web UI 中重新添加 `kubernetes-mcp-server` 配置。

### 方案3：检查数据库

直接查询数据库，看看是否还在数据库中。

## 预防措施

1. **定期备份数据库**
2. **检查保存逻辑**：确保保存一个服务时不会影响其他服务
3. **添加数据验证**：在保存前验证数据完整性

