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

	return fromChatCompletionMessage(resp.Choices[0].Message)
}
