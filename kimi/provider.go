// Package kimi is the Kimi model provider. It speaks to Moonshot's Chat
// Completions API and implements gogent.Model through the shared chat builder.
package kimi

import (
	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gogent/openai"
)

const baseURL = "https://api.moonshot.ai/v1"

// Option changes the requests a Kimi chat model sends.
type Option func(*settings)

type settings struct {
	baseURL         string
	disableThinking bool
}

// WithoutThinking sends thinking {"type": "disabled"} on every request.
// Kimi K2.6 thinks by default; turning it off starts replies sooner at some cost to quality.
func WithoutThinking() Option {
	return func(s *settings) { s.disableThinking = true }
}

// NewChat builds a chat model aimed at the Kimi API.
func NewChat(apiKey string, registry *gogent.ToolRegistry, opts ...Option) *openai.ModelBuilder {
	s := settings{baseURL: baseURL}
	for _, opt := range opts {
		opt(&s)
	}
	requestOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(s.baseURL),
	}
	if s.disableThinking {
		requestOptions = append(requestOptions, option.WithJSONSet("thinking", map[string]string{"type": "disabled"}))
	}
	client := openaisdk.NewClient(requestOptions...)
	return openai.NewChat(&client, registry)
}
