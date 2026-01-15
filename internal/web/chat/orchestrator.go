package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/your-org/k8s-mcp-agent/internal/web/llm"
	"github.com/your-org/k8s-mcp-agent/internal/web/mcpclient"
	"github.com/your-org/k8s-mcp-agent/internal/web/store"
	"github.com/your-org/k8s-mcp-agent/internal/web/types"
	"github.com/your-org/k8s-mcp-agent/pkg/mcp"
)

// Orchestrator LLM 和工具协调器
type Orchestrator struct {
	llmClient    llm.Client
	toolManager  *mcpclient.ToolManager
	maxSteps     int
	sessionStore store.SessionStore
}

// Session 会话
type Session struct {
	ID        string
	AgentID   string
	Messages  []llm.Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewOrchestrator 创建新的协调器
func NewOrchestrator(llmClient llm.Client, toolManager *mcpclient.ToolManager, sessionStore store.SessionStore) *Orchestrator {
	return &Orchestrator{
		llmClient:    llmClient,
		toolManager:  toolManager,
		maxSteps:     10, // 最大工具调用次数
		sessionStore: sessionStore,
	}
}

// Chat 处理对话请求
func (o *Orchestrator) Chat(ctx context.Context, sessionID string, userMessage string, agent *types.AgentConfig, llmConfig *types.LLMConfig) (*types.ChatResponse, error) {
	// 获取或创建会话
	session, err := o.getOrCreateSession(ctx, sessionID, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create session: %w", err)
	}

	// 添加用户消息
	session.Messages = append(session.Messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})
	session.UpdatedAt = time.Now()

	// 获取该 Agent 可用的工具列表
	allowedTools := o.toolManager.ListToolsForAgent(agent)
	if len(allowedTools) == 0 {
		log.Printf("[Orchestrator] ERROR: Agent %s (ID: %s, MCP: %s) has no available tools", agent.Name, agent.ID, agent.MCPServerID)
		return nil, fmt.Errorf("所选智能体未关联任何可用工具，请检查 MCP 服务配置和工具列表")
	}
	log.Printf("[Orchestrator] Agent %s has %d available tools: %v", agent.Name, len(allowedTools), getToolNames(allowedTools))

	// 根据 Strategy 选择处理方式
	strategy := agent.Strategy
	if strategy == "" {
		// 如果没有 Strategy，检测并确定（这种情况应该不会发生，因为 API 层已经处理了）
		strategy = o.getOrDetermineStrategy(agent, llmConfig)
	}

	log.Printf("[Orchestrator] Using strategy: %s for agent %s", strategy, agent.Name)

	// 根据策略选择处理方式
	if strategy == "function_call" {
		return o.chatWithFunctionCalling(ctx, sessionID, session, userMessage, agent, llmConfig, allowedTools)
	} else {
		return o.chatWithPromptBased(ctx, sessionID, session, userMessage, agent, allowedTools)
	}
}

