// Package deepseek is the DeepSeek model provider. It speaks to DeepSeek's Chat
// Completions API and implements gogent.Model through the shared chat builder.
package deepseek

import (
	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gogent/openai"
)

const baseURL = "https://api.deepseek.com"

// Option changes the requests a DeepSeek chat model sends.
type Option func(*settings)

type settings struct {
	baseURL        string
	enableThinking bool
}

// WithThinking sends thinking {"type": "enabled"} on every request.
// DeepSeek models support a thinking mode; enabling it includes a reasoning
// pass in the model response.
func WithThinking() Option {
	return func(s *settings) { s.enableThinking = true }
}

// NewChat builds a chat model aimed at the DeepSeek API.
func NewChat(apiKey string, registry *gogent.ToolRegistry, opts ...Option) *openai.ModelBuilder {
	s := settings{baseURL: baseURL}
	for _, opt := range opts {
		opt(&s)
	}
	requestOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(s.baseURL),
	}
	if s.enableThinking {
		requestOptions = append(requestOptions, option.WithJSONSet("thinking", map[string]string{"type": "enabled"}))
	}
	client := openaisdk.NewClient(requestOptions...)
	return openai.NewChat(&client, registry)
}
