package openai

import (
	"testing"

	openaisdk "github.com/openai/openai-go"
)

func TestUsageFromCompletion(t *testing.T) {
	usage := usageFromCompletion(openaisdk.CompletionUsage{
		PromptTokens:     1200,
		CompletionTokens: 30,
		PromptTokensDetails: openaisdk.CompletionUsagePromptTokensDetails{
			CachedTokens: 400,
		},
	})
	if usage == nil || usage.Input != 1200 || usage.Output != 30 || usage.Cached != 400 {
		t.Fatalf("usage = %#v", usage)
	}
	if usageFromCompletion(openaisdk.CompletionUsage{}) != nil {
		t.Fatal("empty usage should be omitted")
	}
}
