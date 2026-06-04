package openai

import (
	"context"
	"encoding/json"
	"testing"

	openaisdk "github.com/openai/openai-go"

	"github.com/cgund98/gogent"
)

func TestToChatCompletionNewParams(t *testing.T) {
	t.Parallel()

	temp := 0.7
	maxTokens := 128
	toolChoice := "auto"

	settings := modelSettings{
		systemPrompt: "You are helpful.",
		model:        "gpt-4o-mini",
		temperature:  &temp,
		maxTokens:    &maxTokens,
		toolChoice:   &toolChoice,
	}
	tools := []gogent.Tool{stubTool{name: "get_weather"}}

	history := []gogent.Message{
		gogent.NewUserMessage("What's the weather?"),
	}

	params, err := toChatCompletionNewParams(settings, tools, history)
	if err != nil {
		t.Fatalf("toChatCompletionNewParams() error = %v", err)
	}

	if params.Model != openaisdk.ChatModel("gpt-4o-mini") {
		t.Fatalf("Model = %q, want gpt-4o-mini", params.Model)
	}
	if len(params.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(params.Messages))
	}
	if len(params.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(params.Tools))
	}
	if params.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("tool name = %q, want get_weather", params.Tools[0].Function.Name)
	}
}

type stubTool struct {
	name string
}

func (t stubTool) Name() string { return t.name }
func (t stubTool) Description() string {
	return "Get weather for a city"
}
func (t stubTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)
}
func (t stubTool) RequiresApproval() bool { return false }
func (t stubTool) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"temp_c":18}`), nil
}
