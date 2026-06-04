package gogent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Agent struct {
	store         MessageStore
	broadcaster   ChatEventBroadcaster
	model         Model
	toolRegistry  *ToolRegistry
	maxIterations int
}

type PendingToolCall struct {
	MessageID  string
	ToolCallID string
	ToolName   string
	Args       json.RawMessage
}

// NewAgent creates an agent with the given dependencies and iteration limit.
// maxIterations applies to model turns only; tool resolution is not capped.
func NewAgent(store MessageStore, broadcaster ChatEventBroadcaster, model Model, toolRegistry *ToolRegistry, maxIterations int) *Agent {
	return &Agent{
		store:         store,
		broadcaster:   broadcaster,
		model:         model,
		toolRegistry:  toolRegistry,
		maxIterations: maxIterations,
	}
}

// RunWithUserInput appends a user message to the chat and runs the agent loop,
// including a model turn once any pending tool work is complete.
func (a *Agent) RunWithUserInput(ctx context.Context, chatID string, userInput string) error {
	userMessage := NewUserMessage(userInput)
	if err := a.addMessages(ctx, chatID, userMessage); err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	return a.run(ctx, chatID, true)
}

// RunNewChat creates a new chat and runs the agent loop,
// including a model turn once any pending tool work is complete.
func (a *Agent) RunNewChat(ctx context.Context, userInput string) (string, error) {
	chatID := uuid.New().String()
	return chatID, a.RunWithUserInput(ctx, chatID, userInput)
}

// ApproveToolCall marks a tool call as approved. The agent runs only after every
// tool call on the same assistant message is settled (approved or rejected).
func (a *Agent) ApproveToolCall(ctx context.Context, chatID string, messageID string, toolCallID string) error {
	message, err := a.store.GetMessage(ctx, chatID, messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}
	if err := message.SetToolCallApproval(toolCallID, ApprovalStatusApproved); err != nil {
		return err
	}
	if err := a.updateMessage(ctx, chatID, message); err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	if !message.AllToolCallsApprovalSettled() {
		return nil
	}

	return a.run(ctx, chatID, true)
}

// RejectToolCall marks a tool call as rejected, appends a denial tool message,
// and stops without calling the model. User feedback is handled separately via RunWithUserInput.
func (a *Agent) RejectToolCall(ctx context.Context, chatID string, messageID string, toolCallID string) error {
	message, err := a.store.GetMessage(ctx, chatID, messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}
	if err := message.SetToolCallApproval(toolCallID, ApprovalStatusRejected); err != nil {
		return err
	}
	if err := a.updateMessage(ctx, chatID, message); err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return a.processToolTurn(ctx, chatID)
}

func (a *Agent) ListMessages(ctx context.Context, chatID string) ([]Message, error) {
	return a.store.Load(ctx, chatID)
}

// ListPendingToolCalls returns approval-required tool calls on the current
// unresolved assistant turn that still await human approval.
func (a *Agent) ListPendingToolCalls(ctx context.Context, chatID string) ([]PendingToolCall, error) {
	messages, err := a.store.Load(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}

	return listApprovalsRequired(messages, a.toolRegistry), nil
}

// Run continues an existing conversation until the model returns a final answer,
// tool execution pauses for approval, or maxIterations is reached.
func (a *Agent) Run(ctx context.Context, chatID string) error {
	return a.run(ctx, chatID, true)
}

// processToolTurn resolves outstanding tool calls without invoking the model.
func (a *Agent) processToolTurn(ctx context.Context, chatID string) error {
	return a.run(ctx, chatID, false)
}

// run is the core agent loop. When invokeModel is false, tool work is processed
// but GenerateResponse is not called after the tool turn completes.
func (a *Agent) run(ctx context.Context, chatID string, invokeModel bool) error {
	messages, err := a.store.Load(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to load messages: %w", err)
	}

	modelIterations := 0
	for {
		// Find and apply unresolved tool calls
		toolResultMessages, paused, err := a.findAndApplyUnresolvedToolCalls(ctx, chatID, messages)
		if err != nil {
			return err
		}
		messages = append(messages, toolResultMessages...)

		// If we're paused or we don't need to invoke the model, return
		if paused || !invokeModel {
			return nil
		}

		if modelIterations >= a.maxIterations {
			return errors.New("max iterations reached")
		}
		modelIterations++

		newMessage, err := a.model.GenerateResponse(ctx, messages)
		if err != nil {
			return fmt.Errorf("failed to generate response: %w", err)
		}

		if err := a.addMessages(ctx, chatID, newMessage); err != nil {
			return fmt.Errorf("failed to add assistant message: %w", err)
		}
		messages = append(messages, newMessage)

		if !newMessage.HasToolCalls() {
			return nil
		}
	}
}

// updateMessage updates a message in the store and broadcasts an event if the broadcaster is set.
func (a *Agent) updateMessage(ctx context.Context, chatID string, message Message) error {
	if err := a.store.UpdateMessage(ctx, chatID, message.ID, message); err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}
	if a.broadcaster != nil {
		if err := a.broadcaster.Broadcast(ChatEvent{
			ID:        message.ID,
			ChatID:    chatID,
			Type:      ChatEventTypeMessageUpdated,
			MessageID: &message.ID,
		}); err != nil {
			return fmt.Errorf("failed to broadcast message updated event: %w", err)
		}
	}
	return nil
}

func (a *Agent) addMessages(ctx context.Context, chatID string, messages ...Message) error {
	if err := a.store.AddMessages(ctx, chatID, messages...); err != nil {
		return fmt.Errorf("failed to add messages: %w", err)
	}
	for _, message := range messages {
		if a.broadcaster == nil {
			continue
		}
		if err := a.broadcaster.Broadcast(ChatEvent{
			ID:        message.ID,
			ChatID:    chatID,
			Type:      ChatEventTypeMessageAdded,
			MessageID: &message.ID,
		}); err != nil {
			return fmt.Errorf("failed to broadcast message added event: %w", err)
		}
	}
	return nil
}

// findAndApplyUnresolvedToolCalls finds and applies unresolved tool calls in the chat history
// and returns the any new tool result messages, a boolean indicating if the agent is paused, and an error if any.
func (a *Agent) findAndApplyUnresolvedToolCalls(ctx context.Context, chatID string, messages []Message) ([]Message, bool, error) {
	// Collect all new tool result messages into a single slice so we can return them later
	aggregatedToolResultMessages := make([]Message, 0)

	for {
		assistant, assistantIndex, ok := findUnresolvedToolTurn(messages)
		if !ok {
			break
		}

		toolResultMessages, paused, err := a.processToolCalls(ctx, chatID, messages, assistant, assistantIndex)
		if err != nil {
			return aggregatedToolResultMessages, false, err
		}
		if len(toolResultMessages) > 0 {
			if err := a.addMessages(ctx, chatID, toolResultMessages...); err != nil {
				return aggregatedToolResultMessages, false, fmt.Errorf("failed to add tool result messages: %w", err)
			}
			aggregatedToolResultMessages = append(aggregatedToolResultMessages, toolResultMessages...)
			messages = append(messages, toolResultMessages...)
		}
		if paused {
			return aggregatedToolResultMessages, true, nil
		}
	}

	return aggregatedToolResultMessages, false, nil
}
