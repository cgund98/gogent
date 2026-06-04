package gogent

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
)

type Message struct {
	ID         string      `json:"id"`
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

// NewUserMessage creates a user message with a generated ID.
func NewUserMessage(content string) Message {
	return Message{
		ID:      uuid.New().String(),
		Role:    MessageRoleUser,
		Content: content,
	}
}

// NewAssistantMessage creates a final assistant reply with no tool calls.
func NewAssistantMessage(content string) Message {
	return Message{
		ID:      uuid.New().String(),
		Role:    MessageRoleAssistant,
		Content: content,
	}
}

// NewAssistantMessageWithToolCalls creates an assistant message that requests one or more tools.
func NewAssistantMessageWithToolCalls(content string, toolCalls []ToolCall) Message {
	return Message{
		ID:        uuid.New().String(),
		Role:      MessageRoleAssistant,
		Content:   content,
		ToolCalls: append([]ToolCall(nil), toolCalls...),
	}
}

// NewToolResultMessage creates one tool result message linked to a tool call ID.
func NewToolResultMessage(toolCallID string, content string) Message {
	return Message{
		ID:         uuid.New().String(),
		Role:       MessageRoleTool,
		ToolCallID: toolCallID,
		Content:    content,
	}
}

// HasToolCalls reports whether the message is an assistant turn that requested tools.
func (m *Message) HasToolCalls() bool {
	return m.Role == MessageRoleAssistant && len(m.ToolCalls) > 0
}

// CanExecuteTools reports whether every tool call on the message is approved and pending execution.
func (m *Message) CanExecuteTools() bool {
	if !m.HasToolCalls() {
		return false
	}

	for _, toolCall := range m.ToolCalls {
		if !toolCall.CanExecute() {
			return false
		}
	}

	return true
}

// SetToolCallApproval updates the approval status of a tool call on an assistant message.
func (m *Message) SetToolCallApproval(toolCallID string, status ApprovalStatus) error {
	for i := range m.ToolCalls {
		if m.ToolCalls[i].ID == toolCallID {
			m.ToolCalls[i].ApprovalStatus = status
			return nil
		}
	}

	return fmt.Errorf("tool call %s not found in message %s", toolCallID, m.ID)
}

// AllToolCallsApprovalSettled reports whether no tool calls on the message remain pending approval.
func (m *Message) AllToolCallsApprovalSettled() bool {
	if !m.HasToolCalls() {
		return true
	}

	for _, toolCall := range m.ToolCalls {
		if toolCall.IsPendingApproval() {
			return false
		}
	}

	return true
}

type MessageStore interface {
	Load(ctx context.Context, chatID string) ([]Message, error)
	GetMessage(ctx context.Context, chatID string, messageID string) (Message, error)
	AddMessages(ctx context.Context, chatID string, messages ...Message) error
	UpdateMessage(ctx context.Context, chatID string, messageID string, message Message) error
	DeleteMessage(ctx context.Context, chatID string, messageID string) error
	DeleteAllMessages(ctx context.Context, chatID string) error
}
