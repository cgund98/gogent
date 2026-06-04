package openai

import (
	"encoding/json"
	"errors"
	"fmt"

	openaisdk "github.com/openai/openai-go"

	"github.com/cgund98/gogent"
)

func NewChat(client *openaisdk.Client, toolRegistry *gogent.ToolRegistry) *ModelBuilder {
	return &ModelBuilder{
		client:       client,
		toolRegistry: toolRegistry,
		settings: modelSettings{
			model: defaultModel,
		},
	}
}

type ModelBuilder struct {
	client       *openaisdk.Client
	toolRegistry *gogent.ToolRegistry
	tools        []gogent.Tool
	settings     modelSettings
}

func (b *ModelBuilder) WithSystemPrompt(systemPrompt string) *ModelBuilder {
	b.settings.systemPrompt = systemPrompt
	return b
}

func (b *ModelBuilder) WithModel(model string) *ModelBuilder {
	b.settings.model = model
	return b
}

func (b *ModelBuilder) WithTemperature(temperature float64) *ModelBuilder {
	b.settings.temperature = &temperature
	return b
}

func (b *ModelBuilder) WithMaxTokens(maxTokens int) *ModelBuilder {
	b.settings.maxTokens = &maxTokens
	return b
}

func (b *ModelBuilder) WithTopP(topP float64) *ModelBuilder {
	b.settings.topP = &topP
	return b
}

func (b *ModelBuilder) WithFrequencyPenalty(frequencyPenalty float64) *ModelBuilder {
	b.settings.frequencyPenalty = &frequencyPenalty
	return b
}

func (b *ModelBuilder) WithPresencePenalty(presencePenalty float64) *ModelBuilder {
	b.settings.presencePenalty = &presencePenalty
	return b
}

func (b *ModelBuilder) WithStop(stopSequences ...string) *ModelBuilder {
	b.settings.stopSequences = append([]string(nil), stopSequences...)
	return b
}

func (b *ModelBuilder) WithTools(tools ...gogent.Tool) *ModelBuilder {
	b.tools = append([]gogent.Tool(nil), tools...)
	return b
}

func (b *ModelBuilder) WithToolChoice(toolChoice string) *ModelBuilder {
	b.settings.toolChoice = &toolChoice
	return b
}

func (b *ModelBuilder) WithSeed(seed int64) *ModelBuilder {
	b.settings.seed = &seed
	return b
}

func (b *ModelBuilder) WithUser(user string) *ModelBuilder {
	b.settings.user = user
	return b
}

func (b *ModelBuilder) WithParallelToolCalls(enabled bool) *ModelBuilder {
	b.settings.parallelToolCalls = &enabled
	return b
}

func (b *ModelBuilder) WithResponseFormat(responseFormat json.RawMessage) *ModelBuilder {
	b.settings.responseFormat = append(json.RawMessage(nil), responseFormat...)
	return b
}

func (b *ModelBuilder) WithJSONResponse() *ModelBuilder {
	return b.WithResponseFormat(json.RawMessage(`{"type":"json_object"}`))
}

func (b *ModelBuilder) Build() (*Model, error) {
	if b.client == nil {
		return nil, errors.New("openai: client is required")
	}
	if b.settings.model == "" {
		return nil, errors.New("openai: model is required")
	}

	if err := validateOptionalFloat("temperature", b.settings.temperature, 0, 2); err != nil {
		return nil, err
	}
	if err := validateOptionalFloat("top_p", b.settings.topP, 0, 1); err != nil {
		return nil, err
	}
	if err := validateOptionalFloat("frequency_penalty", b.settings.frequencyPenalty, -2, 2); err != nil {
		return nil, err
	}
	if err := validateOptionalFloat("presence_penalty", b.settings.presencePenalty, -2, 2); err != nil {
		return nil, err
	}
	if b.settings.maxTokens != nil && *b.settings.maxTokens <= 0 {
		return nil, errors.New("openai: max tokens must be greater than zero")
	}

	settings := b.settings
	tools := append([]gogent.Tool(nil), b.tools...)
	if len(tools) == 0 && b.toolRegistry != nil {
		tools = b.toolRegistry.Tools()
	}
	if len(tools) > 0 && settings.toolChoice == nil {
		toolChoice := "auto"
		settings.toolChoice = &toolChoice
	}

	return &Model{
		client:       b.client,
		settings:     settings,
		tools:        tools,
		toolRegistry: b.toolRegistry,
	}, nil
}

// validateOptionalFloat returns an error when a configured optional float is outside the allowed range.
func validateOptionalFloat(name string, value *float64, minVal, maxVal float64) error {
	if value == nil {
		return nil
	}
	if *value < minVal || *value > maxVal {
		return fmt.Errorf("openai: %s must be between %v and %v", name, minVal, maxVal)
	}
	return nil
}
