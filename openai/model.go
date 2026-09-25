package openai

import (
	"context"
	"errors"
	"fmt"

	openaisdk "github.com/openai/openai-go"

	"github.com/cgund98/gogent"
)

type Model struct {
	client       *openaisdk.Client
	settings     modelSettings
	tools        []gogent.Tool
	toolRegistry *gogent.ToolRegistry
}

func (m *Model) SetSystemPrompt(prompt string) {
	m.settings.systemPrompt = prompt
}

func (m *Model) SystemPrompt() string {
	if m == nil {
		return ""
	}
	return m.settings.systemPrompt
}

func (m *Model) GenerateResponse(ctx context.Context, history []gogent.Message) (gogent.Message, error) {
	params, err := toChatCompletionNewParams(m.settings, m.tools, history)
	if err != nil {
		return gogent.Message{}, err
	}

	resp, err := m.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return gogent.Message{}, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return gogent.Message{}, errors.New("openai: empty choices in response")
	}

	message, err := fromChatCompletionMessage(resp.Choices[0].Message)
	if err != nil {
		return gogent.Message{}, err
	}
	message.Usage = usageFromCompletion(resp.Usage)
	return message, nil
}

func usageFromCompletion(usage openaisdk.CompletionUsage) *gogent.Usage {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.PromptTokensDetails.CachedTokens == 0 {
		return nil
	}
	return &gogent.Usage{
		Input:  int(usage.PromptTokens),
		Output: int(usage.CompletionTokens),
		Cached: int(usage.PromptTokensDetails.CachedTokens),
	}
}
