# LLM 调用验证指南

## 如何验证 qwen-plus 是否在使用

### 1. 查看日志输出

重启服务后，在控制台或日志文件中查找以下关键日志：

#### LLM 调用日志（Orchestrator 层）
```
[Orchestrator] ========== LLM Call #1 ==========
[Orchestrator] Calling LLM with X messages (context history included)
[Orchestrator] Message[0] Role=system, ContentLength=XXX, Preview=...
[Orchestrator] Message[1] Role=user, ContentLength=XXX, Preview=...
[Orchestrator] ✓ LLM response received in XXXms (length: XXX)
[Orchestrator] LLM Response Preview: ...
[Orchestrator] ========== End LLM Call #1 ==========
```

#### LLM API 调用日志（DashScopeClient 层）
```
[DashScopeClient] ========== LLM API Call ==========
[DashScopeClient] URL=https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation
[DashScopeClient] Model=qwen-plus
[DashScopeClient] Messages Count=X
[DashScopeClient] Message[0]: Role=system, Length=XXX, Preview=...
[DashScopeClient] Message[1]: Role=user, Length=XXX, Preview=...
[DashScopeClient] ✓ LLM API Response received: Length=XXX, Preview=...
[DashScopeClient] ========== End LLM API Call ==========
```

### 2. 验证上下文推理

#### 检查历史消息是否被包含
查找以下日志：
```
[Orchestrator] Context: SessionID=xxx, HistoricalMessages=X, TotalMessages=Y
[Orchestrator] Historical messages will be included in LLM context for reasoning
```

如果 `HistoricalMessages > 0`，说明历史消息已被包含在上下文中。

#### 检查消息数量
- **新会话**：`TotalMessages = 2`（system + user）
- **有历史的会话**：`TotalMessages = 2 + HistoricalMessages`

### 3. 验证 LLM 是否真正工作

#### 检查 LLM 响应
如果看到以下日志，说明 LLM 正在工作：
- `✓ LLM response received in XXXms`
- `LLM Response Preview: ...`（包含实际的响应内容）

#### 检查响应内容
LLM 的响应应该包含：
- JSON 格式的工具调用请求（如 `{"action": "call_tool", ...}`）
- 或者自然语言回答（如 `{"action": "respond", "reply": "..."}`）

### 4. 常见问题排查

#### 问题1：没有看到 LLM 调用日志
**可能原因：**
- LLM 配置未正确加载
- API Key 未设置或无效
- 网络连接问题

**解决方法：**
1. 检查日志中的 `[Chat API] Using configured default LLM: ...`
2. 确认 LLM 配置中的 API Key 已设置
3. 测试 LLM 连接（在配置页面点击"测试连接"）

#### 问题2：LLM 被调用但响应不智能
**可能原因：**
- 上下文历史未正确传递
- 系统提示词不够明确
- LLM 模型配置错误

**解决方法：**
1. 检查 `HistoricalMessages` 数量是否正确
2. 查看 `Message[X]` 日志，确认历史消息内容
3. 检查系统提示词（在 Agent 配置中）
4. 确认使用的模型是 `qwen-plus`

#### 问题3：上下文推理不工作
**可能原因：**
- 会话历史未保存
- 历史消息未正确加载

**解决方法：**
1. 检查 MongoDB 连接状态
2. 查看 `GetSession` 日志，确认历史消息是否被加载
3. 检查 `session.Messages` 是否包含历史消息

### 5. 测试步骤

1. **启动服务并查看日志**
   ```bash
   # Windows
   scripts\start-with-console.bat
   ```

2. **发送第一条消息**
   - 观察日志中的 `LLM Call #1`
   - 确认 `HistoricalMessages=0`（新会话）

3. **发送第二条消息（测试上下文）**
   - 观察日志中的 `LLM Call #1` 和 `LLM Call #2`
   - 确认 `HistoricalMessages > 0`（包含第一条消息）
   - 检查 LLM 是否能理解上下文关系

4. **验证推理能力**
   - 发送一个需要上下文的问题（如"刚才提到的 Pod 有多少个？"）
   - 检查 LLM 是否能正确引用之前的对话内容

### 6. 日志示例

#### 正常工作的日志示例
```
[Orchestrator] Context: SessionID=session_xxx_1234567890, HistoricalMessages=2, TotalMessages=4
[Orchestrator] Historical messages will be included in LLM context for reasoning
[Orchestrator] ========== LLM Call #1 ==========
[Orchestrator] Calling LLM with 4 messages (context history included)
[Orchestrator] Message[0] Role=system, ContentLength=1234, Preview=你是智能体...
[Orchestrator] Message[1] Role=user, ContentLength=50, Preview=查询zk命名空间下的Pod
[Orchestrator] Message[2] Role=assistant, ContentLength=200, Preview={"action": "call_tool"...
[Orchestrator] Message[3] Role=user, ContentLength=100, Preview=刚才提到的Pod有多少个？
[DashScopeClient] ========== LLM API Call ==========
[DashScopeClient] Model=qwen-plus
[DashScopeClient] Messages Count=4
[Orchestrator] ✓ LLM response received in 1.2s (length: 350)
[Orchestrator] LLM Response Preview: {"action": "respond", "reply": "根据刚才的查询结果..."}
```

### 7. 性能指标

正常情况下：
- **LLM API 调用耗时**：1-3 秒
- **总响应时间**：2-5 秒（包含工具调用）
- **上下文消息数量**：根据对话历史动态增长

如果响应时间过长（>10秒），可能是：
- 网络延迟
- LLM API 服务响应慢
- 工具调用超时



