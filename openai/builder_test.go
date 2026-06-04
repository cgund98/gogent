package openai

import (
	"testing"

	openaisdk "github.com/openai/openai-go"

	"github.com/cgund98/gogent"
)

func TestModelBuilderBuild(t *testing.T) {
	t.Parallel()

	temp := 0.7
	maxTokens := 512
	client := openaisdk.Client{}

	model, err := NewChat(&client, gogent.NewToolRegistry()).
		WithSystemPrompt("You are a helpful assistant.").
		WithModel("gpt-4o").
		WithTemperature(temp).
		WithMaxTokens(maxTokens).
		WithTopP(0.9).
		WithFrequencyPenalty(0.1).
		WithPresencePenalty(0.2).
		WithStop("\n", "END").
		WithToolChoice("auto").
		WithSeed(42).
		WithUser("user-123").
		WithParallelToolCalls(true).
		WithJSONResponse().
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if model == nil {
		t.Fatal("Build() returned nil model")
	}
	if model.client != &client {
		t.Fatal("Build() did not preserve injected client")
	}
	if model.settings.model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", model.settings.model)
	}
}

func TestModelBuilderBuildIncludesRegistryTools(t *testing.T) {
	t.Parallel()

	client := openaisdk.Client{}
	registry := gogent.NewToolRegistry()
	if err := registry.RegisterTool(stubTool{name: "multiplication"}); err != nil {
		t.Fatalf("RegisterTool(multiplication) error = %v", err)
	}
	if err := registry.RegisterTool(stubTool{name: "addition"}); err != nil {
		t.Fatalf("RegisterTool(addition) error = %v", err)
	}

	model, err := NewChat(&client, registry).
		WithModel("gpt-4o-mini").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(model.tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2 from registry", len(model.tools))
	}
	if model.tools[0].Name() != "addition" {
		t.Fatalf("first tool = %q, want addition", model.tools[0].Name())
	}
	if model.settings.toolChoice == nil || *model.settings.toolChoice != "auto" {
		t.Fatalf("toolChoice = %v, want auto default", model.settings.toolChoice)
	}

	params, err := toChatCompletionNewParams(model.settings, model.tools, nil)
	if err != nil {
		t.Fatalf("toChatCompletionNewParams() error = %v", err)
	}
	if len(params.Tools) != 2 {
		t.Fatalf("len(params.Tools) = %d, want 2", len(params.Tools))
	}
}

func TestModelBuilderBuildValidation(t *testing.T) {
	t.Parallel()

	client := openaisdk.Client{}

	tests := []struct {
		name    string
		builder *ModelBuilder
	}{
		{
			name: "missing client",
			builder: &ModelBuilder{
				settings: modelSettings{
					model: defaultModel,
				},
			},
		},
		{
			name: "missing model",
			builder: &ModelBuilder{
				client: &client,
			},
		},
		{
			name: "invalid temperature",
			builder: &ModelBuilder{
				client: &client,
				settings: modelSettings{
					model:       defaultModel,
					temperature: ptr(3.0),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := tt.builder.Build(); err == nil {
				t.Fatal("Build() error = nil, want error")
			}
		})
	}
}

func ptr[T any](value T) *T {
	return &value
}
