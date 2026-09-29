package anthropic

import (
	"encoding/json"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/cgund98/gogent"
)

func TestToMessageParamsUserText(t *testing.T) {
	messages, err := toMessageParams([]gogent.Message{
		gogent.NewUserMessage("hello"),
	})
	if err != nil {
		t.Fatalf("toMessageParams() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Role != anthropicsdk.MessageParamRoleUser {
		t.Fatalf("Role = %q, want user", messages[0].Role)
	}
	if len(messages[0].Content) != 1 || messages[0].Content[0].OfText == nil {
		t.Fatalf("content = %+v, want a single text block", messages[0].Content)
	}
	if messages[0].Content[0].OfText.Text != "hello" {
		t.Fatalf("text = %q, want hello", messages[0].Content[0].OfText.Text)
	}
}

func TestToMessageParamsAssistantToolRequestAndCoalescedResults(t *testing.T) {
	history := []gogent.Message{
		gogent.NewAssistantMessageWithToolCalls("", []gogent.ToolCall{
			{
				ID:              "toolu_a",
				ToolName:        "get_weather",
				Args:            json.RawMessage(`{"city":"Paris"}`),
				ApprovalStatus:  gogent.ApprovalStatusApproved,
				ExecutionStatus: gogent.ExecutionStatusCompleted,
				Result:          json.RawMessage(`{"temp_c":18}`),
			},
			{
				ID:              "toolu_b",
				ToolName:        "get_weather",
				Args:            json.RawMessage(`{"city":"London"}`),
				ApprovalStatus:  gogent.ApprovalStatusApproved,
				ExecutionStatus: gogent.ExecutionStatusCompleted,
				Result:          json.RawMessage(`{"temp_c":12}`),
			},
		}),
		gogent.NewToolResultMessage("toolu_a", `{"temp_c":18}`),
		gogent.NewToolResultMessage("toolu_b", `{"temp_c":12}`),
	}

	messages, err := toMessageParams(history)
	if err != nil {
		t.Fatalf("toMessageParams() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2 (assistant + coalesced tool results)", len(messages))
	}

	assistant := messages[0]
	if assistant.Role != anthropicsdk.MessageParamRoleAssistant {
		t.Fatalf("assistant Role = %q", assistant.Role)
	}
	if len(assistant.Content) != 2 {
		t.Fatalf("len(assistant.Content) = %d, want 2 tool_use blocks", len(assistant.Content))
	}
	if assistant.Content[0].OfToolUse == nil || assistant.Content[0].OfToolUse.Name != "get_weather" {
		t.Fatalf("assistant.Content[0] = %+v, want a tool_use block", assistant.Content[0])
	}
	if assistant.Content[0].OfToolUse.ID != "toolu_a" {
		t.Fatalf("tool_use id = %q, want toolu_a", assistant.Content[0].OfToolUse.ID)
	}

	toolResults := messages[1]
	if toolResults.Role != anthropicsdk.MessageParamRoleUser {
		t.Fatalf("tool results Role = %q, want user", toolResults.Role)
	}
	if len(toolResults.Content) != 2 {
		t.Fatalf("len(toolResults.Content) = %d, want 2 tool_result blocks", len(toolResults.Content))
	}
	first := toolResults.Content[0].OfToolResult
	if first == nil {
		t.Fatalf("toolResults.Content[0] = %+v, want a tool_result block", toolResults.Content[0])
	}
	if first.ToolUseID != "toolu_a" {
		t.Fatalf("tool_result tool_use_id = %q, want toolu_a", first.ToolUseID)
	}
	if len(first.Content) != 1 || first.Content[0].OfText == nil || first.Content[0].OfText.Text != `{"temp_c":18}` {
		t.Fatalf("tool_result content = %+v, want the raw result text", first.Content)
	}
}

func TestToMessageParamsCoalescesConsecutiveUserTurns(t *testing.T) {
	messages, err := toMessageParams([]gogent.Message{
		gogent.NewUserMessage("first"),
		gogent.NewUserMessage("second"),
	})
	if err != nil {
		t.Fatalf("toMessageParams() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1 coalesced user turn", len(messages))
	}
	if len(messages[0].Content) != 2 {
		t.Fatalf("len(content) = %d, want 2 text blocks", len(messages[0].Content))
	}
}

func TestToMessageParamsRejectsEmptyAssistant(t *testing.T) {
	_, err := toMessageParams([]gogent.Message{{
		ID:   "msg_empty",
		Role: gogent.MessageRoleAssistant,
	}})
	if err == nil {
		t.Fatal("toMessageParams() error = nil, want error for empty assistant message")
	}
}

func TestToToolUnionParams(t *testing.T) {
	tools := []gogent.Tool{
		stubTool{
			name:        "get_weather",
			description: "Get the weather",
			parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		},
	}

	params, err := toToolUnionParams(tools)
	if err != nil {
		t.Fatalf("toToolUnionParams() error = %v", err)
	}
	if len(params) != 1 || params[0].OfTool == nil {
		t.Fatalf("params = %+v, want one tool", params)
	}
	tool := params[0].OfTool
	if tool.Name != "get_weather" {
		t.Fatalf("Name = %q", tool.Name)
	}
	if tool.Description.Value != "Get the weather" {
		t.Fatalf("Description = %q", tool.Description.Value)
	}
	if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "city" {
		t.Fatalf("Required = %v, want [city]", tool.InputSchema.Required)
	}
	if tool.InputSchema.Properties == nil {
		t.Fatal("Properties is nil, want the tool schema properties")
	}
}

func TestToToolUnionParamsInvalidSchema(t *testing.T) {
	_, err := toToolUnionParams([]gogent.Tool{
		stubTool{name: "broken", parameters: json.RawMessage(`not-json`)},
	})
	if err == nil {
		t.Fatal("toToolUnionParams() error = nil, want error for invalid schema")
	}
}

func TestFromAnthropicMessage(t *testing.T) {
	message, err := fromAnthropicMessage(&anthropicsdk.Message{
		Content: []anthropicsdk.ContentBlockUnion{
			{Type: "text", Text: "Let me check."},
			{Type: "tool_use", ID: "toolu_1", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)},
		},
		Usage: anthropicsdk.Usage{InputTokens: 10, OutputTokens: 5, CacheReadInputTokens: 2},
	})
	if err != nil {
		t.Fatalf("fromAnthropicMessage() error = %v", err)
	}
	if message.Role != gogent.MessageRoleAssistant {
		t.Fatalf("Role = %q, want assistant", message.Role)
	}
	if message.Content != "Let me check." {
		t.Fatalf("Content = %q", message.Content)
	}
	if !message.HasToolCalls() || len(message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %+v, want one pending call", message.ToolCalls)
	}
	toolCall := message.ToolCalls[0]
	if toolCall.ID != "toolu_1" || toolCall.ToolName != "get_weather" {
		t.Fatalf("toolCall = %+v", toolCall)
	}
	if string(toolCall.Args) != `{"city":"Paris"}` {
		t.Fatalf("Args = %s, want {\"city\":\"Paris\"}", toolCall.Args)
	}
	if toolCall.ApprovalStatus != gogent.ApprovalStatusPending {
		t.Fatalf("ApprovalStatus = %q, want pending", toolCall.ApprovalStatus)
	}
	if message.Usage == nil || message.Usage.Input != 10 || message.Usage.Output != 5 || message.Usage.Cached != 2 {
		t.Fatalf("Usage = %+v", message.Usage)
	}
}

func TestFromAnthropicMessageTextOnlyHasNoUsage(t *testing.T) {
	message, err := fromAnthropicMessage(&anthropicsdk.Message{
		Content: []anthropicsdk.ContentBlockUnion{{Type: "text", Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("fromAnthropicMessage() error = %v", err)
	}
	if message.HasToolCalls() {
		t.Fatal("expected no tool calls")
	}
	if message.Usage != nil {
		t.Fatalf("Usage = %+v, want nil", message.Usage)
	}
}

func TestFromAnthropicMessageNil(t *testing.T) {
	if _, err := fromAnthropicMessage(nil); err == nil {
		t.Fatal("fromAnthropicMessage(nil) error = nil, want error")
	}
}
