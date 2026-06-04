package gogent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type stubModel struct {
	responses []Message
	calls     int
}

func (m *stubModel) GenerateResponse(_ context.Context, _ []Message) (Message, error) {
	m.calls++
	if len(m.responses) == 0 {
		return Message{}, nil
	}
	response := m.responses[0]
	m.responses = m.responses[1:]
	return response, nil
}

type memoryStore struct {
	messages map[string][]Message
}

func newMemoryStore() *memoryStore {
	return &memoryStore{messages: make(map[string][]Message)}
}

func (s *memoryStore) Load(_ context.Context, chatID string) ([]Message, error) {
	return append([]Message(nil), s.messages[chatID]...), nil
}

func (s *memoryStore) GetMessage(_ context.Context, chatID string, messageID string) (Message, error) {
	for _, message := range s.messages[chatID] {
		if message.ID == messageID {
			return message, nil
		}
	}
	return Message{}, fmt.Errorf("message %s not found", messageID)
}

func (s *memoryStore) AddMessages(_ context.Context, chatID string, messages ...Message) error {
	s.messages[chatID] = append(s.messages[chatID], messages...)
	return nil
}

func (s *memoryStore) UpdateMessage(_ context.Context, chatID string, messageID string, message Message) error {
	for i, existing := range s.messages[chatID] {
		if existing.ID == messageID {
			s.messages[chatID][i] = message
			return nil
		}
	}
	return nil
}

func (s *memoryStore) DeleteMessage(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *memoryStore) DeleteAllMessages(_ context.Context, chatID string) error {
	delete(s.messages, chatID)
	return nil
}

type stubTool struct {
	name             string
	requiresApproval bool
	result           json.RawMessage
}

func (t stubTool) Name() string { return t.name }
func (t stubTool) Description() string {
	return "stub tool"
}
func (t stubTool) Parameters() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (t stubTool) RequiresApproval() bool      { return t.requiresApproval }
func (t stubTool) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return t.result, nil
}

type trackingStubTool struct {
	stubTool
	executeCount *int
}

func (t trackingStubTool) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	*t.executeCount++
	return t.result, nil
}

func mustRegisterTool(t *testing.T, registry *ToolRegistry, tool Tool) {
	t.Helper()
	if err := registry.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}
}

func toolResultMessages(messages []Message) []Message {
	toolMessages := make([]Message, 0)
	for _, message := range messages {
		if message.Role == MessageRoleTool {
			toolMessages = append(toolMessages, message)
		}
	}
	return toolMessages
}

func assistantMessageByID(messages []Message, messageID string) (Message, bool) {
	for _, message := range messages {
		if message.ID == messageID {
			return message, true
		}
	}
	return Message{}, false
}

