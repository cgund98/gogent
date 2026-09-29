package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/cgund98/gogent"
)

// toMessageParams converts gogent history into Anthropic input messages.
//
// Anthropic has no "tool" role: tool results travel as tool_result blocks on a
// user turn, and the API combines consecutive same-role turns anyway. This
// function therefore coalesces adjacent messages that map to the same role so
// that N tool results become one user message with N tool_result blocks.
func toMessageParams(history []gogent.Message) ([]anthropicsdk.MessageParam, error) {
	messages := make([]anthropicsdk.MessageParam, 0, len(history))

	for _, message := range history {
		role, blocks, err := toContentBlocks(message)
		if err != nil {
			return nil, err
		}

		if n := len(messages); n > 0 && messages[n-1].Role == role {
			messages[n-1].Content = append(messages[n-1].Content, blocks...)
			continue
		}

		messages = append(messages, anthropicsdk.MessageParam{
			Role:    role,
			Content: blocks,
		})
	}

	return messages, nil
}

// toContentBlocks maps a single gogent message to an Anthropic role and content blocks.
func toContentBlocks(message gogent.Message) (anthropicsdk.MessageParamRole, []anthropicsdk.ContentBlockParamUnion, error) {
	switch message.Role {
	case gogent.MessageRoleUser:
		return anthropicsdk.MessageParamRoleUser, []anthropicsdk.ContentBlockParamUnion{
			anthropicsdk.NewTextBlock(message.Content),
		}, nil
	case gogent.MessageRoleAssistant:
		blocks, err := toAssistantContentBlocks(message)
		if err != nil {
			return "", nil, err
		}
		return anthropicsdk.MessageParamRoleAssistant, blocks, nil
	case gogent.MessageRoleTool:
		if message.ToolCallID == "" {
			return "", nil, fmt.Errorf("anthropic: tool message %q is missing tool_call_id", message.ID)
		}
		return anthropicsdk.MessageParamRoleUser, []anthropicsdk.ContentBlockParamUnion{
			anthropicsdk.NewToolResultBlock(message.ToolCallID, message.Content, false),
		}, nil
	default:
		return "", nil, fmt.Errorf("anthropic: unsupported message role %q", message.Role)
	}
}

// toAssistantContentBlocks converts an assistant message, stripping gogent-only
// tool call metadata and emitting tool_use blocks for requested tools.
func toAssistantContentBlocks(message gogent.Message) ([]anthropicsdk.ContentBlockParamUnion, error) {
	blocks := make([]anthropicsdk.ContentBlockParamUnion, 0, len(message.ToolCalls)+1)

	if message.Content != "" {
		blocks = append(blocks, anthropicsdk.NewTextBlock(message.Content))
	}

	for _, toolCall := range message.ToolCalls {
		input, err := toToolInput(toolCall)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, anthropicsdk.NewToolUseBlock(toolCall.ID, input, toolCall.ToolName))
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("anthropic: assistant message %q has no content", message.ID)
	}

	return blocks, nil
}

// toToolInput decodes gogent tool call arguments into the JSON value Anthropic
// expects for a tool_use block's input field.
func toToolInput(toolCall gogent.ToolCall) (any, error) {
	if len(toolCall.Args) == 0 {
		return map[string]any{}, nil
	}

	var input any
	if err := json.Unmarshal(toolCall.Args, &input); err != nil {
		return nil, fmt.Errorf("anthropic: tool call %s args: %w", toolCall.ID, err)
	}
	if input == nil {
		return map[string]any{}, nil
	}

	return input, nil
}

// toToolUnionParams converts registered tools into Anthropic tool definitions.
func toToolUnionParams(tools []gogent.Tool) ([]anthropicsdk.ToolUnionParam, error) {
	params := make([]anthropicsdk.ToolUnionParam, len(tools))

	for i, tool := range tools {
		var schema struct {
			Properties any      `json:"properties"`
			Required   []string `json:"required"`
		}
		if len(tool.Parameters()) > 0 {
			if err := json.Unmarshal(tool.Parameters(), &schema); err != nil {
				return nil, fmt.Errorf("anthropic: tool %s parameters: %w", tool.Name(), err)
			}
		}

		toolParam := anthropicsdk.ToolParam{
			Name: tool.Name(),
			InputSchema: anthropicsdk.ToolInputSchemaParam{
				Properties: schema.Properties,
				Required:   schema.Required,
			},
		}
		if description := tool.Description(); description != "" {
			toolParam.Description = anthropicsdk.String(description)
		}

		params[i] = anthropicsdk.ToolUnionParam{OfTool: &toolParam}
	}

	return params, nil
}

// fromAnthropicMessage parses an Anthropic response into a gogent message,
// collecting text and mapping each tool_use block to a pending tool call.
func fromAnthropicMessage(message *anthropicsdk.Message) (gogent.Message, error) {
	if message == nil {
		return gogent.Message{}, errors.New("anthropic: nil message in response")
	}

	content := ""
	var toolCalls []gogent.ToolCall

	for _, block := range message.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, gogent.NewPendingToolCall(block.ID, block.Name, toRawToolInput(block.Input)))
		}
	}

	var assistantMessage gogent.Message
	if len(toolCalls) == 0 {
		assistantMessage = gogent.NewAssistantMessage(content)
	} else {
		assistantMessage = gogent.NewAssistantMessageWithToolCalls(content, toolCalls)
	}
	assistantMessage.Usage = usageFromMessage(message.Usage)

	return assistantMessage, nil
}

// toRawToolInput returns the raw JSON arguments for a tool_use block, defaulting
// to an empty object when the provider sent none.
func toRawToolInput(input json.RawMessage) json.RawMessage {
	if len(input) == 0 || !json.Valid(input) {
		return json.RawMessage("{}")
	}
	return append(json.RawMessage(nil), input...)
}

// usageFromMessage maps Anthropic token counts onto a gogent usage value, or nil
// when the provider reported none.
func usageFromMessage(usage anthropicsdk.Usage) *gogent.Usage {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CacheReadInputTokens == 0 {
		return nil
	}
	return &gogent.Usage{
		Input:  int(usage.InputTokens),
		Output: int(usage.OutputTokens),
		Cached: int(usage.CacheReadInputTokens),
	}
}
