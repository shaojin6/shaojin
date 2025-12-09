package llm

import (
	"strings"

	"github.com/your-org/k8s-mcp-agent/internal/web/types"
)

// 模型能力映射表（硬编码）
// Key 格式: "provider:model"
var modelCapabilities = map[string][]types.ModelFeature{
	// DashScope/Qwen
	"dashscope:qwen-max":     {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"dashscope:qwen-plus":    {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"dashscope:qwen-turbo":   {types.ModelFeatureToolCall},
	"dashscope:qwen-7b-chat": {types.ModelFeatureToolCall},
	"qwen:qwen-max":          {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"qwen:qwen-plus":         {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"qwen:qwen-turbo":        {types.ModelFeatureToolCall},
	"tongyi:qwen-max":        {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"tongyi:qwen-plus":       {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"bailian:qwen-max":       {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"bailian:qwen-plus":      {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"sfm:qwen-max":           {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"modelstudio:qwen-max":   {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},

	// OpenAI
	"openai:gpt-4":              {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"openai:gpt-4-turbo":        {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"openai:gpt-4-turbo-preview": {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"openai:gpt-3.5-turbo":      {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"openai:gpt-3.5-turbo-16k":  {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},
	"openai:gpt-3.5-turbo-0125": {types.ModelFeatureToolCall, types.ModelFeatureMultiToolCall},

	// Ollama（需要确认，暂时不添加）
	// "ollama:llama2": {...},
}

// DetectModelCapabilities 检测模型能力
func DetectModelCapabilities(provider, model string) []types.ModelFeature {
	// 尝试精确匹配
	key := provider + ":" + model
	if features, ok := modelCapabilities[key]; ok {
		return features
	}

	// 尝试小写匹配（某些配置可能大小写不一致）
	keyLower := strings.ToLower(provider) + ":" + strings.ToLower(model)
	if features, ok := modelCapabilities[keyLower]; ok {
		return features
	}

	return []types.ModelFeature{}
}

// SupportsFunctionCalling 检查是否支持 Function Calling
func SupportsFunctionCalling(provider, model string) bool {
	features := DetectModelCapabilities(provider, model)
	for _, f := range features {
		if f == types.ModelFeatureToolCall || f == types.ModelFeatureMultiToolCall {
			return true
		}
	}
	return false
}