func TestRejectToolCallAppendsToolMessageWithoutModelCall(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	model := &stubModel{}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, stubTool{name: "delete_user", requiresApproval: true, result: json.RawMessage(`{"deleted":true}`)})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "delete_user",
			Args:            json.RawMessage(`{"id":"123"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})

	if err := store.AddMessages(context.Background(), "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.RejectToolCall(context.Background(), "chat-1", assistantMessage.ID, "call_a"); err != nil {
		t.Fatalf("RejectToolCall() error = %v", err)
	}

	if model.calls != 0 {
		t.Fatalf("GenerateResponse calls = %d, want 0 after rejection", model.calls)
	}

	messages, err := store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var toolMessage Message
	for _, message := range messages {
		if message.Role == MessageRoleTool && message.ToolCallID == "call_a" {
			toolMessage = message
			break
		}
	}
	if toolMessage.ID == "" {
		t.Fatal("expected tool rejection message for call_a")
	}
	if toolMessage.Content != ToolCallRejectedContent {
		t.Fatalf("Content = %q, want rejection payload", toolMessage.Content)
	}
}

func TestRunWithUserInputAfterRejectionCallsModel(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("Understood, I won't delete the user."),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, stubTool{name: "delete_user", requiresApproval: true})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "delete_user",
			Args:            json.RawMessage(`{"id":"123"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(context.Background(), "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.RejectToolCall(context.Background(), "chat-1", assistantMessage.ID, "call_a"); err != nil {
		t.Fatalf("RejectToolCall() error = %v", err)
	}

	if err := agent.RunWithUserInput(context.Background(), "chat-1", "Please suggest an alternative instead."); err != nil {
		t.Fatalf("RunWithUserInput() error = %v", err)
	}

	if model.calls != 1 {
		t.Fatalf("GenerateResponse calls = %d, want 1 after user feedback", model.calls)
	}

	messages, err := store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if messages[len(messages)-2].Role != MessageRoleUser {
		t.Fatalf("second-to-last message role = %q, want user", messages[len(messages)-2].Role)
	}
}

func TestAgentWaitsForAllToolResultsBeforeModelCall(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, stubTool{name: "get_weather", result: json.RawMessage(`{"temp_c":18}`)})

	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("done"),
		},
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := store.AddMessages(context.Background(), "chat-1", NewAssistantMessageWithToolCalls("", []ToolCall{
		{ID: "call_a", ToolName: "get_weather", Args: json.RawMessage(`{"city":"Paris"}`), ApprovalStatus: ApprovalStatusApproved, ExecutionStatus: ExecutionStatusPending},
		{ID: "call_b", ToolName: "get_weather", Args: json.RawMessage(`{"city":"London"}`), ApprovalStatus: ApprovalStatusApproved, ExecutionStatus: ExecutionStatusPending},
	})); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	if err := agent.Run(context.Background(), "chat-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if model.calls != 1 {
		t.Fatalf("GenerateResponse calls = %d, want 1 after tool results", model.calls)
	}

	messages, err := store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(messages) != 4 {
		t.Fatalf("len(messages) = %d, want 4 (assistant + 2 tool results + final assistant)", len(messages))
	}
}

func TestFindUnresolvedToolTurn(t *testing.T) {
	t.Parallel()

	messages := []Message{
		NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_a"}}),
		NewToolResultMessage("call_a", "ok"),
		NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_b"}}),
	}

	assistant, assistantIndex, ok := findUnresolvedToolTurn(messages)
	if !ok {
		t.Fatal("findUnresolvedToolTurn() = false, want true")
	}
	if assistantIndex != 2 {
		t.Fatalf("assistantIndex = %d, want 2", assistantIndex)
	}
	if assistant.ID != messages[2].ID {
		t.Fatalf("assistant ID = %q, want message at index 2 (%q)", assistant.ID, messages[2].ID)
	}
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].ID != "call_b" {
		t.Fatalf("assistant tool calls = %+v, want unresolved call_b", assistant.ToolCalls)
	}
}

func TestApproveToolCallRunsWhenAllToolsSettled(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("Both lookups complete."),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, stubTool{name: "get_weather", requiresApproval: true, result: json.RawMessage(`{"temp_c":18}`)})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "get_weather",
			Args:            json.RawMessage(`{"city":"Paris"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
		{
			ID:              "call_b",
			ToolName:        "get_weather",
			Args:            json.RawMessage(`{"city":"London"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(context.Background(), "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.ApproveToolCall(context.Background(), "chat-1", assistantMessage.ID, "call_a"); err != nil {
		t.Fatalf("ApproveToolCall(call_a) error = %v", err)
	}
	if model.calls != 0 {
		t.Fatalf("GenerateResponse calls = %d after first approval, want 0", model.calls)
	}

	messages, err := store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d after first approval, want 1", len(messages))
	}

	if err := agent.ApproveToolCall(context.Background(), "chat-1", assistantMessage.ID, "call_b"); err != nil {
		t.Fatalf("ApproveToolCall(call_b) error = %v", err)
	}
	if model.calls != 1 {
		t.Fatalf("GenerateResponse calls = %d after all approvals, want 1", model.calls)
	}

	messages, err = store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("len(messages) = %d, want 4 (assistant + 2 tool results + final assistant)", len(messages))
	}
}

func TestSetToolCallApproval(t *testing.T) {
	t.Parallel()

	message := NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_a", ApprovalStatus: ApprovalStatusPending}})
	if err := message.SetToolCallApproval("call_a", ApprovalStatusRejected); err != nil {
		t.Fatalf("SetToolCallApproval() error = %v", err)
	}
	if message.ToolCalls[0].ApprovalStatus != ApprovalStatusRejected {
		t.Fatalf("ApprovalStatus = %q, want rejected", message.ToolCalls[0].ApprovalStatus)
	}
}

