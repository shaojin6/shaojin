# 远程 MCP 服务故障排查

## 常见错误

### 1. 404 错误：工具端点未找到

**错误信息**：`status 404, body: 404 page not found`

**可能原因**：
- 工具端点路径不正确
- MCP 服务未正确部署
- 服务地址有误

**解决方案**：

1. **检查服务地址**
   - 确保 URL 中没有空格
   - 确保 URL 格式正确（例如：`http://11.0.1.110:30080/mcp`）
   - 如果是集群内服务，使用完整 DNS：`http://service-name.namespace.svc.cluster.local:port`

2. **检查工具端点**
   - 在"高级选项"中指定正确的工具端点
   - 常见端点路径：
     - `/api/tools`
     - `/tools`
     - `/mcp/tools`
     - `/v1/tools`
   - 如果不确定，留空让系统自动尝试

3. **测试连接**
   - 先测试服务是否可达：`curl http://your-service-url/health` 或类似端点
   - 检查 MCP 服务的实际工具端点路径

### 2. 连接超时

**错误信息**：`failed to connect: timeout`

**解决方案**：
- 增加超时时间（默认 30 秒）
- 检查网络连接
- 确认服务正在运行

### 3. 认证失败

**错误信息**：`status 401` 或 `status 403`

**解决方案**：
- 检查请求头配置
- 确认 Token/API Key 正确
- 确认认证方式匹配（Bearer Token 或 API Key）

## 配置建议

### 集群内服务

```
服务终点 URL: http://k8s-mcp-service.dify.svc.cluster.local:8080
工具端点: /api/tools (或留空自动尝试)
请求头:
  - Authorization: Bearer your-token
```

### 外部服务

```
服务终点 URL: http://11.0.1.110:30080/mcp
工具端点: /tools (根据实际服务调整)
请求头:
  - Authorization: Bearer your-token
  - X-API-Key: your-api-key (如果需要)
```

## 调试步骤

1. **验证服务地址**
   ```bash
   curl http://your-service-url/health
   ```

2. **测试工具端点**
   ```bash
   curl http://your-service-url/api/tools
   ```

3. **检查请求头**
   - 确保在配置中添加了正确的认证头
   - 格式：`Authorization: Bearer token` 或 `X-API-Key: key`

4. **查看详细错误**
   - 测试按钮会显示详细的错误信息
   - 根据错误信息调整配置

## 自动端点发现

如果工具端点留空，系统会自动尝试以下路径：
1. `/api/tools`
2. `/tools`
3. `/mcp/tools`
4. `/v1/tools`

如果所有路径都失败，会显示详细的错误信息。

## 针对你的错误

根据你遇到的 `404 page not found` 错误，请检查：

1. **URL 格式**：确保是 `http://11.0.1.110:30080/mcp`（没有空格）
2. **工具端点**：你的服务可能使用不同的端点路径，例如：
   - `/mcp/tools`
   - `/api/v1/tools`
   - 或其他自定义路径
3. **服务状态**：确认服务在 `http://11.0.1.110:30080` 正常运行

建议在"高级选项"中尝试不同的工具端点路径，或联系服务提供方确认正确的端点路径。

