package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/cgund98/gogent"
)

func TestNewChatBuildsAnthropicModel(t *testing.T) {
	model, err := NewChat("test-key", gogent.NewToolRegistry()).
		WithModel("claude-opus-5-5").
		WithSystemPrompt("hello").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if model.SystemPrompt() != "hello" {
		t.Fatalf("prompt = %q", model.SystemPrompt())
	}
	if model.settings.model != "claude-opus-5-5" {
		t.Fatalf("model = %q", model.settings.model)
	}
}

func TestBuildDefaultsMaxTokens(t *testing.T) {
	model, err := NewChat("test-key", gogent.NewToolRegistry()).Build()
	if err != nil {
		t.Fatal(err)
	}
	if model.settings.maxTokens == nil || *model.settings.maxTokens != defaultMaxTokens {
		t.Fatalf("maxTokens = %v, want %d", model.settings.maxTokens, defaultMaxTokens)
	}
	if model.settings.model != defaultModel {
		t.Fatalf("model = %q, want %q", model.settings.model, defaultModel)
	}
}

func TestModelBuilderBuildIncludesRegistryTools(t *testing.T) {
	registry := gogent.NewToolRegistry()
	if err := registry.RegisterTool(stubTool{name: "multiplication"}); err != nil {
		t.Fatalf("RegisterTool(multiplication) error = %v", err)
	}
	if err := registry.RegisterTool(stubTool{name: "addition"}); err != nil {
		t.Fatalf("RegisterTool(addition) error = %v", err)
	}

	model, err := NewChat("test-key", registry).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(model.tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2 from registry", len(model.tools))
	}
	if model.tools[0].Name() != "addition" {
		t.Fatalf("first tool = %q, want addition", model.tools[0].Name())
	}
}

func TestModelBuilderWithEffort(t *testing.T) {
	tests := []struct {
		name   string
		effort string
	}{
		{name: "low", effort: "low"},
		{name: "medium", effort: "medium"},
		{name: "high", effort: "high"},
		{name: "xhigh", effort: "xhigh"},
		{name: "max", effort: "max"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewChat("test-key", gogent.NewToolRegistry()).
				WithModel("claude-opus-5-5").
				WithEffort(tt.effort).
				Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if model.settings.effort == nil {
				t.Fatal("effort is nil, want set")
			}
			if *model.settings.effort != tt.effort {
				t.Fatalf("effort = %q, want %q", *model.settings.effort, tt.effort)
			}
		})
	}
}

func TestModelBuilderBuildValidation(t *testing.T) {
	client := anthropicsdk.NewClient(option.WithAPIKey("test-key"))

	tests := []struct {
		name    string
		builder *ModelBuilder
	}{
		{
			name:    "missing client",
			builder: &ModelBuilder{settings: modelSettings{model: defaultModel}},
		},
		{
			name:    "missing model",
			builder: &ModelBuilder{client: &client},
		},
		{
			name: "invalid temperature",
			builder: &ModelBuilder{
				client:   &client,
				settings: modelSettings{model: defaultModel, temperature: ptr(3.0)},
			},
		},
		{
			name: "invalid top_p",
			builder: &ModelBuilder{
				client:   &client,
				settings: modelSettings{model: defaultModel, topP: ptr(2.0)},
			},
		},
		{
			name: "invalid top_k",
			builder: &ModelBuilder{
				client:   &client,
				settings: modelSettings{model: defaultModel, topK: ptr(0)},
			},
		},
		{
			name: "invalid max tokens",
			builder: &ModelBuilder{
				client:   &client,
				settings: modelSettings{model: defaultModel, maxTokens: ptr(0)},
			},
		},
		{
			name: "invalid effort",
			builder: &ModelBuilder{
				client:   &client,
				settings: modelSettings{model: defaultModel, effort: ptr("extreme")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.builder.Build(); err == nil {
				t.Fatal("Build() error = nil, want error")
			}
		})
	}
}

type stubTool struct {
	name        string
	description string
	parameters  json.RawMessage
}

func (t stubTool) Name() string { return t.name }

func (t stubTool) Description() string { return t.description }

func (t stubTool) Parameters() json.RawMessage { return t.parameters }

func (t stubTool) RequiresApproval(context.Context, json.RawMessage) (gogent.ApprovalDecision, error) {
	return gogent.ApprovalDecision{}, nil
}

func (t stubTool) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func ptr[T any](value T) *T {
	return &value
}