// chatWithPromptBased Prompt-based 模式处理（原有逻辑）
func (o *Orchestrator) chatWithPromptBased(ctx context.Context, sessionID string, session *Session, userMessage string, agent *types.AgentConfig, allowedTools []mcp.Tool) (*types.ChatResponse, error) {
	agentID := session.AgentID

	var steps []types.ChatStep
	var finalReply string
	
	// 检查是否有 scale 相关工具
	hasScaleTool := false
	scaleToolNames := []string{}
	for _, tool := range allowedTools {
		toolNameLower := strings.ToLower(tool.Name)
		if strings.Contains(toolNameLower, "scale") {
			hasScaleTool = true
			scaleToolNames = append(scaleToolNames, tool.Name)
		}
	}
	if hasScaleTool {
		log.Printf("[Orchestrator] Found scale tools: %v", scaleToolNames)
	} else {
		log.Printf("[Orchestrator] WARNING: No scale tools found in available tools")
	}

	allowedToolNames := make(map[string]struct{})
	for _, tool := range allowedTools {
		allowedToolNames[strings.ToLower(tool.Name)] = struct{}{}
	}

	// 构建系统提示词
	systemPrompt := o.buildSystemPrompt(agent, allowedTools)

	// 准备消息列表（包含系统提示词）
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}
	
	// 限制历史消息数量，只保留最近3轮对话（6条消息：3个user + 3个assistant）
	// 这样可以保持一定的上下文，但不会无限累加
	const maxHistoryRounds = 3
	const maxHistoryMessages = maxHistoryRounds * 2 // 每轮包含 user 和 assistant 两条消息
	
	var historicalMessages []llm.Message
	if len(session.Messages) > maxHistoryMessages {
		// 只保留最近的消息
		historicalMessages = session.Messages[len(session.Messages)-maxHistoryMessages:]
		log.Printf("[Orchestrator] Limiting history: %d total messages, keeping last %d messages", 
			len(session.Messages), len(historicalMessages))
	} else {
		historicalMessages = session.Messages
	}
	
	messages = append(messages, historicalMessages...)
	
	// 记录上下文信息
	log.Printf("[Orchestrator] Context: SessionID=%s, TotalHistoricalMessages=%d, UsingHistoricalMessages=%d, TotalMessages=%d", 
		sessionID, len(session.Messages), len(historicalMessages), len(messages))
	if len(historicalMessages) > 0 {
		log.Printf("[Orchestrator] Historical messages (last %d rounds) will be included in LLM context", len(historicalMessages)/2)
	} else {
		log.Printf("[Orchestrator] No historical messages (new session)")
	}

	// 记录总开始时间
	totalStartTime := time.Now()
	
	// 循环处理，直到 LLM 返回最终答案或达到最大步数
	for step := 0; step < o.maxSteps; step++ {
		// 检查已用时间
		elapsed := time.Since(totalStartTime)
		log.Printf("[Orchestrator] Step %d: Elapsed time: %v", step+1, elapsed)
		
		// 检查上下文是否已取消（超时）
		if ctx.Err() != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("[Orchestrator] ERROR: Request timeout at step %d (elapsed: %v)", step+1, elapsed)
				return nil, fmt.Errorf("请求超时：处理步骤 %d 时超过时间限制（已耗时 %v）。可能是 LLM 响应过慢、工具刷新耗时过长或进行了多轮 LLM 调用", step+1, elapsed)
			}
			return nil, fmt.Errorf("请求被取消: %w", ctx.Err())
		}

		log.Printf("[Orchestrator] Step %d: Calling LLM (elapsed: %v)", step+1, elapsed)

		// 调用 LLM
		log.Printf("[Orchestrator] ========== LLM Call #%d ==========", step+1)
		log.Printf("[Orchestrator] Calling LLM with %d messages (context history included)", len(messages))
		
		// 显示消息摘要，帮助调试上下文传递
		for i, msg := range messages {
			contentPreview := msg.Content
			if len(contentPreview) > 200 {
				contentPreview = contentPreview[:200] + "..."
			}
			log.Printf("[Orchestrator] Message[%d] Role=%s, ContentLength=%d, Preview=%s", 
				i, msg.Role, len(msg.Content), contentPreview)
		}
		
		llmStartTime := time.Now()
		elapsedBeforeLLM := time.Since(totalStartTime)
		log.Printf("[Orchestrator] LLM call starting (elapsed: %v, messages: %d)", elapsedBeforeLLM, len(messages))
		
		llmResponse, err := o.llmClient.Chat(messages)
		llmDuration := time.Since(llmStartTime)
		elapsedAfterLLM := time.Since(totalStartTime)
		
		if err != nil {
			log.Printf("[Orchestrator] ERROR: LLM call failed after %v (total elapsed: %v): %v", llmDuration, elapsedAfterLLM, err)
			// 区分不同类型的错误
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
				return nil, fmt.Errorf("LLM 调用超时（单次耗时 %v，总耗时 %v）。请检查 LLM 服务连接和网络状况", llmDuration, elapsedAfterLLM)
			}
			return nil, fmt.Errorf("LLM 调用失败: %w", err)
		}
		
		// 显示LLM响应的详细信息
		responsePreview := llmResponse
		if len(responsePreview) > 500 {
			responsePreview = responsePreview[:500] + "..."
		}
		log.Printf("[Orchestrator] ✓ LLM response received in %v (total elapsed: %v, length: %d)", llmDuration, elapsedAfterLLM, len(llmResponse))
		if llmDuration > 30*time.Second {
			log.Printf("[Orchestrator] WARNING: LLM call took too long (%v), this may cause timeout issues", llmDuration)
		}
		log.Printf("[Orchestrator] LLM Response Preview: %s", responsePreview)
		log.Printf("[Orchestrator] ========== End LLM Call #%d ==========", step+1)

		// 记录 LLM 响应
		steps = append(steps, types.ChatStep{
			Type: "llm",
			Text: llmResponse,
		})

		// 尝试解析 LLM 响应，看是否需要调用工具
		action, toolName, toolArgs, thought, reply := o.parseLLMResponse(llmResponse, allowedTools)

		if action == "call_tool" && toolName != "" {
			if _, ok := allowedToolNames[strings.ToLower(toolName)]; !ok {
				log.Printf("[Orchestrator] Tool %s is not available for agent %s", toolName, agentID)
				messages = append(messages, llm.Message{
					Role:    "assistant",
					Content: llmResponse,
				})
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("工具 %s 不在当前智能体的可用列表，请仅使用提供的工具。", toolName),
				})
				continue
			}

			// 需要调用工具
			log.Printf("[Orchestrator] LLM requested tool: %s with args: %+v", toolName, toolArgs)

			steps = append(steps, types.ChatStep{
				Type:      "tool",
				Tool:      toolName,
				Arguments: toolArgs,
			})

		// 调用工具
		elapsedBeforeTool := time.Since(totalStartTime)
		log.Printf("[Orchestrator] Calling tool: %s with args: %+v (elapsed: %v)", toolName, toolArgs, elapsedBeforeTool)
		
		// 检查上下文是否已取消（超时）
		if ctx.Err() != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("[Orchestrator] ERROR: Request timeout before calling tool %s (elapsed: %v)", toolName, elapsedBeforeTool)
				log.Printf("[Orchestrator] Timeout analysis: Total elapsed=%v, LLM calls=%d, Last LLM duration=%v", 
					elapsedBeforeTool, step+1, llmDuration)
				return nil, fmt.Errorf("请求超时：在调用工具 %s 前超过时间限制（已耗时 %v，进行了 %d 轮 LLM 调用）。可能原因：1) LLM 响应过慢 2) 工具刷新耗时过长 3) 多轮对话累积耗时", toolName, elapsedBeforeTool, step+1)
			}
			return nil, fmt.Errorf("请求被取消: %w", ctx.Err())
		}

			toolStartTime := time.Now()
			toolResult, err := o.toolManager.CallTool(toolName, toolArgs)
			toolDuration := time.Since(toolStartTime)
			if err != nil {
				// 工具调用失败，将错误信息反馈给 LLM
				errorMsg := fmt.Sprintf("Tool %s failed: %v", toolName, err)
				log.Printf("[Orchestrator] ERROR: Tool %s failed after %v: %s", toolName, toolDuration, errorMsg)
				
				// 区分不同类型的错误
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
					return nil, fmt.Errorf("工具 %s 调用超时（耗时 %v）。请检查 MCP 服务 %s 的连接和响应速度", toolName, toolDuration, agent.MCPServerID)
				}
				
				// 检查是否是权限错误
				isPermissionError := strings.Contains(strings.ToLower(err.Error()), "forbidden") ||
					strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
					strings.Contains(strings.ToLower(err.Error()), "permission") ||
					strings.Contains(strings.ToLower(err.Error()), "access denied")
				
				var userErrorMsg string
				if isPermissionError {
					userErrorMsg = fmt.Sprintf(`工具 %s 调用失败：权限不足。

错误信息：%s

可能的原因：
1. Kubernetes ServiceAccount 没有足够的权限执行此操作
2. RBAC 配置限制了此操作
3. 需要检查 Kubernetes 集群的权限配置

建议：
- 检查 ServiceAccount 的 RBAC 配置
- 确认是否有执行此操作的权限（如 scale StatefulSet、edit 资源等）
- 如果权限不足，请联系集群管理员添加相应权限

如果确实有权限，请重试或尝试使用其他工具。`, toolName, err.Error())
				} else {
					userErrorMsg = fmt.Sprintf("工具调用失败: %s。请重试或提供其他解决方案。", errorMsg)
				}

				messages = append(messages, llm.Message{
					Role:    "assistant",
					Content: llmResponse,
				})
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: userErrorMsg,
				})

				steps[len(steps)-1].Result = map[string]interface{}{
					"error": errorMsg,
				}
				continue
			}

			// 工具调用成功
			log.Printf("[Orchestrator] Tool %s called successfully", toolName)
			var toolResultData interface{}
			if toolResult != nil && len(toolResult.Content) > 0 {
				// 解析工具返回的 JSON
				if err := json.Unmarshal([]byte(toolResult.Content[0].Text), &toolResultData); err != nil {
					toolResultData = toolResult.Content[0].Text
				}
				log.Printf("[Orchestrator] Tool %s returned result (length: %d)", toolName, len(toolResult.Content[0].Text))
				// 记录工具返回结果的前500个字符，用于调试
				resultPreview := toolResult.Content[0].Text
				if len(resultPreview) > 500 {
					resultPreview = resultPreview[:500] + "..."
				}
				log.Printf("[Orchestrator] Tool %s result preview: %s", toolName, resultPreview)
			} else {
				log.Printf("[Orchestrator] WARNING: Tool %s returned empty result", toolName)
			}

			// 更新工具步骤的结果
			steps[len(steps)-1].Result = toolResultData

			// 将工具结果添加到上下文
			toolResultText := ""
			if toolResult != nil && len(toolResult.Content) > 0 {
				toolResultText = toolResult.Content[0].Text
			}

			// 记录工具调用完成
			log.Printf("[Orchestrator] Tool %s completed, result length: %d", toolName, len(toolResultText))
			
			// 检查工具是否返回错误
			hasError := strings.Contains(strings.ToLower(toolResultText), "failed") ||
				strings.Contains(strings.ToLower(toolResultText), "error") ||
				strings.Contains(strings.ToLower(toolResultText), "yaml") ||
				strings.Contains(strings.ToLower(toolResultText), "json") ||
				strings.Contains(strings.ToLower(toolResultText), "schema") ||
				strings.Contains(strings.ToLower(toolResultText), "field not declared")
			
			// 检查是否是 Kubernetes 冲突错误
			hasConflict := strings.Contains(strings.ToLower(toolResultText), "conflict") ||
				strings.Contains(strings.ToLower(toolResultText), "kubectl-edit") ||
				strings.Contains(strings.ToLower(toolResultText), "apply failed")
			
			// 检查是否是 YAML/JSON 格式错误
			hasFormatError := strings.Contains(strings.ToLower(toolResultText), "did not find expected") ||
				strings.Contains(strings.ToLower(toolResultText), "converting yaml to json") ||
				strings.Contains(strings.ToLower(toolResultText), "invalid json")
			
			// 检查是否是权限错误
			hasPermissionError := strings.Contains(strings.ToLower(toolResultText), "forbidden") ||
				strings.Contains(strings.ToLower(toolResultText), "unauthorized") ||
				strings.Contains(strings.ToLower(toolResultText), "permission") ||
				strings.Contains(strings.ToLower(toolResultText), "access denied") ||
				strings.Contains(strings.ToLower(toolResultText), "cannot") ||
				strings.Contains(strings.ToLower(toolResultText), "not allowed")
			
			// 检查是否是创建/更新操作的工具
			isCreateOrUpdate := strings.Contains(strings.ToLower(toolName), "create") || 
				strings.Contains(strings.ToLower(toolName), "update") ||
				strings.Contains(strings.ToLower(toolName), "apply") ||
				strings.Contains(strings.ToLower(toolName), "patch")

			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: llmResponse,
			})
			// 构建强化的提示词，确保LLM只使用工具返回的真实数据
			// 检查结果是否为空
			isEmpty := toolResultText == "" || toolResultText == "[]" || toolResultText == "{}" || 
				strings.TrimSpace(toolResultText) == "" || 
				(strings.HasPrefix(toolResultText, "{") && strings.Contains(toolResultText, `"items":[]`)) ||
				(strings.HasPrefix(toolResultText, "{") && strings.Contains(toolResultText, `"pods":[]`)) ||
				(strings.HasPrefix(toolResultText, "{") && strings.Contains(toolResultText, `"deployments":[]`))
			
			var userPrompt string
			if hasPermissionError && isCreateOrUpdate && strings.Contains(strings.ToLower(toolName), "resources_create_or_update") {
				// 权限错误 + resources_create_or_update：引导使用更简单的工具
				userPrompt = fmt.Sprintf(`【重要】工具 %s 执行失败（可能是权限问题）：

工具返回的错误：%s

**用户的问题**："%s"

【关键任务 - 尝试使用更简单的工具】
工具执行失败，可能是权限不足或操作过于复杂。对于 scale 操作（修改副本数），你应该：

1. **优先使用专门的 scale 工具**：
   - 检查可用工具列表中是否有 scale 相关工具（如 statefulset_scale、deployment_scale、resources_scale 等）
   - 这些工具专门用于 scale 操作，更简单、更可靠，通常权限要求也更低
   - 如果用户要 scale StatefulSet（如 zk），应该使用 statefulset_scale 或 resources_scale

2. **如果确实没有 scale 工具**：
   - 先调用 resources_get 获取资源的最新状态
   - 然后使用 resources_create_or_update，但只修改 spec.replicas 字段
   - 确保 resource 参数是有效的 JSON 字符串

3. **检查权限**：
   - 如果所有工具都失败，可能是 Kubernetes ServiceAccount 权限不足
   - 需要检查 RBAC 配置，确认是否有执行 scale 操作的权限

**回复格式**：
优先尝试 scale 工具：
{"action": "call_tool", "tool": "statefulset_scale", "arguments": {"name": "资源名称", "namespace": "命名空间", "replicas": 副本数}, "thought": "resources_create_or_update 可能权限不足，尝试使用专门的 scale 工具", "reply": "检测到权限问题，尝试使用更简单的 scale 工具..."}
或者：
{"action": "call_tool", "tool": "resources_scale", "arguments": {"apiVersion": "apps/v1", "kind": "StatefulSet", "name": "资源名称", "namespace": "命名空间", "replicas": 副本数}, ...}

请优先尝试使用 scale 工具，而不是 resources_create_or_update。`, toolName, toolResultText, userMessage)
			} else if hasConflict && isCreateOrUpdate {
				// Kubernetes 冲突错误：资源正在被其他操作修改
				userPrompt = fmt.Sprintf(`【重要】工具 %s 执行失败（Kubernetes 资源冲突）：

工具返回的错误：%s

**用户的问题**："%s"

【关键任务 - 解决资源冲突的智能方案】
工具执行失败，错误信息显示资源正在被其他操作修改（可能是 kubectl edit 或其他操作）。你需要采用更智能的解决方案：

**方案一：删除并重新创建（推荐，适用于 StatefulSet/Deployment）**
如果资源冲突无法解决，可以采用删除并重新创建的方式：
1. **第一步**：调用 resources_get 获取资源的完整定义（包括所有配置）
2. **第二步**：提取资源的关键信息（name、namespace、apiVersion、kind、spec 等）
3. **第三步**：调用 resources_delete 删除冲突的资源
4. **第四步**：基于获取的资源定义，修改需要更改的字段（如 spec.replicas），然后调用 resources_create_or_update 重新创建

**方案二：获取最新状态后重新应用（如果删除不可行）**
1. **第一步**：调用 resources_get 获取资源的最新状态
2. **第二步**：基于最新资源定义，重新构建要修改的字段（如 replicas）
3. **第三步**：使用更新后的资源定义重新调用 resources_create_or_update

**重要提示**：
- 对于 StatefulSet/Deployment 的 scale 操作，如果遇到冲突，优先使用方案一（删除并重新创建）
- 删除 StatefulSet 不会删除 Pod（如果设置了保留策略），重新创建后会继续管理现有 Pod
- resource 参数必须是有效的 JSON 字符串
- 确保 JSON 格式正确（所有字符串用双引号，正确转义特殊字符）

**回复格式**：
优先使用方案一（删除并重新创建）：
第一步：{"action": "call_tool", "tool": "resources_get", "arguments": {"apiVersion": "apps/v1", "kind": "StatefulSet", "name": "资源名称", "namespace": "命名空间"}, "thought": "资源冲突，先获取完整资源定义以便重新创建", "reply": "检测到资源冲突，正在获取资源完整定义..."}
第二步：{"action": "call_tool", "tool": "resources_delete", "arguments": {"apiVersion": "apps/v1", "kind": "StatefulSet", "name": "资源名称", "namespace": "命名空间"}, "thought": "删除冲突的资源以便重新创建", "reply": "正在删除冲突的资源..."}
第三步：{"action": "call_tool", "tool": "resources_create_or_update", "arguments": {"resource": "基于获取的资源定义构建的完整 JSON 字符串（已修改 spec.replicas）"}, "thought": "重新创建资源，副本数已正确设置", "reply": "正在重新创建资源..."}

**重要**：操作完成后，必须调用工具检查资源状态（如 pods_list_in_namespace 或 resources_get），然后提供详细的操作总结，包括：
1. 问题诊断（说明发现了什么问题）
2. 执行步骤（说明采取了什么操作）
3. 验证结果（说明操作是否成功）
4. 当前状态（说明资源的当前状态，如 Pod 是否正在创建）
5. 后续建议（告诉用户下一步该做什么）

请采用智能方案解决资源冲突，并在操作完成后提供详细的操作总结。`, toolName, toolResultText, userMessage)
			} else if hasFormatError && isCreateOrUpdate {
				// YAML/JSON 格式错误
				userPrompt = fmt.Sprintf(`【重要】工具 %s 执行失败（资源定义格式错误）：

工具返回的错误：%s

**用户的问题**："%s"

【关键任务 - 修复资源定义格式】
工具执行失败，错误信息显示资源定义的 JSON 格式不正确。你需要：

1. **分析错误原因**：
   - 错误 "yaml: did not find expected ',' or '}'" 表示 JSON 格式不正确
   - 可能的原因：
     * JSON 字符串中有未转义的特殊字符（如换行符、引号等）
     * JSON 结构不完整（缺少括号、逗号等）
     * 字符串值没有正确用引号包裹
     * JSON 字符串中的引号没有正确转义（应该使用 \\"）

2. **修复 JSON 格式**：
   - 从之前的工具调用中获取原始资源定义
   - 确保 resource 参数是**有效的 JSON 字符串**（不是 YAML）
   - 修复格式问题：
     * 所有字符串值都用双引号包裹（不是单引号）
     * 所有 JSON 字符串中的双引号必须转义为 \\"
     * 换行符必须转义为 \\n
     * 确保 JSON 结构完整（所有括号匹配，所有逗号正确）
     * 确保没有尾随逗号

3. **重新调用工具**：
   - 使用修复后的资源定义重新调用 resources_create_or_update
   - 确保 resource 参数是有效的 JSON 字符串

**重要提示**：
- resource 参数必须是**有效的 JSON 字符串**（不是 YAML）
- 如果 resource 中包含多行文本，必须使用 \\n 转义换行符
- JSON 字符串中的所有双引号必须转义为 \\"
- 确保 JSON 结构完整：所有括号匹配，所有逗号正确
- 示例格式：{"resource": "{\\"apiVersion\\":\\"apps/v1\\",\\"kind\\":\\"StatefulSet\\",\\"metadata\\":{...},\\"spec\\":{...}}"}

**回复格式**：
使用 JSON 格式重新调用工具，示例：
{"action": "call_tool", "tool": "resources_create_or_update", "arguments": {"resource": "修复后的完整 JSON 字符串（所有引号已转义）"}, "thought": "分析格式错误并修复 JSON 字符串", "reply": "检测到资源定义格式错误，正在修复并重新执行..."}

请分析格式错误，修复 JSON 字符串，然后重新调用工具。`, toolName, toolResultText, userMessage)
			} else if hasError && isCreateOrUpdate {
				// 其他错误（如 schema 错误）
				userPrompt = fmt.Sprintf(`【重要】工具 %s 执行失败（来自 Kubernetes API 的错误信息）：

工具返回的错误：%s

**用户的问题**："%s"

【关键任务 - 修复资源定义】
工具执行失败，错误信息显示资源定义有问题。你需要：

1. **分析错误原因**：
   - 如果错误是 "field not declared in schema"（如 ".updateStrategy"）：说明字段位置或格式不正确：
     * StatefulSet 的 updateStrategy 应该在 spec.updateStrategy，格式：{"type": "RollingUpdate"} 或 {"type": "OnDelete"}
     * 不要将 updateStrategy 放在错误的位置
     * 确保字段名称拼写正确（注意大小写）
   - 其他错误：仔细阅读错误信息，找出问题所在

2. **修复资源定义**：
   - 从之前的工具调用中获取原始资源定义
   - 修复字段问题：
     * 确保 updateStrategy 在 spec 下：spec.updateStrategy = {"type": "RollingUpdate"}
     * 移除或修正不在 schema 中的字段
     * 参考 Kubernetes API 文档确保字段格式正确
   - 修复 JSON 格式问题：
     * 确保所有字符串值都用双引号包裹
     * 确保 JSON 结构完整（所有括号匹配）
     * 如果 resource 参数是字符串，确保 JSON 字符串正确转义（使用 \\n 表示换行，\\" 表示引号）

3. **重新调用工具**：
   - 使用修复后的资源定义重新调用 resources_create_or_update
   - 确保 resource 参数是有效的 JSON 字符串

**重要提示**：
- resource 参数必须是**有效的 JSON 字符串**（不是 YAML）
- 如果 resource 中包含多行文本，必须使用 \\n 转义换行符
- StatefulSet 的 updateStrategy 格式：{"type": "RollingUpdate"} 或 {"type": "OnDelete"}
- 确保所有字段都在正确的位置（metadata 在顶层，spec 在顶层）

**回复格式**：
使用 JSON 格式重新调用工具，示例：
{"action": "call_tool", "tool": "resources_create_or_update", "arguments": {"resource": "修复后的完整 JSON 字符串"}, "thought": "分析错误原因并修复资源定义", "reply": "检测到资源定义错误，正在修复并重新执行..."}

请分析错误原因，修复资源定义，然后重新调用工具。`, toolName, toolResultText, userMessage)
			} else if isEmpty {
				// 结果为空时的特殊处理
				userPrompt = fmt.Sprintf(`【重要】工具 %s 的执行结果为空（来自 Kubernetes API 的实时数据）：

工具返回：%s

【关键理解】
用户说："%s"

请仔细分析用户的真实意图：
1. **用户是否提到了资源名称**（如 kafka）？这说明资源是存在的
2. **用户是否提到了操作**（如 scale、设置副本、重启等）？这说明用户想要执行操作，而不是查询
3. **如果用户说"我的kafka被我通过scale将副本设置为0，将帮我将副本设置为1"**：
   - 这说明：kafka 资源是存在的，只是副本数被设置为0了
   - 用户的需求是：恢复副本数为1（这是一个操作请求）
   - 你需要：先找到 kafka 资源，然后执行相应的操作

【当前情况】
通过标签查询没有找到资源。但这不意味着资源不存在，可能是因为：
1. 查询条件不准确（标签选择器、资源名称等）
2. 资源在不同的命名空间
3. 资源使用了不同的名称或标签

【你必须执行的操作】
**绝对不能只回复"没有找到"，必须立即尝试其他查询方式：**

1. **立即调用工具列出所有相关资源**（不指定命名空间，查询所有命名空间）：
   - 如果查询的是 Deployment，调用工具列出所有 Deployment
   - 如果查询的是 Pod，调用工具列出所有 Pod
   - 让用户从列表中找到正确的资源

2. **使用 JSON 格式调用工具**：
   {
     "action": "call_tool",
     "tool": "list_deployments" 或 "resources_list",
     "arguments": {"namespace": ""} 或 {"apiVersion": "apps/v1", "kind": "Deployment"},
     "thought": "通过标签查询失败，我将列出所有 Deployment 来查找 kafka 资源",
     "reply": "没有找到标签为 app=kafka 的资源。我将列出所有 Deployment 来查找 kafka 资源，请稍候..."
   }

3. **找到资源后，根据用户的意图执行相应操作**：
   - 如果用户想要 scale，找到资源后调用相应的工具执行 scale 操作
   - 如果用户想要查询，找到资源后提供查询结果

【回复要求】
- **必须调用工具列出所有相关资源**，不能只回复"没有找到"
- 使用 JSON 格式调用工具
- 回复要详细、有帮助，体现你的推理过程

用户的原始问题：%s

【重要】你必须立即调用工具列出所有相关资源，不要只回复"没有找到"。`, toolName, toolResultText, userMessage, userMessage)
			} else {
				// 正常结果处理
				// 检查是否是创建/更新操作的工具
				isCreateOrUpdate := strings.Contains(strings.ToLower(toolName), "create") || 
					strings.Contains(strings.ToLower(toolName), "update") ||
					strings.Contains(strings.ToLower(toolName), "apply") ||
					strings.Contains(strings.ToLower(toolName), "patch")
				
				if isCreateOrUpdate {
					// 对于创建/更新操作，要求简洁的成功提示，并自动检查资源状态
					// 检查是否是 StatefulSet、Deployment 等会创建 Pod 的资源
					isPodCreatingResource := strings.Contains(strings.ToLower(toolResultText), "statefulset") ||
						strings.Contains(strings.ToLower(toolResultText), "deployment") ||
						strings.Contains(strings.ToLower(toolResultText), "replicaset") ||
						strings.Contains(strings.ToLower(userMessage), "zookeeper") ||
						strings.Contains(strings.ToLower(userMessage), "zk") ||
						strings.Contains(strings.ToLower(userMessage), "pod") ||
						strings.Contains(strings.ToLower(userMessage), "启动") ||
						strings.Contains(strings.ToLower(userMessage), "启动")
					
					if isPodCreatingResource {
						// 需要检查 Pod 状态
						userPrompt = fmt.Sprintf(`【重要】工具 %s 已成功执行（来自 Kubernetes API 的实时数据）：

工具返回：%s

**用户的问题**："%s"

【关键任务 - 操作后检查】
工具已经成功执行了创建/更新操作（如 scale StatefulSet/Deployment）。现在你需要：

1. **立即检查 Pod 状态**：
   - 从工具返回的结果中提取资源名称和命名空间（如 zk StatefulSet 在 paas-public 命名空间）
   - **必须立即调用工具检查 Pod 状态**，例如：
     * 调用 pods_list_in_namespace 工具，查询该命名空间下的 Pod
     * 或者调用 resources_list 工具，查询 Pod 资源
   - 检查 Pod 是否已经启动（Status 是否为 Running）
   - 统计有多少个 Pod 处于 Running 状态

2. **提供完整的操作结果**：
   - 告诉用户操作已成功执行
   - 报告 Pod 的当前状态（如："已成功将 zk StatefulSet 的副本数设置为 3。当前有 2 个 Pod 正在启动中，1 个 Pod 已运行。"）
   - 如果 Pod 还未完全启动，说明当前状态和预期状态

**回复格式**：
- **第一步**：先调用工具检查 Pod 状态（使用 JSON 格式）：
  示例：{"action": "call_tool", "tool": "pods_list_in_namespace", "arguments": {"namespace": "paas-public", "labelSelector": "app=zk"}, "thought": "操作已成功执行，现在检查 Pod 是否已启动", "reply": "已成功执行操作。正在检查 Pod 状态..."}
  或者：{"action": "call_tool", "tool": "resources_list", "arguments": {"apiVersion": "v1", "kind": "Pod", "namespace": "paas-public"}, "thought": "操作已成功执行，现在检查 Pod 是否已启动", "reply": "已成功执行操作。正在检查 Pod 状态..."}
- **第二步**：基于 Pod 检查结果，提供完整的操作报告

**重要**：
- **必须调用工具检查 Pod 状态**，不能只回复"操作成功"
- 你的 "reply" 字段应该简洁但包含关键信息（操作结果 + Pod 状态）
- 不要包含完整的资源定义、配置详情等
- 工具调用的详细信息会显示在"执行步骤"中，用户可以看到

请基于上述工具的执行结果，**立即调用工具检查 Pod 状态**，然后提供完整的操作报告。`, toolName, toolResultText, userMessage)
					} else {
						// 不需要检查 Pod 的操作，保持简洁回复
						userPrompt = fmt.Sprintf(`【重要】工具 %s 已成功执行（来自 Kubernetes API 的实时数据）：

工具返回：%s

**用户的问题**："%s"

【关键要求 - 简洁回复】
工具已经成功执行了创建/更新操作。你的回复必须非常简洁：

1. **不要**输出完整的资源定义（YAML/JSON）
2. **不要**列出所有配置细节
3. **只**告诉用户操作是否成功，以及关键信息

**回复格式示例**：
- 如果创建成功："已成功创建/更新资源。资源名称：xxx，命名空间：xxx"
- 如果更新成功："已成功更新资源。资源名称：xxx"
- 如果操作完成："操作已完成。资源已启动/配置已更新"

**重要**：
- 你的 "reply" 字段应该只包含简洁的成功提示（1-2句话）
- 不要包含完整的资源定义、配置详情等
- 工具调用的详细信息会显示在"执行步骤"中，用户可以看到

请基于上述工具的执行结果，提供简洁的成功提示。必须使用 JSON 格式回复，格式：{"action": "respond", "reply": "简洁的成功提示（1-2句话）"}。`, toolName, toolResultText, userMessage)
					}
				} else {
					// 对于查询操作，提供详细、智能的回答
					userPrompt = fmt.Sprintf(`【重要】以下是工具 %s 的真实执行结果（来自 Kubernetes API 的实时数据）：

%s

用户的问题："%s"

### 关键任务 - 智能分析和详细回答
你必须仔细分析工具返回的数据，理解用户的问题，然后提供详细、专业的回答。

### 分析策略
1. 理解用户意图：
   - 如果用户问"有多少个"（如"有多少个 namespace"、"有多少个 Pod"）：
     * 先统计数量并明确回答
     * 然后提供完整的列表（像 dify 那样），让用户可以看到所有项目
     * 使用清晰的格式：编号列表或表格
   - 如果用户问"列出"或"显示"：
     * 提供完整的列表，包含所有详细信息
   - 如果用户指定了命名空间或其他条件：
     * 先过滤出符合条件的数据
     * 然后统计数量并提供列表

2. 回答格式要求（参考 dify 的风格）：
   - 开头：直接回答用户的问题（如"你的 Kubernetes 集群总共有 25 个 Namespace。"）
   - 列表标题：如果数据较多，使用"以下是完整的列表："或类似的标题
   - 列表格式：使用编号列表（1. 2. 3. ...）或表格格式
   - 详细信息：对于每个项目，提供关键信息（如名称、状态等）
   - 代码和命令：必须使用代码块格式，例如：
     \`\`\`bash
     kubectl get pods -n default
     \`\`\`
   - 错误信息：多行错误信息必须使用代码块包裹：
     \`\`\`text
     错误信息内容
     \`\`\`

3. 示例 - Namespace 查询：
   用户问："我的k8s集群有多少个namespace"
   工具返回：包含 25 个 namespace 的列表
   你的回答应该是：
   "你的 Kubernetes 集群总共有 25 个 Namespace。
   
   以下是完整的列表：
   1. cmdevops-middleware
   2. default
   3. dify
   ...（列出所有 25 个）"

4. 示例 - Pod 数量查询：
   用户问："dify 命名空间有多少个 Pod？"
   工具返回：包含 dify、default、kafka 等多个命名空间的 Pod 列表
   你的回答应该是：
   "在 dify 命名空间中共有 12 个 Pod。
   
   以下是 dify 命名空间中的 Pod 列表：
   1. pod-name-1 (Status: Running)
   2. pod-name-2 (Status: Running)
   ...（列出所有 12 个 Pod）"

### 严格要求
1. 必须只使用工具返回的真实数据
2. 对于数量查询，必须提供完整列表（不要只说数量）
3. 使用清晰的格式（编号列表、表格等）
4. 提供详细、专业、有条理的回答
5. 如果工具返回的数据为空，明确告诉用户"没有找到"
6. 代码、命令、错误信息必须使用代码块格式
7. 减少粗体标记的使用，只在真正需要强调时使用

### 关于回复格式
- 工具调用的详细信息会显示在"执行步骤"中
- 你的 "reply" 字段应该包含：
  * 直接回答用户的问题（数量或状态）
  * 完整的列表（如果数据不多，或用户明确要求列表）
  * 清晰的格式和结构
  * 代码和命令使用代码块格式
- "reply" 应该像 dify 那样详细和专业

请基于上述工具的真实执行结果，提供详细、专业的回答，包括数量统计和完整列表。必须使用 JSON 格式回复，格式：
\`\`\`json
{"action": "respond", "reply": "你的详细回答（包含数量统计和完整列表，代码和命令使用代码块格式）"}
\`\`\``, toolName, toolResultText, userMessage)
				}
			}
			
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: userPrompt,
			})

			// 继续下一轮
			continue
		} else {
			// LLM 返回最终答案
			// 检查：如果这是第一次循环且没有调用工具，但问题是关于K8s资源的，要求先调用工具
			if step == 0 && action == "respond" && len(allowedTools) > 0 {
				// 检查用户问题是否包含K8s相关关键词
				userQuestionLower := strings.ToLower(userMessage)
				k8sKeywords := []string{"pod", "pods", "deployment", "namespace", "service", "node", "cluster", "k8s", "kubernetes", "命名空间", "容器"}
				needsTool := false
				for _, keyword := range k8sKeywords {
					if strings.Contains(userQuestionLower, keyword) {
						needsTool = true
						break
					}
				}
				
				if needsTool {
					log.Printf("[Orchestrator] WARNING: LLM returned answer without calling tool for K8s-related question. Forcing tool call.")
					// 要求LLM先调用工具
					messages = append(messages, llm.Message{
						Role:    "assistant",
						Content: llmResponse,
					})
					messages = append(messages, llm.Message{
						Role:    "user",
						Content: "你的回答似乎没有调用工具。对于 Kubernetes 资源查询，你必须先调用相应的工具获取实时数据，不能基于训练数据直接回答。请重新分析问题并调用合适的工具。",
					})
					continue
				}
			}
			
			finalReply = reply
			if finalReply == "" {
				// 如果 reply 为空，尝试使用 thought，或者生成友好回复
				if thought != "" {
					finalReply = thought
				} else {
					// 如果响应是 JSON 格式但没有 reply，尝试提取有用信息
					if strings.HasPrefix(strings.TrimSpace(llmResponse), "{") {
						// 是 JSON 格式，生成友好提示
						if toolName != "" {
							finalReply = fmt.Sprintf("正在使用工具 %s 处理您的请求，请稍候...", toolName)
						} else {
							finalReply = "正在处理您的请求，请稍候..."
						}
					} else {
						// 不是 JSON，直接使用原始响应
						finalReply = llmResponse
					}
				}
			}
			
			// 清理最终回答，移除工具调用的技术细节
			finalReply = cleanFinalReply(finalReply, allowedTools)

			// 更新会话
			session.Messages = append(session.Messages, llm.Message{
				Role:    "assistant",
				Content: finalReply,
			})
			break
		}
	}

	if finalReply == "" {
		finalReply = "抱歉，处理时间过长，请简化您的问题。"
	}

	// 保存会话到 MongoDB
	sessionDoc := &store.SessionDoc{
		ID:        session.ID,
		AgentID:   session.AgentID,
		Messages:  session.Messages,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
	if err := o.sessionStore.SaveSession(ctx, sessionDoc); err != nil {
		log.Printf("Warning: Failed to save session to MongoDB: %v", err)
	}

	return &types.ChatResponse{
		SessionID: session.ID,
		AgentID:   agentID,
		Reply:     finalReply,
		Steps:     steps,
	}, nil
}

