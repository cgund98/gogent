// Package anthropic is the Anthropic model provider. It speaks to the Anthropic
// Messages API and implements gogent.Model with its own builder, mirroring the
// shape of the openai package.
package anthropic

import (
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/cgund98/gogent"
)

// Option changes the requests an Anthropic chat model sends.
type Option func(*settings)

type settings struct {
	baseURL string
}

// WithBaseURL points the client at a different endpoint, such as a proxy or a
// test server. It defaults to https://api.anthropic.com.
func WithBaseURL(baseURL string) Option {
	return func(s *settings) { s.baseURL = baseURL }
}

// NewChat builds a chat model aimed at the Anthropic API. Register tools on the
// registry before or after construction; Build reads them from the same pointer.
func NewChat(apiKey string, registry *gogent.ToolRegistry, opts ...Option) *ModelBuilder {
	s := settings{baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt(&s)
	}

	client := anthropicsdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(s.baseURL),
	)

	maxTokens := defaultMaxTokens
	return &ModelBuilder{
		client:       &client,
		toolRegistry: registry,
		settings: modelSettings{
			model:     defaultModel,
			maxTokens: &maxTokens,
		},
	}
}

type ModelBuilder struct {
	client       *anthropicsdk.Client
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

func (b *ModelBuilder) WithMaxTokens(maxTokens int) *ModelBuilder {
	b.settings.maxTokens = &maxTokens
	return b
}

func (b *ModelBuilder) WithTemperature(temperature float64) *ModelBuilder {
	b.settings.temperature = &temperature
	return b
}

func (b *ModelBuilder) WithTopP(topP float64) *ModelBuilder {
	b.settings.topP = &topP
	return b
}

func (b *ModelBuilder) WithTopK(topK int) *ModelBuilder {
	b.settings.topK = &topK
	return b
}

func (b *ModelBuilder) WithStop(stopSequences ...string) *ModelBuilder {
	b.settings.stopSequences = append([]string(nil), stopSequences...)
	return b
}

// WithEffort sets output_config.effort, which controls how many tokens the model
// spends on a response. Accepted values are "low", "medium", "high", "xhigh",
// and "max". When unset, the model default applies.
func (b *ModelBuilder) WithEffort(effort string) *ModelBuilder {
	b.settings.effort = &effort
	return b
}

func (b *ModelBuilder) WithTools(tools ...gogent.Tool) *ModelBuilder {
	b.tools = append([]gogent.Tool(nil), tools...)
	return b
}

func (b *ModelBuilder) Build() (*Model, error) {
	if b.client == nil {
		return nil, errors.New("anthropic: client is required")
	}
	if b.settings.model == "" {
		return nil, errors.New("anthropic: model is required")
	}
	if b.settings.maxTokens != nil && *b.settings.maxTokens <= 0 {
		return nil, errors.New("anthropic: max tokens must be greater than zero")
	}
	if err := validateOptionalFloat("temperature", b.settings.temperature, 0, 1); err != nil {
		return nil, err
	}
	if err := validateOptionalFloat("top_p", b.settings.topP, 0, 1); err != nil {
		return nil, err
	}
	if b.settings.topK != nil && *b.settings.topK <= 0 {
		return nil, errors.New("anthropic: top_k must be greater than zero")
	}
	if b.settings.effort != nil && !validEffort(*b.settings.effort) {
		return nil, errors.New("anthropic: effort must be one of low, medium, high, xhigh, max")
	}

	settings := b.settings
	if settings.maxTokens == nil {
		maxTokens := defaultMaxTokens
		settings.maxTokens = &maxTokens
	}

	tools := append([]gogent.Tool(nil), b.tools...)
	if len(tools) == 0 && b.toolRegistry != nil {
		tools = b.toolRegistry.Tools()
	}

	return &Model{
		client:       b.client,
		settings:     settings,
		tools:        tools,
		toolRegistry: b.toolRegistry,
	}, nil
}

func validEffort(effort string) bool {
	for _, v := range []string{"low", "medium", "high", "xhigh", "max"} {
		if effort == v {
			return true
		}
	}
	return false
}

// validateOptionalFloat returns an error when a configured optional float is outside the allowed range.
func validateOptionalFloat(name string, value *float64, minVal, maxVal float64) error {
	if value == nil {
		return nil
	}
	if *value < minVal || *value > maxVal {
		return fmt.Errorf("anthropic: %s must be between %v and %v", name, minVal, maxVal)
	}
	return nil
}
