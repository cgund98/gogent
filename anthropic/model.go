package anthropic

import (
	"context"
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/cgund98/gogent"
)

type Model struct {
	client       *anthropicsdk.Client
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
	params, err := toMessageNewParams(m.settings, m.tools, history)
	if err != nil {
		return gogent.Message{}, err
	}

	resp, err := m.client.Messages.New(ctx, params)
	if err != nil {
		return gogent.Message{}, fmt.Errorf("anthropic messages: %w", err)
	}
	if resp == nil {
		return gogent.Message{}, errors.New("anthropic: empty response")
	}

	return fromAnthropicMessage(resp)
}
