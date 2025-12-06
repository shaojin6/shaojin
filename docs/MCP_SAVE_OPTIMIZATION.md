# MCP 配置保存优化

## 问题分析

### 问题1：保存很慢

**原因**：
- 保存后会调用 `await loadRemoteMCPs()`，重新加载所有 MCP 服务
- `loadRemoteMCPs()` 还会异步加载所有服务的工具列表
- 导致保存操作很慢

**解决方案**：
- 保存后只更新当前项，而不是重新加载所有数据
- 删除后只从列表中移除该项，而不是重新加载所有数据

### 问题2：点击 ansible-mcp-server 的保存，另一个 kubernetes-mcp-server 也会保存

**原因**：
- `editingItem.value` 是引用类型，可能被意外修改
- 保存时使用了错误的 `serverId`

**解决方案**：
- 在 `handleEdit` 中使用深拷贝，避免引用问题
- 在保存时确保使用正确的 `originalServerId`
- 添加防止重复提交的逻辑

## 优化内容

### 1. 防止重复提交

```javascript
const saveRemoteMCP = async () => {
  // 防止重复提交
  if (loading.value) {
    return
  }
  // ...
}
```

### 2. 深拷贝 editingItem

```javascript
const handleEdit = (mcp) => {
  // 深拷贝，避免引用问题
  editingItem.value = {
    name: mcp.name,
    serverId: mcp.serverId,
    // ... 其他字段
  }
  // ...
}
```

### 3. 确保使用正确的 serverId

```javascript
const originalServerId = editingItem.value ? editingItem.value.serverId : null

if (editingItem.value) {
  // 编辑时使用原始的 serverId（不可修改），确保使用正确的标识符
  if (!originalServerId) {
    ElMessage.error('无法确定要更新的服务标识符')
    return
  }
  await updateRemoteMCP(originalServerId, config)
  // ...
}
```

### 4. 优化保存逻辑（只更新当前项）

```javascript
// 只更新当前项，而不是重新加载所有数据（优化性能）
const index = remoteMCPs.value.findIndex(mcp => (mcp.serverId || mcp.name) === originalServerId)
if (index !== -1) {
  // 更新本地数据
  Object.assign(remoteMCPs.value[index], {
    name: config.name,
    serverId: config.serverId,
    // ... 其他字段
  })
}
```

### 5. 优化删除逻辑（只移除当前项）

```javascript
// 只从列表中移除该项，而不是重新加载所有数据（优化性能）
const index = remoteMCPs.value.findIndex(item => (item.serverId || item.name) === identifier)
if (index !== -1) {
  remoteMCPs.value.splice(index, 1)
}
```

## 性能提升

### 优化前
- 保存：需要重新加载所有 MCP 服务 + 异步加载所有工具列表
- 删除：需要重新加载所有 MCP 服务 + 异步加载所有工具列表
- 时间：可能需要几秒钟

### 优化后
- 保存：只更新当前项（毫秒级）
- 删除：只移除当前项（毫秒级）
- 时间：几乎瞬间完成

## 注意事项

1. ✅ **数据一致性**：虽然不再重新加载所有数据，但后端数据已经更新，前端只是同步更新本地状态
2. ✅ **错误处理**：如果保存失败，会显示错误消息，不会更新本地数据
3. ✅ **事件通知**：仍然会触发 `config-updated` 事件，通知其他组件更新

## 总结

通过优化保存和删除逻辑：
- ✅ **性能提升**：保存和删除操作从几秒钟降低到毫秒级
- ✅ **问题修复**：解决了保存时影响其他服务的问题
- ✅ **用户体验**：操作更加流畅，响应更快

