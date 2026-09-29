package anthropic

import (
	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/cgund98/gogent"
)

// toMessageNewParams builds the Anthropic Messages request for a model turn.
func toMessageNewParams(settings modelSettings, tools []gogent.Tool, history []gogent.Message) (anthropicsdk.MessageNewParams, error) {
	messages, err := toMessageParams(history)
	if err != nil {
		return anthropicsdk.MessageNewParams{}, err
	}

	maxTokens := defaultMaxTokens
	if settings.maxTokens != nil {
		maxTokens = *settings.maxTokens
	}

	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(settings.model),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
	}

	if settings.systemPrompt != "" {
		params.System = []anthropicsdk.TextBlockParam{
			{Text: settings.systemPrompt},
		}
	}
	if settings.temperature != nil {
		params.Temperature = anthropicsdk.Float(*settings.temperature)
	}
	if settings.topP != nil {
		params.TopP = anthropicsdk.Float(*settings.topP)
	}
	if settings.topK != nil {
		params.TopK = anthropicsdk.Int(int64(*settings.topK))
	}
	if len(settings.stopSequences) > 0 {
		params.StopSequences = settings.stopSequences
	}
	if settings.effort != nil {
		params.OutputConfig = anthropicsdk.OutputConfigParam{
			Effort: anthropicsdk.OutputConfigEffort(*settings.effort),
		}
	}
	if len(tools) > 0 {
		sdkTools, err := toToolUnionParams(tools)
		if err != nil {
			return anthropicsdk.MessageNewParams{}, err
		}
		params.Tools = sdkTools
	}

	return params, nil
}