// getToolNames 获取工具名称列表（用于日志）
func getToolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

// cleanFinalReply 清理最终回答，移除工具调用的技术细节
// 工具调用的详细信息会显示在 steps 中，最终回答应该只包含自然语言
func cleanFinalReply(reply string, tools []mcp.Tool) string {
	if reply == "" {
		return reply
	}
	
	cleaned := reply
	
	// 移除 JSON 格式的工具调用信息（优先处理）
	// 查找并移除类似 {"action": "call_tool", "tool": "...", ...} 的 JSON 块
	for {
		jsonStart := strings.Index(cleaned, `{"action"`)
		if jsonStart < 0 {
			break
		}
		// 找到匹配的右括号
		braceCount := 0
		jsonEnd := -1
		for i := jsonStart; i < len(cleaned); i++ {
			if cleaned[i] == '{' {
				braceCount++
			} else if cleaned[i] == '}' {
				braceCount--
				if braceCount == 0 {
					jsonEnd = i
					break
				}
			}
		}
		if jsonEnd > jsonStart {
			cleaned = cleaned[:jsonStart] + cleaned[jsonEnd+1:]
		} else {
			break
		}
	}
	
	// 移除工具名称（包括下划线格式和空格格式）
	for _, tool := range tools {
		// 移除工具名称（如 "pods_list_in_namespace"）
		cleaned = strings.ReplaceAll(cleaned, tool.Name, "")
		cleaned = strings.ReplaceAll(cleaned, strings.ToLower(tool.Name), "")
		// 移除工具名称（空格格式，如 "pods list in namespace"）
		toolNameSpaced := strings.ReplaceAll(tool.Name, "_", " ")
		cleaned = strings.ReplaceAll(cleaned, toolNameSpaced, "")
		cleaned = strings.ReplaceAll(cleaned, strings.ToLower(toolNameSpaced), "")
	}
	
	// 移除常见的技术性描述短语
	technicalPhrases := []string{
		"调用工具",
		"使用工具",
		"工具调用",
		"执行工具",
		"工具执行结果",
		"工具返回结果",
		"工具结果",
		"执行结果",
		"返回结果",
		"工具名称",
		"工具参数",
		"参数：",
		"参数:",
		"arguments",
		"Arguments",
		"action:",
		"action：",
		"tool:",
		"tool：",
		"namespace=",
		"name=",
	}
	
	for _, phrase := range technicalPhrases {
		cleaned = strings.ReplaceAll(cleaned, phrase, "")
	}
	
	// 清理多余的空行和空格
	lines := strings.Split(cleaned, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "}") {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}
	cleaned = strings.Join(cleanedLines, "\n")
	
	// 如果清理后为空，返回原始回复
	if strings.TrimSpace(cleaned) == "" {
		return reply
	}
	
	return strings.TrimSpace(cleaned)
}

