package gogent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// processToolCalls executes, rejects, or skips each unresolved tool call on the
// given assistant message (matched by message ID and tool call ID). It returns
// tool result messages to append, and paused=true when any call still awaits approval.
func (a *Agent) processToolCalls(ctx context.Context, chatID string, messages []Message, assistant Message, assistantIndex int) ([]Message, bool, error) {
	resolved := resolvedToolCallIDsForTurn(messages, assistantIndex)
	updatedToolCalls := append([]ToolCall(nil), assistant.ToolCalls...)

	// Collect all new tool result messages into a single slice so we can return them later
	toolResultMessages := make([]Message, 0, len(updatedToolCalls))
	paused := false

	for i := range updatedToolCalls {
		toolCall := updatedToolCalls[i]

		if _, ok := resolved[toolCall.ID]; ok {
			continue
		}

		if toolCall.ExecutionStatus == ExecutionStatusCompleted {
			if len(toolCall.Result) > 0 {
				toolResultMessages = append(
					toolResultMessages,
					NewToolResultMessage(toolCall.ID, string(toolCall.Result)),
				)
				continue
			}
			toolCall.ExecutionStatus = ExecutionStatusPending
		}

		if toolCall.IsRejected() {
			toolCall = failToolCall(toolCall, ToolCallRejectedContent)
			updatedToolCalls[i] = toolCall
			toolResultMessages = append(toolResultMessages, NewToolResultMessage(toolCall.ID, ToolCallRejectedContent))
			continue
		}

		var tool Tool
		if a.toolRegistry != nil {
			tool = a.toolRegistry.GetTool(toolCall.ToolName)
		}
		if tool == nil {
			content := ToolCallNotFoundContent(toolCall.ToolName)
			toolCall = failToolCall(toolCall, content)
			updatedToolCalls[i] = toolCall
			toolResultMessages = append(toolResultMessages, NewToolResultMessage(toolCall.ID, content))
			continue
		}

		if tool.RequiresApproval() && toolCall.IsPendingApproval() {
			paused = true
			continue
		}

		if !toolCall.IsApproved() {
			toolCall.ApprovalStatus = ApprovalStatusApproved
		}

		toolCall.ExecutionStatus = ExecutionStatusRunning
		updatedToolCalls[i] = toolCall

		result, err := ExecuteTool(ctx, tool, toolCall)
		if err != nil {
			content := ToolCallExecutionErrorContent(toolCall.ToolName, err)
			toolCall = failToolCall(toolCall, content)
			updatedToolCalls[i] = toolCall
			toolResultMessages = append(toolResultMessages, NewToolResultMessage(toolCall.ID, content))
			continue
		}

		toolCall.Result = result
		toolCall.ExecutionStatus = ExecutionStatusCompleted
		updatedToolCalls[i] = toolCall
		toolResultMessages = append(toolResultMessages, NewToolResultMessage(toolCall.ID, string(result)))
	}

	assistant.ToolCalls = updatedToolCalls
	if err := a.updateMessage(ctx, chatID, assistant); err != nil {
		return nil, false, fmt.Errorf("failed to update assistant message: %w", err)
	}

	return toolResultMessages, paused, nil
}

func failToolCall(toolCall ToolCall, content string) ToolCall {
	toolCall.ExecutionStatus = ExecutionStatusFailed
	toolCall.Result = json.RawMessage(content)
	return toolCall
}

// ExecuteTool runs a single approved tool call and returns its JSON result.
func ExecuteTool(ctx context.Context, tool Tool, toolCall ToolCall) (json.RawMessage, error) {
	if !toolCall.CanExecute() {
		return nil, errors.New("tool call cannot be executed")
	}

	result, err := tool.Execute(ctx, toolCall.Args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool %s: %w", toolCall.ToolName, err)
	}

	return result, nil
}
