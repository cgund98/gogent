package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gogent/examples/core"
	"github.com/cgund98/gogent/inmemory"
	"github.com/cgund98/gogent/openai"
)

const maxIterations = 10
const modelName = "gpt-4o-mini"
const prompt = `
You are a helpful assistant. Use tools to solve problems wherever possible.
Format every assistant reply in GitHub-flavored Markdown (headings, lists, emphasis, and fenced code blocks when useful).
Do not use LaTeX or $...$ math delimiters. Write arithmetic with plain text and * for multiplication (e.g. 3 * 4 = 12).
`

func main() {
	ctx := context.Background()

	store := inmemory.NewMessageStore()
	registry := gogent.NewToolRegistry()
	if err := registry.RegisterTool(&AdditionTool{}); err != nil {
		log.Fatalf("register addition tool: %v", err)
	}
	if err := registry.RegisterTool(&MultiplicationTool{}); err != nil {
		log.Fatalf("register multiplication tool: %v", err)
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	client := openaisdk.NewClient(option.WithAPIKey(cfg.OpenAIAPIKey))
	model, err := openai.NewChat(&client, registry).
		WithModel(modelName).
		WithSystemPrompt(prompt).
		Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build model: %v\n", err)
		os.Exit(1)
	}

	events := gogent.NewChannelBroadcaster()
	agent := gogent.NewAgent(store, events, model, registry, maxIterations)

	m := newChatModel(ctx, agent, store, registry, events)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run tui: %v\n", err)
		os.Exit(1)
	}
}