func TestRunDoesNotExecuteToolPendingApproval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	executeCount := 0

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("should not reach model"),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, trackingStubTool{
		stubTool: stubTool{
			name:             "delete_user",
			requiresApproval: true,
			result:           json.RawMessage(`{"deleted":true}`),
		},
		executeCount: &executeCount,
	})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "delete_user",
			Args:            json.RawMessage(`{"id":"123"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(ctx, "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.Run(ctx, "chat-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if executeCount != 0 {
		t.Fatalf("Execute() calls = %d, want 0 while approval is pending", executeCount)
	}
	if model.calls != 0 {
		t.Fatalf("GenerateResponse calls = %d, want 0 while paused for approval", model.calls)
	}

	messages, err := store.Load(ctx, "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(toolResultMessages(messages)) != 0 {
		t.Fatalf("tool result messages = %d, want 0 before approval", len(toolResultMessages(messages)))
	}

	updatedAssistant, ok := assistantMessageByID(messages, assistantMessage.ID)
	if !ok {
		t.Fatal("assistant message not found after Run()")
	}
	if updatedAssistant.ToolCalls[0].ExecutionStatus != ExecutionStatusPending {
		t.Fatalf("ExecutionStatus = %q, want pending", updatedAssistant.ToolCalls[0].ExecutionStatus)
	}
}

func TestRunWithUserInputDoesNotExecuteToolPendingApproval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	executeCount := 0

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessageWithToolCalls("", []ToolCall{
				{
					ID:              "call_a",
					ToolName:        "delete_user",
					Args:            json.RawMessage(`{"id":"123"}`),
					ApprovalStatus:  ApprovalStatusPending,
					ExecutionStatus: ExecutionStatusPending,
				},
			}),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, trackingStubTool{
		stubTool: stubTool{
			name:             "delete_user",
			requiresApproval: true,
			result:           json.RawMessage(`{"deleted":true}`),
		},
		executeCount: &executeCount,
	})

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.RunWithUserInput(ctx, "chat-1", "Delete user 123"); err != nil {
		t.Fatalf("RunWithUserInput() error = %v", err)
	}
	if executeCount != 0 {
		t.Fatalf("Execute() calls = %d, want 0 while approval is pending", executeCount)
	}
	if model.calls != 1 {
		t.Fatalf("GenerateResponse calls = %d, want 1 for initial model turn", model.calls)
	}

	messages, err := store.Load(ctx, "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(toolResultMessages(messages)) != 0 {
		t.Fatalf("tool result messages = %d, want 0 before approval", len(toolResultMessages(messages)))
	}
}

func TestRepeatedRunDoesNotExecuteToolPendingApproval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	executeCount := 0

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("should not reach model"),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, trackingStubTool{
		stubTool: stubTool{
			name:             "delete_user",
			requiresApproval: true,
			result:           json.RawMessage(`{"deleted":true}`),
		},
		executeCount: &executeCount,
	})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "delete_user",
			Args:            json.RawMessage(`{"id":"123"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(ctx, "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	for i := range 3 {
		if err := agent.Run(ctx, "chat-1"); err != nil {
			t.Fatalf("Run() iteration %d error = %v", i+1, err)
		}
	}

	if executeCount != 0 {
		t.Fatalf("Execute() calls = %d, want 0 after repeated runs while approval is pending", executeCount)
	}
	if model.calls != 0 {
		t.Fatalf("GenerateResponse calls = %d, want 0 while paused for approval", model.calls)
	}
}

func TestRunExecutesApprovedToolsButNotPendingApprovalTools(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	approvedExecuteCount := 0
	pendingExecuteCount := 0

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("should not reach model"),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, trackingStubTool{
		stubTool: stubTool{
			name:             "delete_user",
			requiresApproval: true,
			result:           json.RawMessage(`{"deleted":true}`),
		},
		executeCount: &approvedExecuteCount,
	})
	mustRegisterTool(t, registry, trackingStubTool{
		stubTool: stubTool{
			name:             "archive_user",
			requiresApproval: true,
			result:           json.RawMessage(`{"archived":true}`),
		},
		executeCount: &pendingExecuteCount,
	})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "delete_user",
			Args:            json.RawMessage(`{"id":"123"}`),
			ApprovalStatus:  ApprovalStatusApproved,
			ExecutionStatus: ExecutionStatusPending,
		},
		{
			ID:              "call_b",
			ToolName:        "archive_user",
			Args:            json.RawMessage(`{"id":"456"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(ctx, "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.Run(ctx, "chat-1"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if approvedExecuteCount != 1 {
		t.Fatalf("approved tool Execute() calls = %d, want 1", approvedExecuteCount)
	}
	if pendingExecuteCount != 0 {
		t.Fatalf("pending approval tool Execute() calls = %d, want 0", pendingExecuteCount)
	}
	if model.calls != 0 {
		t.Fatalf("GenerateResponse calls = %d, want 0 while one tool still awaits approval", model.calls)
	}

	messages, err := store.Load(ctx, "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(toolResultMessages(messages)) != 1 {
		t.Fatalf("tool result messages = %d, want 1 for settled tool while paused for approval", len(toolResultMessages(messages)))
	}

	updatedAssistant, ok := assistantMessageByID(messages, assistantMessage.ID)
	if !ok {
		t.Fatal("assistant message not found after Run()")
	}
	if updatedAssistant.ToolCalls[0].ExecutionStatus != ExecutionStatusCompleted {
		t.Fatalf("approved tool ExecutionStatus = %q, want completed", updatedAssistant.ToolCalls[0].ExecutionStatus)
	}
	if updatedAssistant.ToolCalls[1].ExecutionStatus != ExecutionStatusPending {
		t.Fatalf("pending approval tool ExecutionStatus = %q, want pending", updatedAssistant.ToolCalls[1].ExecutionStatus)
	}
}

func TestRunExecutesApprovalRequiredToolAfterApproval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	executeCount := 0

	store := newMemoryStore()
	model := &stubModel{
		responses: []Message{
			NewAssistantMessage("Deletion complete."),
		},
	}
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, trackingStubTool{
		stubTool: stubTool{
			name:             "delete_user",
			requiresApproval: true,
			result:           json.RawMessage(`{"deleted":true}`),
		},
		executeCount: &executeCount,
	})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "delete_user",
			Args:            json.RawMessage(`{"id":"123"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(ctx, "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         model,
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.Run(ctx, "chat-1"); err != nil {
		t.Fatalf("Run() before approval error = %v", err)
	}
	if executeCount != 0 {
		t.Fatalf("Execute() calls before approval = %d, want 0", executeCount)
	}

	if err := agent.ApproveToolCall(ctx, "chat-1", assistantMessage.ID, "call_a"); err != nil {
		t.Fatalf("ApproveToolCall() error = %v", err)
	}
	if executeCount != 1 {
		t.Fatalf("Execute() calls after approval = %d, want 1", executeCount)
	}
	if model.calls != 1 {
		t.Fatalf("GenerateResponse calls after approval = %d, want 1", model.calls)
	}

	messages, err := store.Load(ctx, "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(toolResultMessages(messages)) != 1 {
		t.Fatalf("tool result messages = %d, want 1 after approval", len(toolResultMessages(messages)))
	}
}

func TestFindUnresolvedToolTurnSkipsResolvedNewerTurn(t *testing.T) {
	t.Parallel()

	messages := []Message{
		NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_old", ToolName: "get_weather"}}),
		NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_new", ToolName: "get_weather"}}),
		NewToolResultMessage("call_new", `{"temp_c":12}`),
	}

	assistant, assistantIndex, ok := findUnresolvedToolTurn(messages)
	if !ok {
		t.Fatal("findUnresolvedToolTurn() = false, want true for older unresolved turn")
	}
	if assistantIndex != 0 {
		t.Fatalf("assistantIndex = %d, want 0 (older turn)", assistantIndex)
	}
	if assistant.ToolCalls[0].ID != "call_old" {
		t.Fatalf("assistant tool calls = %+v, want unresolved call_old", assistant.ToolCalls)
	}
}

func TestListPendingToolCallsOnlyApprovalRequiredOnCurrentTurn(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, stubTool{name: "get_weather", requiresApproval: true})
	mustRegisterTool(t, registry, stubTool{name: "addition", requiresApproval: false})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_auto",
			ToolName:        "addition",
			Args:            json.RawMessage(`{"a":1,"b":2}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
		{
			ID:              "call_approval",
			ToolName:        "get_weather",
			Args:            json.RawMessage(`{"city":"Paris"}`),
			ApprovalStatus:  ApprovalStatusPending,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(context.Background(), "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         &stubModel{},
		toolRegistry:  registry,
		maxIterations: 3,
	}

	pending, err := agent.ListPendingToolCalls(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("ListPendingToolCalls() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("len(pending) = %d, want 1", len(pending))
	}
	if pending[0].ToolCallID != "call_approval" {
		t.Fatalf("ToolCallID = %q, want call_approval", pending[0].ToolCallID)
	}
}

type failingStubTool struct {
	stubTool
}

func (t failingStubTool) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("boom")
}

func TestRunAppendsToolMessageOnExecutionFailure(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	registry := NewToolRegistry()
	mustRegisterTool(t, registry, failingStubTool{stubTool: stubTool{name: "broken_tool"}})

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "broken_tool",
			Args:            json.RawMessage(`{}`),
			ApprovalStatus:  ApprovalStatusApproved,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(context.Background(), "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         &stubModel{responses: []Message{NewAssistantMessage("done")}},
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.Run(context.Background(), "chat-1"); err != nil {
		t.Fatalf("Run() error = %v, want nil with tool error transcript", err)
	}

	messages, err := store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(toolResultMessages(messages)) != 1 {
		t.Fatalf("tool result messages = %d, want 1", len(toolResultMessages(messages)))
	}
	if toolResultMessages(messages)[0].Content == "" {
		t.Fatal("expected non-empty tool error content")
	}

	updatedAssistant, ok := assistantMessageByID(messages, assistantMessage.ID)
	if !ok {
		t.Fatal("assistant message not found")
	}
	if updatedAssistant.ToolCalls[0].ExecutionStatus != ExecutionStatusFailed {
		t.Fatalf("ExecutionStatus = %q, want failed", updatedAssistant.ToolCalls[0].ExecutionStatus)
	}
}

func TestRunAppendsToolMessageOnUnknownTool(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	registry := NewToolRegistry()

	assistantMessage := NewAssistantMessageWithToolCalls("", []ToolCall{
		{
			ID:              "call_a",
			ToolName:        "missing_tool",
			Args:            json.RawMessage(`{}`),
			ApprovalStatus:  ApprovalStatusApproved,
			ExecutionStatus: ExecutionStatusPending,
		},
	})
	if err := store.AddMessages(context.Background(), "chat-1", assistantMessage); err != nil {
		t.Fatalf("AddMessages() error = %v", err)
	}

	agent := &Agent{
		store:         store,
		broadcaster:   NopBroadcaster{},
		model:         &stubModel{responses: []Message{NewAssistantMessage("done")}},
		toolRegistry:  registry,
		maxIterations: 3,
	}

	if err := agent.Run(context.Background(), "chat-1"); err != nil {
		t.Fatalf("Run() error = %v, want nil with not-found tool transcript", err)
	}

	messages, err := store.Load(context.Background(), "chat-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(toolResultMessages(messages)) != 1 {
		t.Fatalf("tool result messages = %d, want 1", len(toolResultMessages(messages)))
	}
}

func TestRegisterToolDuplicateReturnsError(t *testing.T) {
	t.Parallel()

	registry := NewToolRegistry()
	if err := registry.RegisterTool(stubTool{name: "addition"}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}
	if err := registry.RegisterTool(stubTool{name: "addition"}); err == nil {
		t.Fatal("RegisterTool() error = nil, want duplicate name error")
	}
}

func TestResolvedToolCallIDsForTurnIgnoresEarlierTranscript(t *testing.T) {
	t.Parallel()

	messages := []Message{
		NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_shared"}}),
		NewToolResultMessage("call_shared", "old-result"),
		NewAssistantMessageWithToolCalls("", []ToolCall{{ID: "call_shared"}}),
	}

	resolved := resolvedToolCallIDsForTurn(messages, 2)
	if _, ok := resolved["call_shared"]; ok {
		t.Fatal("tool result before assistant turn should not resolve that turn's call")
	}
}
