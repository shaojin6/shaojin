# 切换到 qwen3-max 模型指南

## 概述

`qwen3-max` 是 DashScope（百炼平台）最新的最强模型，相比 `qwen-plus` 有更好的性能和推理能力。

## 如何切换到 qwen3-max

### 方法1：通过 Web 界面（推荐）

1. **打开配置页面**
   - 访问 `http://localhost:9090`
   - 点击左侧菜单的"配置管理" → "LLM 配置"

2. **编辑现有配置**
   - 找到你当前的 LLM 配置（通常是默认配置）
   - 点击"编辑"按钮

3. **修改模型**
   - 在"模型"下拉框中选择 **"qwen3-max（最新最强）"**
   - 其他配置保持不变（API 地址、API Key 等）
   - 点击"保存"

4. **测试连接**
   - 点击"测试连接"按钮，确认配置正确
   - 如果测试成功，说明已切换到 qwen3-max

### 方法2：通过 API

```bash
# 获取当前配置
curl http://localhost:9090/api/config/llm

# 更新模型为 qwen3-max
curl -X POST http://localhost:9090/api/config/llm \
  -H "Content-Type: application/json" \
  -d '{
    "id": "your-llm-config-id",
    "name": "阿里云百炼",
    "provider": "bailian",
    "baseUrl": "https://dashscope.aliyuncs.com/api/v1",
    "model": "qwen3-max",
    "apiKey": "your-api-key",
    "enabled": true,
    "isDefault": true
  }'
```

**注意**：需要替换：
- `your-llm-config-id` - 你的 LLM 配置 ID（从 GET 请求中获取）
- `your-api-key` - 你的 API Key

### 方法3：使用配置脚本

```powershell
# Windows PowerShell
.\scripts\configure-llm.ps1
```

运行脚本后，当提示输入模型名称时，输入 `qwen3-max`（或直接回车使用默认值）。

## 验证是否切换成功

### 1. 查看日志

发送一条测试消息后，查看日志：

```
[DashScopeClient] Model=qwen3-max
```

如果看到 `Model=qwen3-max`，说明切换成功。

### 2. 查看配置

访问 `http://localhost:9090/api/config/llm`，检查返回的 JSON 中 `model` 字段是否为 `qwen3-max`。

## qwen3-max 的特点

- **最新版本**：DashScope 最新最强的模型
- **更强性能**：相比 qwen-plus 有更好的推理能力
- **8k 上下文**：支持更长的对话历史（API 限定输入为 6k tokens）
- **适合复杂任务**：适合需要深度推理和高质量回答的场景

## 注意事项

1. **API Key 要求**：确保你的 API Key 有权限使用 qwen3-max 模型
2. **费用**：qwen3-max 可能比 qwen-plus 费用更高，请参考百炼平台计费说明
3. **响应时间**：qwen3-max 可能比 qwen-plus 响应稍慢，但质量更高
4. **兼容性**：使用相同的 API 端点和格式，无需修改代码

## 常见问题

### Q: 切换后测试连接失败？
A: 检查：
1. API Key 是否正确
2. API Key 是否有权限使用 qwen3-max
3. 网络连接是否正常

### Q: 如何确认当前使用的模型？
A: 查看日志文件 `service.log`，搜索 `[DashScopeClient] Model=`，可以看到当前使用的模型名称。

### Q: 可以同时配置多个模型吗？
A: 可以，但只有设置为 `isDefault: true` 的配置会被使用。

## 回退到 qwen-plus

如果需要回退到 qwen-plus，只需在配置中将模型改回 `qwen-plus` 即可。

