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

// NewChat builds a chat model aimed at the Kimi API.
func NewChat(apiKey string, registry *gogent.ToolRegistry) *openai.ModelBuilder {
	client := openaisdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return openai.NewChat(&client, registry)
}
