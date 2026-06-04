package gogent

import "testing"

func TestNewToolResultMessage(t *testing.T) {
	t.Parallel()

	msg := NewToolResultMessage("call_abc123", `{"temp_c":18}`)

	if msg.Role != MessageRoleTool {
		t.Fatalf("Role = %q, want %q", msg.Role, MessageRoleTool)
	}
	if msg.ToolCallID != "call_abc123" {
		t.Fatalf("ToolCallID = %q, want call_abc123", msg.ToolCallID)
	}
	if msg.Content != `{"temp_c":18}` {
		t.Fatalf("Content = %q, want JSON result", msg.Content)
	}
	if len(msg.ToolCalls) != 0 {
		t.Fatal("tool result message should not contain tool_calls")
	}
}

func TestMessageHasToolCalls(t *testing.T) {
	t.Parallel()

	assistant := NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_1", ToolName: "get_weather"}})
	if !assistant.HasToolCalls() {
		t.Fatal("assistant message with tool calls should return true")
	}

	toolResult := NewToolResultMessage("call_1", "ok")
	if toolResult.HasToolCalls() {
		t.Fatal("tool result message should not have tool calls")
	}
}