// parseLLMResponse 解析 LLM 响应，提取工具调用信息
func (o *Orchestrator) parseLLMResponse(response string, tools []mcp.Tool) (action, toolName string, toolArgs map[string]interface{}, thought, reply string) {
	// 尝试解析 JSON 格式的响应
	// 格式: {"action": "call_tool", "tool": "list_pods", "arguments": {...}, "thought": "...", "reply": "..."}

	// 先尝试查找 JSON 块
	jsonStart := strings.Index(response, "{")
	if jsonStart >= 0 {
		jsonEnd := strings.LastIndex(response, "}")
		if jsonEnd > jsonStart {
			jsonStr := response[jsonStart : jsonEnd+1]
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
				if act, ok := parsed["action"].(string); ok {
					action = act
				}
				if tool, ok := parsed["tool"].(string); ok {
					toolName = tool
				}
				if args, ok := parsed["arguments"].(map[string]interface{}); ok {
					toolArgs = args
				}
				if th, ok := parsed["thought"].(string); ok {
					thought = th
				}
				if rp, ok := parsed["reply"].(string); ok {
					reply = rp
				}
				return
			}
		}
	}

	// 如果没有 JSON，尝试从文本中提取工具调用意图
	// 简单的启发式规则
	responseLower := strings.ToLower(response)

	// 检查是否包含工具名称
	for _, tool := range tools {
		if strings.Contains(responseLower, tool.Name) || strings.Contains(responseLower, strings.ReplaceAll(tool.Name, "_", " ")) {
			action = "call_tool"
			toolName = tool.Name

			// 尝试提取命名空间
			// 如果用户明确指定了命名空间，则使用指定的命名空间
			// 否则不指定命名空间，让工具查询所有命名空间
			if strings.Contains(responseLower, "namespace") || strings.Contains(responseLower, "命名空间") {
				// 简单的命名空间提取
				if strings.Contains(responseLower, "default") {
					toolArgs = map[string]interface{}{"namespace": "default"}
				} else if strings.Contains(responseLower, "dify") {
					toolArgs = map[string]interface{}{"namespace": "dify"}
				} else {
					// 如果提到了命名空间但没有明确指定，尝试提取
					// 这里可以留空，让工具查询所有命名空间
					toolArgs = map[string]interface{}{}
				}
			} else {
				// 如果没有提到命名空间，不指定命名空间参数，查询所有命名空间
				toolArgs = map[string]interface{}{}
			}
			return
		}
	}

	// 默认返回响应
	action = "respond"
	reply = response
	return
}

