package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/invopop/jsonschema"
	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gogent/examples/core"
	"github.com/cgund98/gogent/inmemory"
	"github.com/cgund98/gogent/openai"
)

type AdditionToolArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

type AdditionTool struct {
}

func jsonschemaFromStruct(v any) json.RawMessage {
	r := jsonschema.Reflector{
		DoNotReference: true, // flat schema, no $ref
	}
	b, err := json.Marshal(r.Reflect(v))
	if err != nil {
		panic(err)
	}
	return b
}

var additionSchema = jsonschemaFromStruct(new(AdditionToolArgs))
var multiplicationSchema = jsonschemaFromStruct(new(MultiplicationToolArgs))

func (t *AdditionTool) Name() string {
	return "addition"
}

func (t *AdditionTool) Description() string {
	return "Add two numbers"
}

func (t *AdditionTool) Parameters() json.RawMessage {
	return additionSchema
}

func (t *AdditionTool) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var args AdditionToolArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}
	result := args.A + args.B
	return json.RawMessage(fmt.Sprintf(`{"result":%d}`, result)), nil
}

func (t *AdditionTool) RequiresApproval() bool {
	return false
}

type MultiplicationToolArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

type MultiplicationTool struct {
}

func (t *MultiplicationTool) Name() string {
	return "multiplication"
}

func (t *MultiplicationTool) Description() string {
	return "Multiply two numbers"
}

func (t *MultiplicationTool) Parameters() json.RawMessage {
	return multiplicationSchema
}

func (t *MultiplicationTool) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var args MultiplicationToolArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, err
	}
	result := args.A * args.B
	return json.RawMessage(fmt.Sprintf(`{"result":%d}`, result)), nil
}

func (t *MultiplicationTool) RequiresApproval() bool {
	return true
}

func main() {

	ctx := context.Background()

	store := inmemory.NewMessageStore()
	toolRegistry := gogent.NewToolRegistry()
	if err := toolRegistry.RegisterTool(&AdditionTool{}); err != nil {
		log.Fatalf("register addition tool: %v", err)
	}
	if err := toolRegistry.RegisterTool(&MultiplicationTool{}); err != nil {
		log.Fatalf("register multiplication tool: %v", err)
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	client := openaisdk.NewClient(option.WithAPIKey(cfg.OpenAIAPIKey))
	model, err := openai.NewChat(&client, toolRegistry).
		WithModel("gpt-4o-mini").
		WithSystemPrompt("You are a helpful assistant. Use tools to solve problems wherever possible.").
		Build()
	if err != nil {
		log.Fatalf("build model: %v", err)
	}
	fmt.Println("Model built")

	agent := gogent.NewAgent(store, gogent.NopBroadcaster{}, model, toolRegistry, 5)
	fmt.Println("Agent created")

	fmt.Println("Running agent with addition tool")
	chatID, err := agent.RunNewChat(ctx, "Add these numbers: 2 and 3")
	if err != nil {
		log.Fatalf("run agent: %v", err)
	}

	messages, err := store.Load(ctx, chatID)
	if err != nil {
		log.Fatalf("load messages: %v", err)
	}

	fmt.Println("--------------------------------")
	for _, message := range messages {
		fmt.Printf("%s: %s\n", message.Role, message.Content)
	}
	fmt.Println("--------------------------------")

	fmt.Println("Running agent with multiplication tool")
	if err := agent.RunWithUserInput(ctx, chatID, "Multiply these numbers: 454 and 543"); err != nil {
		log.Fatalf("run agent: %v", err)
	}

	messages, err = store.Load(ctx, chatID)
	if err != nil {
		log.Fatalf("load messages: %v", err)
	}

	fmt.Println("--------------------------------")
	for _, message := range messages {
		fmt.Printf("%s: %s\n", message.Role, message.Content)
	}
	fmt.Println("--------------------------------")

	pendingToolCalls, err := agent.ListPendingToolCalls(ctx, chatID)
	if err != nil {
		log.Fatalf("list pending tool calls: %v", err)
	}
	if len(pendingToolCalls) == 0 {
		log.Fatalf("no pending tool calls")
	} else if len(pendingToolCalls) > 1 {
		log.Fatalf("multiple pending tool calls")
	}
	fmt.Println("Pending tool calls:")
	for _, toolCall := range pendingToolCalls {
		fmt.Printf("%s: tool_call_id=%s, args=%s\n", toolCall.ToolName, toolCall.ToolCallID, toolCall.Args)
	}
	fmt.Println("--------------------------------")

	fmt.Println("Approving tool call")

	pendingToolCall := pendingToolCalls[0]
	if err := agent.ApproveToolCall(ctx, chatID, pendingToolCall.MessageID, pendingToolCall.ToolCallID); err != nil {
		log.Fatalf("approve tool call: %v", err)
	}

	messages, err = store.Load(ctx, chatID)
	if err != nil {
		log.Fatalf("load messages: %v", err)
	}
	fmt.Println("--------------------------------")
	for _, message := range messages {
		fmt.Printf("%s: %s\n", message.Role, message.Content)
	}
	fmt.Println("--------------------------------")

	fmt.Println("Done")
}
