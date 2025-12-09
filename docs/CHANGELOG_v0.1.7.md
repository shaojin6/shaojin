# 版本更新日志 v0.1.7

## 🎉 重大功能更新：Function Calling 支持

### ✨ 新增功能

#### 1. Function Calling 模式支持
- ✅ 支持标准的 Function Calling API（优先使用）
- ✅ 自动检测模型能力，智能选择最佳模式
- ✅ 支持 DashScope/Qwen、OpenAI 等模型的 Function Calling
- ✅ 自动回退到 Prompt-based 模式（如果不支持 Function Calling）

#### 2. 模型能力检测
- ✅ 新增模型能力检测模块（`internal/web/llm/capabilities.go`）
- ✅ 硬编码模型能力映射表，支持主流模型
- ✅ 自动检测并保存 Agent 策略

#### 3. 数据库增强
- ✅ `agents` 表新增 `strategy` 字段
- ✅ 自动数据库迁移（兼容现有数据库）
- ✅ Strategy 持久化存储

### 🔧 技术改进

#### LLM 客户端层
- ✅ 扩展 `Client` 接口，新增 `ChatWithTools()` 方法
- ✅ 实现 `DashScopeClient.ChatWithTools()`（使用 OpenAI 兼容接口）
- ✅ 实现 `OpenAIClient.ChatWithTools()`
- ✅ 扩展 `Message` 结构体，支持 `ToolCalls` 和 `ToolCallID`

#### Orchestrator 层
- ✅ 重构为两种模式：`chatWithFunctionCalling()` 和 `chatWithPromptBased()`
- ✅ 实现工具格式转换（MCP Tool → LLM Tool）
- ✅ 完善错误处理和自动回退机制

#### API 层
- ✅ Chat API 自动检测并保存 Agent Strategy
- ✅ 向后兼容，不影响现有功能

### 📊 支持的模型

#### Function Calling 支持
- DashScope/Qwen: qwen-max, qwen-plus, qwen-turbo, qwen-7b-chat
- OpenAI: gpt-4, gpt-4-turbo, gpt-3.5-turbo 等

#### Prompt-based 模式（后备）
- 所有模型（包括不支持 Function Calling 的模型）
- Ollama 等本地模型

### 🔒 兼容性

- ✅ **完全向后兼容**：保留所有现有功能
- ✅ **自动迁移**：数据库自动添加 `strategy` 字段
- ✅ **无破坏性变更**：现有配置和功能不受影响

### 🐛 修复

- ✅ 修复编译错误和代码质量问题
- ✅ 完善空指针检查
- ✅ 优化错误处理逻辑

### 📝 文档

- ✅ 新增 `FUNCTION_CALLING_IMPLEMENTATION_SUMMARY.md`
- ✅ 新增 `FUNCTION_CALLING_FINAL_CHECK.md`
- ✅ 新增 `FUNCTION_CALLING_RISK_ANALYSIS.md`

### 🎯 使用说明

#### 对于用户
- **无需手动配置**：系统自动检测模型能力并选择最佳模式
- **透明切换**：用户无感知，系统自动使用最适合的方式
- **完全兼容**：现有功能不受影响

#### 对于开发者
- **添加新模型**：在 `capabilities.go` 中添加模型能力映射
- **调试**：查看日志中的 `[Orchestrator] Using strategy: ...` 信息

### 📦 部署说明

1. **备份数据库**（重要）
2. **部署新代码**
3. **检查启动日志**，确认数据库迁移成功：
   ```
   [MySQLStore] Added strategy column to agents table
   ```
4. **测试核心功能**（对话、工具调用）
5. **监控日志**，关注是否有错误或警告

### ⚠️ 注意事项

1. **数据库迁移**：首次启动会自动添加 `strategy` 字段
2. **模型支持**：新模型如果不在映射表中，会默认使用 Prompt-based 模式
3. **回退机制**：Function Calling 失败时自动回退到 Prompt-based

### 🔄 从 v0.1.6 升级

1. 停止服务
2. 备份数据库
3. 部署新代码
4. 启动服务（自动执行数据库迁移）
5. 验证功能

---

**发布日期**：2024-12-09
**版本号**：v0.1.7
**主要变更**：Function Calling 支持