// buildSystemPrompt 构建系统提示词
func (o *Orchestrator) buildSystemPrompt(agent *types.AgentConfig, tools []mcp.Tool) string {
	// 构建工具列表描述
	var toolDescs []string
	for _, tool := range tools {
		desc := fmt.Sprintf("- %s: %s", tool.Name, tool.Description)
		if tool.InputSchema.Properties != nil && len(tool.InputSchema.Properties) > 0 {
			var params []string
			for name, prop := range tool.InputSchema.Properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					paramType := "string"
					if t, ok := propMap["type"].(string); ok {
						paramType = t
					}
					paramDesc := ""
					if d, ok := propMap["description"].(string); ok {
						paramDesc = d
					}
					params = append(params, fmt.Sprintf("%s (%s): %s", name, paramType, paramDesc))
				}
			}
			if len(params) > 0 {
				desc += "\n  参数: " + strings.Join(params, ", ")
			}
		}
		toolDescs = append(toolDescs, desc)
	}
	toolsList := strings.Join(toolDescs, "\n")

	// 如果 Agent 配置了自定义提示词，使用自定义提示词（替换占位符）
	if agent != nil && agent.SystemPrompt != "" {
		customPrompt := agent.SystemPrompt
		
		// 替换 {agentName} 占位符
		agentName := "MCP 智能体"
		if agent.Name != "" {
			agentName = agent.Name
		}
		customPrompt = strings.ReplaceAll(customPrompt, "{agentName}", agentName)
		
		// 替换 {description} 占位符
		description := "可以帮助用户管理和查询 Kubernetes 集群。"
		if agent.Description != "" {
			description = agent.Description
		}
		customPrompt = strings.ReplaceAll(customPrompt, "{description}", description)
		
		// 替换 {tools} 占位符
		if strings.Contains(customPrompt, "{tools}") {
			customPrompt = strings.ReplaceAll(customPrompt, "{tools}", toolsList)
		} else {
			// 如果没有占位符，在末尾追加工具列表
			customPrompt += "\n\n可用工具：\n" + toolsList
		}
		
		log.Printf("[Orchestrator] Using custom system prompt for agent %s (length: %d)", agentName, len(customPrompt))
		return customPrompt
	}

	// 使用默认提示词模板
	agentName := "MCP 智能体"
	if agent != nil && agent.Name != "" {
		agentName = agent.Name
	}
	description := "可以帮助用户管理和查询 Kubernetes 集群。"
	if agent != nil && agent.Description != "" {
		description = agent.Description
	}

	prompt := fmt.Sprintf(`你是智能体 "%s"，%s

可用工具：
%s

【重要 - 回答格式规范】：
1. 代码和命令必须使用代码块格式：
   - 代码使用：\`\`\`go 或 \`\`\`bash 等
   - 命令使用：\`\`\`bash
   - 单行代码或变量名使用行内代码：\`变量名\`
   - 示例：
     \`\`\`bash
     kubectl get pods -n default
     \`\`\`
   
2. 减少粗体标记使用：
   - 只在真正需要强调的关键信息时使用粗体
   - 避免在列表项、普通描述中过度使用粗体
   - 使用标题层级（###、####）来组织内容，而不是大量粗体

3. 错误信息和日志使用代码块：
   - 多行错误信息必须使用代码块包裹
   - 示例：
     \`\`\`text
     felix is not ready: Get "http://localhost:9099/readiness": dial tcp [::1]:9099: connect: connection refused
     BIRD is not ready: Error querying BIRD: unable to connect to BIRDv4 socket
     \`\`\`

【重要 - resources_create_or_update 工具使用指南】：
1. 对于 scale 操作（修改副本数）：
   - 使用 resources_create_or_update 工具
   - 关键步骤：
     a. 先调用 resources_get 获取资源的完整定义
     b. 从获取的资源中提取完整的 JSON 结构
     c. 只修改 spec.replicas 字段（不要修改其他字段）
     d. 将修改后的完整资源定义作为 resource 参数传递给 resources_create_or_update
   - resource 参数格式要求：
     * 必须是有效的 JSON 字符串（不是 YAML）
     * 所有字符串值用双引号包裹
     * JSON 字符串中的双引号必须转义为 \\"
     * 换行符必须转义为 \\n
     * 确保 JSON 结构完整（所有括号匹配）
   
2. 示例 - Scale StatefulSet：
   - 用户："将 zk StatefulSet 的副本数设置为 3"
   - 第一步：
     \`\`\`json
     {"action": "call_tool", "tool": "resources_get", "arguments": {"apiVersion": "apps/v1", "kind": "StatefulSet", "name": "zk", "namespace": "paas-public"}}
     \`\`\`
   - 第二步：从 resources_get 返回的结果中提取完整的资源定义，只修改 spec.replicas 为 3，然后：
     \`\`\`json
     {"action": "call_tool", "tool": "resources_create_or_update", "arguments": {"resource": "{\\"apiVersion\\":\\"apps/v1\\",\\"kind\\":\\"StatefulSet\\",\\"metadata\\":{...完整metadata...},\\"spec\\":{\\"replicas\\":3,...完整spec...}}"}}
     \`\`\`

【重要 - 回答格式要求】：
当你完成操作后，必须提供详细、结构化的回答，包括：

### 操作总结
- 问题诊断：说明你发现了什么问题（如"StatefulSet 的副本数被设置为 0，导致没有 Pod 被创建"）
- 执行步骤：详细说明你采取了什么操作（如"由于直接更新遇到了配置冲突，我采取了以下步骤：删除了旧的、处于冲突状态的 StatefulSet；重新创建了一个新的 StatefulSet，其 replicas 字段已正确设置为 3"）
- 验证结果：说明操作是否成功（如"新的 StatefulSet 已成功创建"）

### 当前状态
- 详细说明资源的当前状态（如"StatefulSet kafka：已激活，期望副本数为 3"）
- 说明 Pod 的状态（如"Pod kafka-0：正在创建中，这可能需要一点时间，因为它需要拉取镜像并启动容器"）
- 说明后续 Pod 的创建计划（如"一旦 kafka-0 就绪，StatefulSet 控制器会自动开始创建 kafka-1 和 kafka-2"）

### 后续建议
- 告诉用户下一步该做什么（如"你可以稍等片刻，然后运行以下命令查看 Pod 状态："）
  \`\`\`bash
  kubectl -n kafka get pods
  \`\`\`

回答要求：
- 回答必须详细、专业、有条理
- 使用中文回答
- 提供清晰的结构（使用标题层级、列表等）
- 包含具体的资源名称、命名空间、状态等信息
- 给出实用的建议和后续步骤
- 代码和命令必须使用代码块格式

### 工具使用指南

2. 对于编辑/修改操作：
   - 优先使用 edit 相关工具（如 resources_edit、statefulset_edit 等）
   - 这些工具专门用于编辑资源，更简单、更可靠
   - 不要使用 resources_create_or_update 来编辑资源，除非没有 edit 工具可用

3. 对于创建新资源：
   - 使用 resources_create_or_update 或 create 相关工具

4. 对于查询操作：
   - 使用 list、get 相关工具（如 list_pods、resources_get、resources_list 等）

### 工作流程
1. 理解用户意图：区分查询（列出/有多少个）和操作（scale/重启/编辑）
2. 根据操作类型选择最合适的工具：
   - Scale 操作 → 优先使用 scale 工具
   - 编辑操作 → 优先使用 edit 工具
   - 创建操作 → 使用 create 或 resources_create_or_update
   - 查询操作 → 使用 list/get 工具
3. 对于 K8s 查询或操作，必须先调用工具获取实时数据
4. 如果工具返回空结果，尝试其他查询方式（如列出所有资源）
5. 分析工具结果：数量查询只统计，列表查询提供总结
6. 用自然语言回答用户问题

### 重要规则
- 优先使用专门的工具（scale、edit）而不是通用的 resources_create_or_update
- 只使用工具返回的真实数据，不使用训练数据
- K8s 查询必须先调用工具
- 操作请求先找到资源再执行
- 不要编造不存在的信息

### 响应格式（JSON）
- 调用工具：
  \`\`\`json
  {"action": "call_tool", "tool": "工具名", "arguments": {...}, "thought": "思考", "reply": "说明"}
  \`\`\`
- 直接回答：
  \`\`\`json
  {"action": "respond", "reply": "回答"}
  \`\`\`

### 示例 - Scale 操作
- 用户："将 zk StatefulSet 的副本数设置为 3"
- 正确做法：
  \`\`\`json
  {"action": "call_tool", "tool": "statefulset_scale", "arguments": {"name": "zk", "namespace": "paas-public", "replicas": 3}, "thought": "用户要 scale StatefulSet，使用专门的 scale 工具", "reply": "正在将 zk StatefulSet 的副本数设置为 3..."}
  \`\`\`
- 或者：
  \`\`\`json
  {"action": "call_tool", "tool": "resources_scale", "arguments": {"apiVersion": "apps/v1", "kind": "StatefulSet", "name": "zk", "namespace": "paas-public", "replicas": 3}}
  \`\`\`
- 错误做法：使用 resources_create_or_update 来修改副本数

resources_create_or_update 格式要求（仅在必要时使用）：
- resource 必须是有效的 JSON 字符串
- 使用 \\n 转义换行，\\" 转义引号
- updateStrategy 在 spec.updateStrategy，格式：{"type": "RollingUpdate"}

重要：无论何时，都必须包含 "reply" 字段，用自然语言向用户说明当前的操作或回答。请用中文回答用户的问题。

### 回答质量要求
当你完成操作后，必须提供详细、结构化的回答，像 dify 那样专业和完整：

1. 操作总结（必须包含）：
   - 问题诊断：说明你发现了什么问题（如"kafka 命名空间中的 StatefulSet kafka 的副本数(replicas)被设置为 0，导致没有 Pod 被创建"）
   - 执行步骤：详细说明你采取了什么操作（如"由于直接更新遇到了配置冲突，我采取了以下步骤：删除了旧的、处于冲突状态的 StatefulSet；重新创建了一个新的 StatefulSet，其 replicas 字段已正确设置为 3"）
   - 验证结果：说明操作是否成功（如"新的 StatefulSet 已成功创建"）

2. 当前状态（必须包含）：
   - 详细说明资源的当前状态（如"StatefulSet kafka：已激活，期望副本数为 3"）
   - 说明 Pod 的状态（如"Pod kafka-0：正在创建中，这可能需要一点时间，因为它需要拉取 kafka:v3.5 镜像并启动容器"）
   - 说明后续 Pod 的创建计划（如"一旦 kafka-0 就绪，StatefulSet 控制器会自动开始创建 kafka-1 和 kafka-2"）

3. 后续建议（必须包含）：
   - 告诉用户下一步该做什么（如"你可以稍等片刻，然后运行以下命令查看 Pod 状态："）
     \`\`\`bash
     kubectl -n kafka get pods
     \`\`\`

回答格式要求：
- 回答必须详细、专业、有条理
- 使用清晰的结构（标题层级、列表等）
- 包含具体的资源名称、命名空间、状态等信息
- 给出实用的建议和后续步骤
- 使用中文回答
- 代码和命令必须使用代码块格式`, agentName, description, toolsList)

	return prompt
}

