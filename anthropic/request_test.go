package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/cgund98/gogent"
)

func TestToMessageNewParams(t *testing.T) {
	settings := modelSettings{
		systemPrompt:  "You are helpful.",
		model:         "claude-opus-5-5",
		maxTokens:     ptr(4096),
		temperature:   ptr(0.5),
		topP:          ptr(0.9),
		topK:          ptr(40),
		stopSequences: []string{"END"},
		effort:        ptr("high"),
	}
	tools := []gogent.Tool{
		stubTool{
			name:        "get_weather",
			description: "Get the weather",
			parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		},
	}
	history := []gogent.Message{gogent.NewUserMessage("hi")}

	params, err := toMessageNewParams(settings, tools, history)
	if err != nil {
		t.Fatalf("toMessageNewParams() error = %v", err)
	}

	if params.Model != anthropicsdk.Model("claude-opus-5-5") {
		t.Fatalf("Model = %q", params.Model)
	}
	if params.MaxTokens != 4096 {
		t.Fatalf("MaxTokens = %d, want 4096", params.MaxTokens)
	}
	if len(params.System) != 1 || params.System[0].Text != "You are helpful." {
		t.Fatalf("System = %+v", params.System)
	}
	if params.Temperature.Value != 0.5 || params.TopP.Value != 0.9 || params.TopK.Value != 40 {
		t.Fatalf("sampling params = %+v", params)
	}
	if len(params.StopSequences) != 1 || params.StopSequences[0] != "END" {
		t.Fatalf("StopSequences = %v", params.StopSequences)
	}
	if params.OutputConfig.Effort != anthropicsdk.OutputConfigEffortHigh {
		t.Fatalf("OutputConfig.Effort = %q, want high", params.OutputConfig.Effort)
	}
	if len(params.Tools) != 1 || params.Tools[0].OfTool == nil || params.Tools[0].OfTool.Name != "get_weather" {
		t.Fatalf("Tools = %+v", params.Tools)
	}
	if len(params.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(params.Messages))
	}
}

func TestToMessageNewParamsOmitsEffortAndDefaultsMaxTokens(t *testing.T) {
	settings := modelSettings{model: "claude-haiku-4-5"}

	params, err := toMessageNewParams(settings, nil, []gogent.Message{gogent.NewUserMessage("hi")})
	if err != nil {
		t.Fatalf("toMessageNewParams() error = %v", err)
	}
	if params.OutputConfig.Effort != "" {
		t.Fatalf("OutputConfig.Effort = %q, want empty", params.OutputConfig.Effort)
	}
	if params.MaxTokens != int64(defaultMaxTokens) {
		t.Fatalf("MaxTokens = %d, want %d", params.MaxTokens, defaultMaxTokens)
	}
	if len(params.System) != 0 {
		t.Fatalf("System = %+v, want empty", params.System)
	}
	if len(params.Tools) != 0 {
		t.Fatalf("Tools = %+v, want empty", params.Tools)
	}
}

func TestGenerateResponseEndToEnd(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5-5",`+
			`"content":[{"type":"text","text":"Let me check."},`+
			`{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"Paris"}}],`+
			`"stop_reason":"tool_use","stop_sequence":null,`+
			`"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":2}}`)
	}))
	defer server.Close()

	registry := gogent.NewToolRegistry()
	if err := registry.RegisterTool(stubTool{
		name:       "get_weather",
		parameters: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
	}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	model, err := NewChat("test-key", registry, WithBaseURL(server.URL)).
		WithModel("claude-sonnet-5-5").
		WithSystemPrompt("You are helpful.").
		WithEffort("low").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	message, err := model.GenerateResponse(context.Background(), []gogent.Message{gogent.NewUserMessage("hi")})
	if err != nil {
		t.Fatalf("GenerateResponse() error = %v", err)
	}

	if body["model"] != "claude-sonnet-5-5" {
		t.Fatalf("request model = %v", body["model"])
	}
	if body["max_tokens"] != float64(defaultMaxTokens) {
		t.Fatalf("request max_tokens = %v", body["max_tokens"])
	}
	outputConfig, ok := body["output_config"].(map[string]any)
	if !ok || outputConfig["effort"] != "low" {
		t.Fatalf("request output_config = %v", body["output_config"])
	}
	if _, ok := body["tools"].([]any); !ok {
		t.Fatalf("request tools = %v", body["tools"])
	}

	if !message.HasToolCalls() || len(message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %+v, want one", message.ToolCalls)
	}
	if message.ToolCalls[0].ID != "toolu_1" || message.ToolCalls[0].ToolName != "get_weather" {
		t.Fatalf("toolCall = %+v", message.ToolCalls[0])
	}
	if string(message.ToolCalls[0].Args) != `{"city":"Paris"}` {
		t.Fatalf("Args = %s", message.ToolCalls[0].Args)
	}
	if message.Usage == nil || message.Usage.Input != 10 || message.Usage.Output != 5 || message.Usage.Cached != 2 {
		t.Fatalf("Usage = %+v", message.Usage)
	}
}
