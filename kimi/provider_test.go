package kimi

import (
	"testing"

	"github.com/cgund98/gogent"
)

func TestNewChatBuildsKimiModel(t *testing.T) {
	model, err := NewChat("test-key", gogent.NewToolRegistry()).
		WithModel("kimi-k2.6").
		WithSystemPrompt("hello").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if model.SystemPrompt() != "hello" {
		t.Fatalf("prompt = %q", model.SystemPrompt())
	}
}