// getOrCreateSession 获取或创建会话
func (o *Orchestrator) getOrCreateSession(ctx context.Context, sessionID string, agent *types.AgentConfig) (*Session, error) {
	agentID := ""
	if agent != nil {
		agentID = agent.ID
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%s_%d", agentID, time.Now().UnixNano())
	}

	// 尝试从 MongoDB 加载
	sessionDoc, err := o.sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session *Session
	if sessionDoc != nil {
		// 检查 Agent ID 是否匹配
		if sessionDoc.AgentID != "" && sessionDoc.AgentID != agentID {
			// Agent 不匹配，创建新会话
			sessionID = fmt.Sprintf("session_%s_%d", agentID, time.Now().UnixNano())
			session = &Session{
				ID:        sessionID,
				AgentID:   agentID,
				Messages:  []llm.Message{},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		} else {
			// 使用现有会话
			session = &Session{
				ID:        sessionDoc.ID,
				AgentID:   sessionDoc.AgentID,
				Messages:  sessionDoc.Messages,
				CreatedAt: sessionDoc.CreatedAt,
				UpdatedAt: sessionDoc.UpdatedAt,
			}
			if session.AgentID == "" {
				session.AgentID = agentID
			}
		}
	} else {
		// 创建新会话
		session = &Session{
			ID:        sessionID,
			AgentID:   agentID,
			Messages:  []llm.Message{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	return session, nil
}

// GetSession 获取会话
func (o *Orchestrator) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sessionDoc, err := o.sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if sessionDoc == nil {
		return nil, nil
	}
	return &Session{
		ID:        sessionDoc.ID,
		AgentID:   sessionDoc.AgentID,
		Messages:  sessionDoc.Messages,
		CreatedAt: sessionDoc.CreatedAt,
		UpdatedAt: sessionDoc.UpdatedAt,
	}, nil
}

// GetSessions 获取会话列表（可按 Agent 过滤，支持分页）
// 返回 SessionMeta 列表以便前端获取标题和消息数量
func (o *Orchestrator) GetSessions(ctx context.Context, agentID string, limit, skip int) ([]*store.SessionMeta, int64, error) {
	// 获取会话元数据（不包含消息）
	metas, err := o.sessionStore.GetSessions(ctx, agentID, limit, skip)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get sessions: %w", err)
	}

	// 获取总数
	total, err := o.sessionStore.CountSessions(ctx, agentID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sessions: %w", err)
	}

	return metas, total, nil
}

// detectFunctionCallingSupport 检测模型是否支持 Function Calling
func (o *Orchestrator) detectFunctionCallingSupport(llmConfig *types.LLMConfig) bool {
	if llmConfig == nil {
		return false
	}
	return llm.SupportsFunctionCalling(llmConfig.Provider, llmConfig.Model)
}

// getOrDetermineStrategy 获取或确定 Agent 的策略
// 如果 Agent 已有 Strategy，直接返回
// 如果没有，检测模型能力并返回（但不保存，由调用方保存）
func (o *Orchestrator) getOrDetermineStrategy(agent *types.AgentConfig, llmConfig *types.LLMConfig) string {
	// 如果已有 Strategy，直接使用
	if agent.Strategy != "" {
		log.Printf("[Orchestrator] Agent %s using stored strategy: %s", agent.Name, agent.Strategy)
		return agent.Strategy
	}

	// 检测模型能力
	supportsFC := o.detectFunctionCallingSupport(llmConfig)
	if supportsFC {
		log.Printf("[Orchestrator] Agent %s: Model %s supports Function Calling, will use function_call strategy", 
			agent.Name, llmConfig.Model)
		return "function_call"
	} else {
		log.Printf("[Orchestrator] Agent %s: Model %s does not support Function Calling, will use prompt_based strategy", 
			agent.Name, llmConfig.Model)
		return "prompt_based"
	}
}

// convertToolsToLLMFormat 转换 MCP Tool 为 LLM Tool 格式（用于 Function Calling）
func (o *Orchestrator) convertToolsToLLMFormat(tools []mcp.Tool) []llm.Tool {
	llmTools := make([]llm.Tool, 0, len(tools))
	for _, tool := range tools {
		// 转换 InputSchema 为 JSON Schema 格式
		parameters := make(map[string]interface{})
		parameters["type"] = "object"
		
		properties := make(map[string]interface{})
		required := make([]string, 0)
		
		if tool.InputSchema.Properties != nil {
			for name, prop := range tool.InputSchema.Properties {
				if propMap, ok := prop.(map[string]interface{}); ok {
					propSchema := make(map[string]interface{})
					
					// 复制属性定义
					if propType, ok := propMap["type"].(string); ok {
						propSchema["type"] = propType
					}
					if desc, ok := propMap["description"].(string); ok {
						propSchema["description"] = desc
					}
					if def, ok := propMap["default"]; ok {
						propSchema["default"] = def
					}
					
					properties[name] = propSchema
				}
			}
		}
		
		parameters["properties"] = properties
		
		// 处理 required 字段
		if tool.InputSchema.Required != nil {
			required = tool.InputSchema.Required
		}
		if len(required) > 0 {
			parameters["required"] = required
		}
		
		llmTool := llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parameters,
			},
		}
		
		llmTools = append(llmTools, llmTool)
	}
	
	return llmTools
}

