package gogent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

type ApprovalStatus string
type ExecutionStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
)

// ToolCallRejectedContent is the JSON payload appended to the transcript when a tool call is rejected.
const ToolCallRejectedContent = `{"error":"rejected","message":"Tool call rejected by user."}`

// ToolCallNotFoundContent returns a JSON tool result when the tool name is not registered.
func ToolCallNotFoundContent(toolName string) string {
	return toolCallErrorPayload("not_found", fmt.Sprintf("tool %q is not registered", toolName), toolName)
}

// ToolCallExecutionErrorContent returns a JSON tool result when tool execution fails.
func ToolCallExecutionErrorContent(toolName string, err error) string {
	message := "tool execution failed"
	if err != nil {
		message = err.Error()
	}
	return toolCallErrorPayload("execution_failed", message, toolName)
}

func toolCallErrorPayload(code, message, toolName string) string {
	payload, err := json.Marshal(map[string]string{
		"error":   code,
		"message": message,
		"tool":    toolName,
	})
	if err != nil {
		return fmt.Sprintf(`{"error":%q,"message":%q,"tool":%q}`, code, message, toolName)
	}
	return string(payload)
}

type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	RequiresApproval() bool
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}

type ToolCall struct {
	ID              string
	ToolName        string
	Args            json.RawMessage
	ApprovalStatus  ApprovalStatus
	ExecutionStatus ExecutionStatus
	Result          json.RawMessage
}

// IsApproved reports whether the tool call has been approved for execution.
func (t *ToolCall) IsApproved() bool {
	return t.ApprovalStatus == ApprovalStatusApproved
}

// IsRejected reports whether the tool call was rejected by a reviewer.
func (t *ToolCall) IsRejected() bool {
	return t.ApprovalStatus == ApprovalStatusRejected
}

// IsPendingApproval reports whether the tool call still awaits human approval.
func (t *ToolCall) IsPendingApproval() bool {
	return t.ApprovalStatus == ApprovalStatusPending || t.ApprovalStatus == ""
}

// NewPendingToolCall creates a tool call parsed from a model response with pending approval and execution status.
func NewPendingToolCall(id, toolName string, args json.RawMessage) ToolCall {
	return ToolCall{
		ID:              id,
		ToolName:        toolName,
		Args:            args,
		ApprovalStatus:  ApprovalStatusPending,
		ExecutionStatus: ExecutionStatusPending,
	}
}

// CanExecute reports whether the tool call is approved and ready to run.
func (t *ToolCall) CanExecute() bool {
	switch t.ExecutionStatus {
	case ExecutionStatusPending, ExecutionStatusRunning:
		return t.IsApproved()
	default:
		return false
	}
}

type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates an empty registry of tools available to an agent.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// RegisterTool adds a tool to the registry keyed by its name.
// Returns an error if a tool with the same name is already registered.
func (r *ToolRegistry) RegisterTool(tool Tool) error {
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = tool
	return nil
}

// GetTool returns the registered tool with the given name, or nil if none exists.
func (r *ToolRegistry) GetTool(name string) Tool {
	if _, exists := r.tools[name]; !exists {
		return nil
	}
	return r.tools[name]
}

// Tools returns all registered tools in name order.
func (r *ToolRegistry) Tools() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name() < tools[j].Name()
	})
	return tools
}
