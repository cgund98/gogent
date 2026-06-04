package openai

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	openaisdk "github.com/openai/openai-go"

	"github.com/cgund98/gogent"
)

type openAIChatMessage struct {
	Role       string           `json:"role"`
	Content    *string          `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolDefinition struct {
	Type     string               `json:"type"`
	Function openAIFunctionSchema `json:"function"`
}

type openAIFunctionSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// toOpenAIChatMessages converts gogent history into OpenAI chat completion messages,
// optionally prepending a system prompt.
func toOpenAIChatMessages(systemPrompt string, history []gogent.Message) ([]openAIChatMessage, error) {
	messages := make([]openAIChatMessage, 0, len(history)+1)

	if systemPrompt != "" {
		content := systemPrompt
		messages = append(messages, openAIChatMessage{
			Role:    "system",
			Content: &content,
		})
	}

	for _, message := range history {
		openAIMessage, err := toOpenAIChatMessage(message)
		if err != nil {
			return nil, err
		}
		messages = append(messages, openAIMessage)
	}

	return messages, nil
}

// toOpenAIChatMessage converts a single gogent message to the OpenAI wire format.
func toOpenAIChatMessage(message gogent.Message) (openAIChatMessage, error) {
	switch message.Role {
	case gogent.MessageRoleUser:
		content := message.Content
		return openAIChatMessage{
			Role:    "user",
			Content: &content,
		}, nil
	case gogent.MessageRoleAssistant:
		return toOpenAIAssistantMessage(message), nil
	case gogent.MessageRoleTool:
		if message.ToolCallID == "" {
			return openAIChatMessage{}, fmt.Errorf("openai: tool message %q is missing tool_call_id", message.ID)
		}
		content := message.Content
		return openAIChatMessage{
			Role:       "tool",
			Content:    &content,
			ToolCallID: message.ToolCallID,
		}, nil
	default:
		return openAIChatMessage{}, fmt.Errorf("openai: unsupported message role %q", message.Role)
	}
}

// toOpenAIAssistantMessage converts an assistant message, stripping gogent-only tool call metadata.
func toOpenAIAssistantMessage(message gogent.Message) openAIChatMessage {
	openAIMessage := openAIChatMessage{
		Role: "assistant",
	}

	if message.Content != "" {
		content := message.Content
		openAIMessage.Content = &content
	}

	if len(message.ToolCalls) > 0 {
		openAIMessage.ToolCalls = toOpenAIToolCalls(message.ToolCalls)
	}

	return openAIMessage
}

// toOpenAIToolCalls converts tool calls to OpenAI function tool call params.
func toOpenAIToolCalls(toolCalls []gogent.ToolCall) []openAIToolCall {
	openAIToolCalls := make([]openAIToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		openAIToolCalls[i] = openAIToolCall{
			ID:   toolCall.ID,
			Type: "function",
			Function: openAIFunctionCall{
				Name:      toolCall.ToolName,
				Arguments: string(toolCall.Args),
			},
		}
	}

	return openAIToolCalls
}

// toOpenAIToolDefinitions converts registered tools into OpenAI function tool definitions.
func toOpenAIToolDefinitions(tools []gogent.Tool) []openAIToolDefinition {
	definitions := make([]openAIToolDefinition, len(tools))
	for i, tool := range tools {
		definitions[i] = openAIToolDefinition{
			Type: "function",
			Function: openAIFunctionSchema{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		}
	}

	return definitions
}

// fromOpenAIAssistantMessage parses an OpenAI assistant message into a gogent message.
func fromOpenAIAssistantMessage(message openAIChatMessage) (gogent.Message, error) {
	if message.Role != "assistant" {
		return gogent.Message{}, fmt.Errorf("openai: expected assistant message, got %q", message.Role)
	}

	content := ""
	if message.Content != nil {
		content = *message.Content
	}

	if len(message.ToolCalls) == 0 {
		return gogent.NewAssistantMessage(content), nil
	}

	toolCalls := make([]gogent.ToolCall, len(message.ToolCalls))
	for i, toolCall := range message.ToolCalls {
		parsed, err := fromOpenAIToolCall(toolCall)
		if err != nil {
			return gogent.Message{}, err
		}
		toolCalls[i] = parsed
	}

	assistantMessage := gogent.NewAssistantMessageWithToolCalls(content, toolCalls)
	assistantMessage.ID = uuid.New().String()
	return assistantMessage, nil
}

// fromOpenAIToolCall parses an OpenAI tool call into a pending gogent ToolCall.
func fromOpenAIToolCall(toolCall openAIToolCall) (gogent.ToolCall, error) {
	if toolCall.ID == "" {
		return gogent.ToolCall{}, fmt.Errorf("openai: tool call is missing id")
	}
	if toolCall.Function.Name == "" {
		return gogent.ToolCall{}, fmt.Errorf("openai: tool call %q is missing function name", toolCall.ID)
	}

	args := json.RawMessage(toolCall.Function.Arguments)
	if !json.Valid(args) {
		args = json.RawMessage("{}")
	}

	return gogent.NewPendingToolCall(toolCall.ID, toolCall.Function.Name, args), nil
}

// fromChatCompletionMessage parses an OpenAI SDK assistant message into a gogent message.
func fromChatCompletionMessage(message openaisdk.ChatCompletionMessage) (gogent.Message, error) {
	wireMessage := openAIChatMessage{Role: "assistant"}
	if message.Content != "" {
		content := message.Content
		wireMessage.Content = &content
	}
	if len(message.ToolCalls) > 0 {
		wireMessage.ToolCalls = make([]openAIToolCall, len(message.ToolCalls))
		for i, toolCall := range message.ToolCalls {
			wireMessage.ToolCalls[i] = openAIToolCall{
				ID:   toolCall.ID,
				Type: "function",
				Function: openAIFunctionCall{
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				},
			}
		}
	}

	return fromOpenAIAssistantMessage(wireMessage)
}