// chatWithFunctionCalling Function Calling 模式处理
func (o *Orchestrator) chatWithFunctionCalling(ctx context.Context, sessionID string, session *Session, userMessage string, agent *types.AgentConfig, llmConfig *types.LLMConfig, allowedTools []mcp.Tool) (*types.ChatResponse, error) {
	agentID := session.AgentID

	var steps []types.ChatStep
	var finalReply string

	allowedToolNames := make(map[string]struct{})
	for _, tool := range allowedTools {
		allowedToolNames[strings.ToLower(tool.Name)] = struct{}{}
	}

	// 转换工具为 LLM 格式
	llmTools := o.convertToolsToLLMFormat(allowedTools)
	log.Printf("[Orchestrator] Converted %d tools to LLM format for Function Calling", len(llmTools))

	// 准备消息列表（不包含系统提示词，工具信息通过 tools 参数传递）
	messages := []llm.Message{}

	// 限制历史消息数量，只保留最近3轮对话
	const maxHistoryRounds = 3
	const maxHistoryMessages = maxHistoryRounds * 2

	var historicalMessages []llm.Message
	if len(session.Messages) > maxHistoryMessages {
		historicalMessages = session.Messages[len(session.Messages)-maxHistoryMessages:]
		log.Printf("[Orchestrator] Limiting history: %d total messages, keeping last %d messages",
			len(session.Messages), len(historicalMessages))
	} else {
		historicalMessages = session.Messages
	}

	messages = append(messages, historicalMessages...)

	// 记录总开始时间
	totalStartTime := time.Now()

	// 循环处理，直到 LLM 返回最终答案或达到最大步数
	for step := 0; step < o.maxSteps; step++ {
		elapsed := time.Since(totalStartTime)
		log.Printf("[Orchestrator] Step %d: Elapsed time: %v", step+1, elapsed)

		if ctx.Err() != nil {
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("[Orchestrator] ERROR: Request timeout at step %d (elapsed: %v)", step+1, elapsed)
				return nil, fmt.Errorf("请求超时：处理步骤 %d 时超过时间限制（已耗时 %v）", step+1, elapsed)
			}
			return nil, fmt.Errorf("请求被取消: %w", ctx.Err())
		}

		log.Printf("[Orchestrator] Step %d: Calling LLM with Function Calling (elapsed: %v)", step+1, elapsed)
		log.Printf("[Orchestrator] ========== LLM Call #%d (Function Calling) ==========", step+1)

		llmStartTime := time.Now()
		llmResponse, err := o.llmClient.ChatWithTools(messages, llmTools)
		llmDuration := time.Since(llmStartTime)

		if err != nil {
			log.Printf("[Orchestrator] ERROR: Function Calling failed after %v: %v", llmDuration, err)
			// 如果 Function Calling 失败，回退到 Prompt-based
			log.Printf("[Orchestrator] Falling back to Prompt-based mode")
			return o.chatWithPromptBased(ctx, sessionID, session, userMessage, agent, allowedTools)
		}

		log.Printf("[Orchestrator] ✓ LLM response received in %v (ContentLength=%d, ToolCalls=%d)",
			llmDuration, len(llmResponse.Content), len(llmResponse.ToolCalls))

		// 记录 LLM 响应
		steps = append(steps, types.ChatStep{
			Type: "llm",
			Text: llmResponse.Content,
		})

		// 检查是否有工具调用
		if len(llmResponse.ToolCalls) > 0 {
			// 有工具调用，执行工具
			log.Printf("[Orchestrator] LLM requested %d tool calls", len(llmResponse.ToolCalls))

			// 添加 assistant 消息（包含 tool_calls）
			messages = append(messages, llm.Message{
				Role:      "assistant",
				Content:   llmResponse.Content,
				ToolCalls: llmResponse.ToolCalls,
			})

			// 执行所有工具调用
			toolResults := make([]llm.Message, 0, len(llmResponse.ToolCalls))
			for _, toolCall := range llmResponse.ToolCalls {
				toolName := toolCall.Function.Name
				toolArgsStr := toolCall.Function.Arguments
				toolCallID := toolCall.ID

				// 解析工具参数
				var toolArgs map[string]interface{}
				if err := json.Unmarshal([]byte(toolArgsStr), &toolArgs); err != nil {
					log.Printf("[Orchestrator] ERROR: Failed to parse tool arguments: %v", err)
					toolResults = append(toolResults, llm.Message{
						Role:       "tool",
						Content:    fmt.Sprintf(`{"error": "Failed to parse tool arguments: %v"}`, err),
						ToolCallID: toolCallID,
					})
					continue
				}

				// 检查工具是否可用
				if _, ok := allowedToolNames[strings.ToLower(toolName)]; !ok {
					log.Printf("[Orchestrator] Tool %s is not available for agent %s", toolName, agentID)
					toolResults = append(toolResults, llm.Message{
						Role:       "tool",
						Content:    fmt.Sprintf(`{"error": "Tool %s is not available"}`, toolName),
						ToolCallID: toolCallID,
					})
					continue
				}

				log.Printf("[Orchestrator] Calling tool: %s (ID: %s) with args: %+v", toolName, toolCallID, toolArgs)

				steps = append(steps, types.ChatStep{
					Type:      "tool",
					Tool:      toolName,
					Arguments: toolArgs,
				})

				// 调用工具
				toolStartTime := time.Now()
				toolResult, err := o.toolManager.CallTool(toolName, toolArgs)
				toolDuration := time.Since(toolStartTime)

				if err != nil {
					log.Printf("[Orchestrator] ERROR: Tool %s failed after %v: %v", toolName, toolDuration, err)
					toolResultText := fmt.Sprintf(`{"error": "Tool %s failed: %v"}`, toolName, err)
					toolResults = append(toolResults, llm.Message{
						Role:       "tool",
						Content:    toolResultText,
						ToolCallID: toolCallID,
					})
					steps[len(steps)-1].Result = map[string]interface{}{
						"error": err.Error(),
					}
					continue
				}

				// 工具调用成功
				var toolResultData interface{}
				var toolResultText string
				if toolResult != nil && len(toolResult.Content) > 0 {
					toolResultText = toolResult.Content[0].Text
					if err := json.Unmarshal([]byte(toolResultText), &toolResultData); err != nil {
						toolResultData = toolResultText
					}
				}

				log.Printf("[Orchestrator] Tool %s completed, result length: %d", toolName, len(toolResultText))
				steps[len(steps)-1].Result = toolResultData

				// 添加工具结果消息（Function Calling 中，tool 消息需要包含 tool_call_id）
				toolResults = append(toolResults, llm.Message{
					Role:       "tool",
					Content:    toolResultText,
					ToolCallID: toolCallID, // 关联对应的 tool call
				})
			}

			// 将所有工具结果添加到消息列表
			messages = append(messages, toolResults...)

			// 继续下一轮
			continue
		} else {
			// 没有工具调用，LLM 返回最终答案
			finalReply = llmResponse.Content
			if finalReply == "" {
				finalReply = "抱歉，我无法处理您的请求。"
			}

			// 清理最终回答
			finalReply = cleanFinalReply(finalReply, allowedTools)

			// 更新会话
			session.Messages = append(session.Messages, llm.Message{
				Role:    "assistant",
				Content: finalReply,
			})
			break
		}
	}

	if finalReply == "" {
		finalReply = "抱歉，处理时间过长，请简化您的问题。"
	}

	// 保存会话
	sessionDoc := &store.SessionDoc{
		ID:        session.ID,
		AgentID:   session.AgentID,
		Messages:  session.Messages,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
	if err := o.sessionStore.SaveSession(ctx, sessionDoc); err != nil {
		log.Printf("Warning: Failed to save session: %v", err)
	}

	return &types.ChatResponse{
		SessionID: session.ID,
		AgentID:   agentID,
		Reply:     finalReply,
		Steps:     steps,
	}, nil
}
