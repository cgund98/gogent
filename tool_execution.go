package gogent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// toolCallOutcome is the settled state of one tool call on a turn after the
// decide phase, and its terminal state after the execute phase.
type toolCallOutcome struct {
	toolCall       ToolCall
	tool           Tool     // resolved tool, nil when the call is not runnable
	needsExecution bool     // approved and ready to run in the execute phase
	resultMessage  *Message // non-nil once settled (result, reject, error, or restore)
}

// processToolCalls executes, rejects, or skips each unresolved tool call on the
// given assistant message (matched by message ID and tool call ID). It returns
// tool result messages to append, and paused=true when any call still awaits approval.
//
// It runs in three phases: decide (sequential, order-preserving), execute
// (concurrent when the turn is fully executable), and assemble (sequential, so
// tool messages keep model tool-call order).
func (a *Agent) processToolCalls(ctx context.Context, chatID string, messages []Message, assistant Message, assistantIndex int) ([]Message, bool, error) {
	resolved := resolvedToolCallIDsForTurn(messages, assistantIndex)
	updatedToolCalls := append([]ToolCall(nil), assistant.ToolCalls...)

	outcomes := make([]toolCallOutcome, len(updatedToolCalls))
	runnable := make([]int, 0, len(updatedToolCalls))
	paused := false

	// Phase A — decide. Walk the unresolved calls in model order and record one
	// outcome each. Approval is evaluated sequentially for deterministic behavior.
	for i := range updatedToolCalls {
		outcome := toolCallOutcome{toolCall: updatedToolCalls[i]}

		if _, ok := resolved[outcome.toolCall.ID]; ok {
			outcomes[i] = outcome
			continue
		}

		// A call that already reached a terminal outcome in an earlier pass needs
		// only its tool message restored. Re-running it would repeat side effects.
		if outcome.toolCall.ExecutionStatus == ExecutionStatusCompleted || outcome.toolCall.ExecutionStatus == ExecutionStatusFailed {
			if len(outcome.toolCall.Result) > 0 {
				msg := NewToolResultMessage(outcome.toolCall.ID, string(outcome.toolCall.Result))
				outcome.resultMessage = &msg
				outcomes[i] = outcome
				continue
			}
			outcome.toolCall.ExecutionStatus = ExecutionStatusPending
		}

		if outcome.toolCall.IsRejected() {
			outcome.toolCall = failToolCall(outcome.toolCall, ToolCallRejectedContent)
			msg := NewToolResultMessage(outcome.toolCall.ID, ToolCallRejectedContent)
			outcome.resultMessage = &msg
			outcomes[i] = outcome
			continue
		}

		var tool Tool
		if a.toolRegistry != nil {
			tool = a.toolRegistry.GetTool(outcome.toolCall.ToolName)
		}
		if tool == nil {
			content := ToolCallNotFoundContent(outcome.toolCall.ToolName)
			outcome.toolCall = failToolCall(outcome.toolCall, content)
			msg := NewToolResultMessage(outcome.toolCall.ID, content)
			outcome.resultMessage = &msg
			outcomes[i] = outcome
			continue
		}

		if outcome.toolCall.IsPendingApproval() {
			decision, err := tool.RequiresApproval(ctx, outcome.toolCall.Args)
			if err != nil {
				content := ToolCallExecutionErrorContent(outcome.toolCall.ToolName, err)
				outcome.toolCall = failToolCall(outcome.toolCall, content)
				msg := NewToolResultMessage(outcome.toolCall.ID, content)
				outcome.resultMessage = &msg
				outcomes[i] = outcome
				continue
			}
			if decision.Required {
				outcome.toolCall.Reason = decision.Reason
				if outcome.toolCall.Reason == "" {
					outcome.toolCall.Reason = "approval required"
				}
				paused = true
				outcomes[i] = outcome
				continue
			}
		}

		if !outcome.toolCall.IsApproved() {
			outcome.toolCall.ApprovalStatus = ApprovalStatusApproved
		}

		outcome.toolCall.ExecutionStatus = ExecutionStatusRunning
		outcome.tool = tool
		outcome.needsExecution = true
		outcomes[i] = outcome
		runnable = append(runnable, i)
	}

	// Phase B — execute. Concurrency is allowed only when the whole turn can run:
	// no call awaits approval, there is more than one runnable call, and the limit
	// permits it. Otherwise fall back to the sequential path.
	if paused || len(runnable) < 2 || a.effectiveMaxConcurrentTools() < 2 {
		for _, i := range runnable {
			outcomes[i] = runToolCall(ctx, outcomes[i])
		}
	} else {
		a.executeToolCallsConcurrently(ctx, runnable, outcomes)
	}

	// Phase C — assemble in original order so tool messages stay in model tool-call order.
	toolResultMessages := make([]Message, 0, len(outcomes))
	for i := range outcomes {
		updatedToolCalls[i] = outcomes[i].toolCall
		if outcomes[i].resultMessage != nil {
			toolResultMessages = append(toolResultMessages, *outcomes[i].resultMessage)
		}
	}

	assistant.ToolCalls = updatedToolCalls
	if err := a.updateMessage(ctx, chatID, assistant); err != nil {
		return nil, false, fmt.Errorf("failed to update assistant message: %w", err)
	}

	return toolResultMessages, paused, nil
}

// runToolCall executes one already-approved call and records its terminal state.
// A failure is written into the outcome as a failed call plus an execution_failed
// tool message. It is never returned to the caller and never cancels siblings.
func runToolCall(ctx context.Context, outcome toolCallOutcome) toolCallOutcome {
	result, err := ExecuteTool(ctx, outcome.tool, outcome.toolCall)
	if err != nil {
		content := ToolCallExecutionErrorContent(outcome.toolCall.ToolName, err)
		outcome.toolCall = failToolCall(outcome.toolCall, content)
		msg := NewToolResultMessage(outcome.toolCall.ID, content)
		outcome.resultMessage = &msg
		return outcome
	}

	outcome.toolCall.Result = result
	outcome.toolCall.ExecutionStatus = ExecutionStatusCompleted
	msg := NewToolResultMessage(outcome.toolCall.ID, string(result))
	outcome.resultMessage = &msg
	return outcome
}

// executeToolCallsConcurrently runs the runnable calls up to the configured limit.
//
// Failure isolation is deliberate: it does not use errgroup and does not derive a
// cancelable context, so one call failing never cancels its siblings. runToolCall
// records each failure in its own outcome instead of returning it, and wg.Wait
// blocks until every call has reached a terminal outcome.
func (a *Agent) executeToolCallsConcurrently(ctx context.Context, runnable []int, outcomes []toolCallOutcome) {
	limit := a.effectiveMaxConcurrentTools()
	if limit > len(runnable) {
		limit = len(runnable)
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, i := range runnable {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			outcomes[i] = runToolCall(ctx, outcomes[i])
		}(i)
	}
	wg.Wait()
}

// failToolCall records a terminal failure for one call. A call that still
// awaited approval is settled as approved, because a failed call can never need
// that approval and leaving it pending would strand the whole turn: no tool
// message is missing for it, but AllToolCallsApprovalSettled would stay false
// and ApproveToolCall would never resume the run. A rejected call keeps its
// rejected status.
func failToolCall(toolCall ToolCall, content string) ToolCall {
	toolCall.ExecutionStatus = ExecutionStatusFailed
	toolCall.Result = json.RawMessage(content)
	if toolCall.IsPendingApproval() {
		toolCall.ApprovalStatus = ApprovalStatusApproved
	}
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
