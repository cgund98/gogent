package openai

import (
	"encoding/json"
	"strings"
	"testing"

	openaisdk "github.com/openai/openai-go"

	"github.com/cgund98/gogent"
)

func TestToOpenAIChatMessagesStripsInternalToolCallFields(t *testing.T) {
	t.Parallel()

	history := []gogent.Message{
		gogent.NewAssistantMessageWithToolCalls("", []gogent.ToolCall{
			{
				ID:              "call_abc",
				ToolName:        "get_weather",
				Args:            json.RawMessage(`{"city":"Paris"}`),
				ApprovalStatus:  gogent.ApprovalStatusApproved,
				ExecutionStatus: gogent.ExecutionStatusCompleted,
				Result:          json.RawMessage(`{"temp_c":18}`),
			},
		}),
	}

	messages, err := toOpenAIChatMessages("", history)
	if err != nil {
		t.Fatalf("toOpenAIChatMessages() error = %v", err)
	}

	payload, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	payloadStr := string(payload)
	if containsAny(payloadStr, "approval", "execution", "approved", "completed", "temp_c") {
		t.Fatalf("serialized payload leaked internal fields: %s", payloadStr)
	}
	if !containsAll(payloadStr, `"call_abc"`, `"get_weather"`, `\"city\":\"Paris\"`) {
		t.Fatalf("serialized payload missing tool call wire fields: %s", payloadStr)
	}
}

func TestToOpenAIChatMessagesToolResult(t *testing.T) {
	t.Parallel()

	history := []gogent.Message{
		gogent.NewToolResultMessage("call_abc", `{"temp_c":18}`),
	}

	messages, err := toOpenAIChatMessages("", history)
	if err != nil {
		t.Fatalf("toOpenAIChatMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Role != "tool" {
		t.Fatalf("Role = %q, want tool", messages[0].Role)
	}
	if messages[0].ToolCallID != "call_abc" {
		t.Fatalf("ToolCallID = %q, want call_abc", messages[0].ToolCallID)
	}
}

func TestFromOpenAIAssistantMessageDefaultsPendingStatuses(t *testing.T) {
	t.Parallel()

	message, err := fromOpenAIAssistantMessage(openAIChatMessage{
		Role: "assistant",
		ToolCalls: []openAIToolCall{
			{
				ID:   "call_abc",
				Type: "function",
				Function: openAIFunctionCall{
					Name:      "get_weather",
					Arguments: `{"city":"Paris"}`,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("fromOpenAIAssistantMessage() error = %v", err)
	}

	if len(message.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(message.ToolCalls))
	}

	toolCall := message.ToolCalls[0]
	if toolCall.ApprovalStatus != gogent.ApprovalStatusPending {
		t.Fatalf("ApprovalStatus = %q, want pending", toolCall.ApprovalStatus)
	}
	if toolCall.ExecutionStatus != gogent.ExecutionStatusPending {
		t.Fatalf("ExecutionStatus = %q, want pending", toolCall.ExecutionStatus)
	}
}

func containsAny(value string, parts ...string) bool {
	for _, part := range parts {
		if contains(value, part) {
			return true
		}
	}
	return false
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !contains(value, part) {
			return false
		}
	}
	return true
}

func contains(value, part string) bool {
	return strings.Contains(value, part)
}

func TestFromChatCompletionMessageText(t *testing.T) {
	t.Parallel()

	message, err := fromChatCompletionMessage(openaisdk.ChatCompletionMessage{
		Role:    "assistant",
		Content: "Hello there",
	})
	if err != nil {
		t.Fatalf("fromChatCompletionMessage() error = %v", err)
	}
	if message.Role != gogent.MessageRoleAssistant {
		t.Fatalf("Role = %q, want assistant", message.Role)
	}
	if message.Content != "Hello there" {
		t.Fatalf("Content = %q, want Hello there", message.Content)
	}
}

func TestFromChatCompletionMessageToolCalls(t *testing.T) {
	t.Parallel()

	message, err := fromChatCompletionMessage(openaisdk.ChatCompletionMessage{
		Role: "assistant",
		ToolCalls: []openaisdk.ChatCompletionMessageToolCall{
			{
				ID:   "call_abc",
				Type: "function",
				Function: openaisdk.ChatCompletionMessageToolCallFunction{
					Name:      "get_weather",
					Arguments: `{"city":"Paris"}`,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("fromChatCompletionMessage() error = %v", err)
	}
	if !message.HasToolCalls() {
		t.Fatal("expected tool calls")
	}
	if message.ToolCalls[0].ApprovalStatus != gogent.ApprovalStatusPending {
		t.Fatalf("ApprovalStatus = %q, want pending", message.ToolCalls[0].ApprovalStatus)
	}
}
