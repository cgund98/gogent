package gogent

// listApprovalsRequired returns approval-required, unresolved tool calls on the
// current assistant turn, in model tool-call order.
func listApprovalsRequired(messages []Message, registry *ToolRegistry) []PendingToolCall {
	assistant, assistantIndex, ok := findUnresolvedToolTurn(messages)
	if !ok {
		return nil
	}

	resolved := resolvedToolCallIDsForTurn(messages, assistantIndex)
	out := make([]PendingToolCall, 0, len(assistant.ToolCalls))
	for _, toolCall := range assistant.ToolCalls {
		if _, ok := resolved[toolCall.ID]; ok {
			continue
		}
		if !toolCall.IsPendingApproval() {
			continue
		}
		if registry == nil {
			continue
		}
		tool := registry.GetTool(toolCall.ToolName)
		if tool == nil || !tool.RequiresApproval() {
			continue
		}
		out = append(out, PendingToolCall{
			MessageID:  assistant.ID,
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.ToolName,
			Args:       toolCall.Args,
		})
	}
	return out
}

// findUnresolvedToolTurn returns the most recent assistant message that still
// has tool calls without matching tool result messages after that turn.
func findUnresolvedToolTurn(messages []Message) (Message, int, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if !messages[i].HasToolCalls() {
			continue
		}
		if allToolCallsResolved(messages, i) {
			continue
		}
		return messages[i], i, true
	}

	return Message{}, -1, false
}

// allToolCallsResolved reports whether every tool call on the assistant message
// has a tool result message after that turn keyed by tool call ID.
func allToolCallsResolved(messages []Message, assistantIndex int) bool {
	if assistantIndex < 0 || assistantIndex >= len(messages) {
		return true
	}

	resolved := resolvedToolCallIDsForTurn(messages, assistantIndex)
	assistant := messages[assistantIndex]
	for _, toolCall := range assistant.ToolCalls {
		if _, ok := resolved[toolCall.ID]; !ok {
			return false
		}
	}

	return true
}

// resolvedToolCallIDsForTurn returns tool call IDs on the assistant message that
// already have a matching tool result message later in the transcript.
func resolvedToolCallIDsForTurn(messages []Message, assistantIndex int) map[string]struct{} {
	if assistantIndex < 0 || assistantIndex >= len(messages) {
		return map[string]struct{}{}
	}

	assistant := messages[assistantIndex]
	pending := make(map[string]struct{}, len(assistant.ToolCalls))
	for _, toolCall := range assistant.ToolCalls {
		pending[toolCall.ID] = struct{}{}
	}

	resolved := make(map[string]struct{}, len(pending))
	for _, message := range messages[assistantIndex+1:] {
		if message.Role != MessageRoleTool || message.ToolCallID == "" {
			continue
		}
		if _, ok := pending[message.ToolCallID]; ok {
			resolved[message.ToolCallID] = struct{}{}
		}
	}

	return resolved
}
