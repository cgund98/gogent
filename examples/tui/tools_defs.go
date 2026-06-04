package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

func jsonschemaFromStruct(v any) json.RawMessage {
	r := jsonschema.Reflector{
		DoNotReference: true,
	}
	b, err := json.Marshal(r.Reflect(v))
	if err != nil {
		panic(err)
	}
	return b
}

// ------------------------------------------------------------

type AdditionToolArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

type AdditionTool struct{}

func (t *AdditionTool) Name() string                { return "addition" }
func (t *AdditionTool) Description() string         { return "Add two numbers" }
func (t *AdditionTool) Parameters() json.RawMessage { return additionSchema }
func (t *AdditionTool) RequiresApproval() bool      { return false }

func (t *AdditionTool) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var args AdditionToolArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}
	result := args.A + args.B
	return json.RawMessage(fmt.Sprintf(`{"result":%d}`, result)), nil
}

var additionSchema = jsonschemaFromStruct(new(AdditionToolArgs))

// ------------------------------------------------------------

type MultiplicationToolArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

type MultiplicationTool struct{}

func (t *MultiplicationTool) Name() string                { return "multiplication" }
func (t *MultiplicationTool) Description() string         { return "Multiply two numbers" }
func (t *MultiplicationTool) Parameters() json.RawMessage { return multiplicationSchema }
func (t *MultiplicationTool) RequiresApproval() bool      { return true }

func (t *MultiplicationTool) Execute(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
	var args MultiplicationToolArgs
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}
	result := args.A * args.B
	return json.RawMessage(fmt.Sprintf(`{"result":%d}`, result)), nil
}

var multiplicationSchema = jsonschemaFromStruct(new(MultiplicationToolArgs))
