# Architecture

Gogent is a library for building agentic applications.

Key features:
- Human-in-the-loop guardrails for tool integrations.
- Provider-agnostic agent loop with a pluggable `Model` and `MessageStore`.

## Components

- **Model** — calls the LLM with conversation history and returns the next assistant message. Implementations live in provider packages (e.g. `openai.NewChat(...).Build()`).
- **MessageStore** — persists chat history so it can be loaded and continued later.
- **Tool** — interfaces the agent with functions written by a developer.
- **ToolRegistry** — lookup table of registered tools available to an agent. `RegisterTool` returns an error if a name is already registered. The agent holds a `*ToolRegistry` pointer so later registrations on the same registry are visible to the agent.
- **ChatEventBroadcaster** — optional hook for UI or streaming clients. The agent emits `message_added` and `message_updated` events when messages are persisted. Use `NopBroadcaster` when events are not needed.
- **Agent** — orchestrates the conversation loop: resolve outstanding tool work, call the model, persist results, repeat until the model produces a final answer or a guardrail pauses the run.

## Source layout

| File | Responsibility |
|------|----------------|
| `agent.go` | Public agent API, run loop, store/broadcast helpers |
| `tool_turn.go` | Transcript scanning — unresolved turns, turn-scoped tool-result matching, `ListPendingToolCalls` filtering |
| `tool_execution.go` | Per-call execution — `processToolCalls`, `ExecuteTool` |
| `message.go` | Message types and assistant `ToolCall` helpers |
| `tool.go` | `Tool` interface, registry, error payloads |
| `event.go` | `ChatEventBroadcaster` and built-in broadcasters |
| `openai/` | OpenAI Chat Completions adapter |
| `inmemory/` | In-memory `MessageStore` |

## Message types

All conversation state is stored as an ordered list of `Message` values. Each message has an internal `ID` (assigned by gogent), a `Role`, and role-specific fields.

| Role | Created by | Purpose | Key fields |
|------|------------|---------|------------|
| `user` | Application / `NewUserMessage` | Input from the end user | `Content` |
| `assistant` (text) | Model / `NewAssistantMessage` | Final model reply with no pending tool work | `Content` |
| `assistant` (tool request) | Model / `NewAssistantMessageWithToolCalls` | Model asking to run one or more tools | `Content` (optional), `ToolCalls` |
| `tool` | Agent / `NewToolResultMessage` | Result of a single executed tool call | `ToolCallID`, `Content` |

### Field rules

- **`ToolCalls`** is only set on `assistant` messages. It holds every tool the model requested in that turn. A single assistant message may contain multiple tool calls when the provider supports parallel tool calling.
- **`ToolCallID`** is only set on `tool` messages. It must match the `ID` of the corresponding entry in the assistant message's `ToolCalls` slice. OpenAI and other providers use this ID to correlate results with requests.
- **`Content` on `tool` messages** holds the serialized tool output (usually JSON as a string). This is what gets sent back to the model on the next request.

### ToolCall (embedded in assistant messages)

Each entry in `ToolCalls` tracks the lifecycle of one requested call:

| Field | Purpose |
|-------|---------|
| `ID` | Provider-assigned call ID (e.g. OpenAI `call_abc123`) |
| `ToolName` | Name of the tool to invoke |
| `Args` | JSON arguments parsed from the model |
| `ApprovalStatus` | `pending`, `approved`, or `rejected` — used for human-in-the-loop guardrails |
| `ExecutionStatus` | `pending`, `running`, `completed`, or `failed` |
| `Result` | JSON output after execution; also copied into the corresponding `tool` message `Content` |

## Workflow state vs model transcript

Gogent uses two layers that must not be confused:

| Layer | Mechanism | Audience | Examples |
|-------|-----------|----------|----------|
| **Workflow state** | Mutate `ToolCall` fields on the assistant message + `MessageStore.UpdateMessage` | Agent, UI, persistence | `ApprovalStatus`, `ExecutionStatus`, cached `Result` |
| **Model transcript** | Append new messages via `MessageStore.AddMessages` | LLM on the next `GenerateResponse` | `tool` result messages; `user` messages from a separate follow-up input |

Mutating the assistant message alone does **not** inform the model about user decisions. Providers such as OpenAI have no field for approval status on `tool_calls`. The model only sees what is appended after the tool-request turn — typically one `tool` message per call.

```mermaid
flowchart TD
    subgraph workflow [Workflow layer]
        A1[Assistant message with ToolCalls]
        A2[UpdateMessage on approval or execution]
    end

    subgraph transcript [Transcript layer]
        T1[tool message with result]
        T2[tool message with rejection]
        T3[tool message with execution error]
    end

    A1 --> A2
    A2 -->|approved and executed| T1
    A2 -->|rejected| T2
    A2 -->|unknown tool or Execute error| T3
    T1 --> ModelTurn[Next Run or RunWithUserInput]
    T2 --> WaitForUser[Wait for separate user input]
    T3 --> ModelTurn
    WaitForUser --> ModelTurn
```

**Do append for the model:** tool results, rejection denials, and structured execution errors (unknown tool, failed execution).

**Do not append for the model:** pure status changes such as `pending` → `approved`, or in-progress execution state. User feedback after a rejection is a separate `RunWithUserInput` call, not part of `RejectToolCall`.

### Tool result payloads

| Situation | Transcript `content` |
|-----------|----------------------|
| Successful execution | Tool JSON return value |
| User rejection | `ToolCallRejectedContent` — `{"error":"rejected",...}` |
| Unknown tool name | `ToolCallNotFoundContent` — `{"error":"not_found",...}` |
| `Tool.Execute` error | `ToolCallExecutionErrorContent` — `{"error":"execution_failed",...}` |

Failed and unknown-tool calls mark the assistant `ToolCall` as `ExecutionStatusFailed` and still append a `tool` message so the model can recover on the next turn.

The OpenAI adapter strips gogent-only `ToolCall` fields when serializing assistant messages. Only `id`, tool name, and arguments are sent to the provider.

## Interaction lifecycle

The agent runs a loop until the model returns a final assistant message, a guardrail pauses execution, or `maxIterations` **model turns** is reached. Tool resolution does not count toward that limit.

Each loop iteration **resolves tool work first**, then optionally calls the model:

1. **`findAndApplyUnresolvedToolCalls`** — scan the transcript for the most recent assistant turn that still lacks `tool` messages, execute/reject/fail each unresolved call, append results, and pause if any approval-required call is still pending.
2. **Stop** if paused, if `invokeModel` is false (e.g. after `RejectToolCall`), or if the model returned a final answer on the previous iteration.
3. **`GenerateResponse`** — call the model with the updated in-memory history.
4. **Loop** if the new assistant message contains `ToolCalls`; tool work runs at step 1 on the next iteration.

A `tool` message counts toward resolving a turn only when it appears **after** that assistant message in the transcript. This prevents tool-call ID collisions across earlier turns.

```mermaid
sequenceDiagram
    participant User
    participant Agent
    participant Store as MessageStore
    participant Model
    participant Tool

    User->>Agent: RunWithUserInput(chatID, input)
    Agent->>Store: Add user message
    Agent->>Store: Load full history

    loop until final answer or pause
        Agent->>Agent: findAndApplyUnresolvedToolCalls

        alt unresolved tool calls remain
            loop each unresolved ToolCall on current turn
                alt requires approval and still pending
                    Agent->>Store: Update assistant message
                    Agent-->>User: paused (awaiting approval)
                else rejected
                    Agent->>Store: Update assistant ToolCalls
                    Agent->>Store: Add tool rejection message
                else unknown or execution error
                    Agent->>Store: Update assistant ToolCalls
                    Agent->>Store: Add structured tool error message
                else execute tool
                    Agent->>Tool: Execute(args)
                    Tool-->>Agent: result
                    Agent->>Store: Update assistant ToolCalls
                    Agent->>Store: Add tool result message
                end
            end
        end

        alt paused or tool turn only
            Agent-->>User: return
        else call model
            Agent->>Model: GenerateResponse(history)
            Model-->>Agent: assistant message
            Agent->>Store: Add assistant message
            alt assistant has no tool calls
                Agent-->>User: done (final answer)
            end
            Note over Agent: Next loop iteration handles new tool calls
        end
    end
```

### Step-by-step

**1. User input**

The application calls `Agent.RunWithUserInput`, which creates a `user` message and persists it:

```
{ role: user, content: "What's the weather in Paris and London?" }
```

**2. Resolve outstanding tool work**

Before each model turn, the agent finds the current unresolved assistant turn and processes each `ToolCall`:

1. Resolve the tool from `ToolRegistry`.
2. If the tool `RequiresApproval()` and the call is still `pending`, persist the assistant message and **pause** the run. Other calls on the same turn may already have been executed or rejected in the same pass.
3. If the call is **rejected**, mark it failed, append a **`tool` message** with a denial payload (do not execute the tool), and continue with any remaining calls in the same turn.
4. If the tool is **not registered**, append a structured `not_found` tool message and mark the call failed.
5. Otherwise auto-approve when approval is not required, set `ExecutionStatus` to `running`, execute the tool, and on success mark the call `completed`. On execution error, append a structured `execution_failed` tool message and mark the call failed.
6. Create one **`tool` message per settled call** (result, rejection, or error) and persist it:

```
{ role: tool, tool_call_id: "call_a", content: "{\"temp_c\":18}" }
{ role: tool, tool_call_id: "call_b", content: "{\"error\":\"rejected\",\"message\":\"Tool call rejected by user.\"}" }
{ role: tool, tool_call_id: "call_c", content: "{\"error\":\"execution_failed\",\"message\":\"...\",\"tool\":\"get_weather\"}" }
```

The agent only calls the model again once **every** `ToolCall` in that assistant turn has a corresponding `tool` message in the transcript.

**3. Model turn**

The agent calls `Model.GenerateResponse` with the full history (including any tool results appended in step 2). The model adapter serializes history to the provider API and parses the response into a gogent `Message`.

Two outcomes:

- **Final answer** — assistant message with text only, no `ToolCalls`. The agent stops and returns.
- **Tool request** — assistant message with one or more `ToolCalls`. The loop continues; step 2 handles the new turn on the next iteration.

Example assistant tool request:

```
{
  role: assistant,
  content: "",
  tool_calls: [
    { id: "call_a", tool_name: "get_weather", args: {"city":"Paris"}, approval: pending, execution: pending },
    { id: "call_b", tool_name: "get_weather", args: {"city":"London"}, approval: pending, execution: pending }
  ]
}
```

**4. Next model turn**

After step 2 completes for a tool-request turn, the loop returns to step 3. The model sees the tool outputs and either responds with text or requests more tools.

Example final assistant message:

```
{ role: assistant, content: "Paris is 18°C and London is 12°C." }
```

### End states

| Outcome | Condition |
|---------|-----------|
| Success | Assistant message with no `ToolCalls` |
| Paused for approval | Tool with `RequiresApproval()` and call not yet approved on the current unresolved turn |
| Error | Store/model error, or `maxIterations` model turns exceeded |

Tool execution failures and unknown tools do **not** stop the run; they append structured `tool` error messages so the model can recover on the next turn.

## Provider mapping (OpenAI)

Gogent's message shape mirrors the OpenAI Chat Completions tool-calling protocol:

| gogent | OpenAI |
|--------|--------|
| `user` message | `role: user` |
| `assistant` message (text) | `role: assistant` with `content` |
| `assistant` message (tool request) | `role: assistant` with `tool_calls` array |
| `tool` message | `role: tool` with `tool_call_id` and `content` |
| `ToolCall.ID` | `tool_calls[].id` |
| `ToolCall.Args` | `tool_calls[].function.arguments` (JSON string on the wire) |

The model adapter is responsible for serializing gogent messages to the provider schema on the way out and parsing provider responses back into `Message` and `ToolCall` values on the way in. The agent loop itself is provider-agnostic.

## Human-in-the-loop

Tools may set `RequiresApproval()` to opt into manual review before execution. When the model requests such a tool:

1. The assistant message (with pending `ToolCalls`) is persisted.
2. The agent returns without executing the tool or creating `tool` messages for that call.
3. The application discovers pending work via **`ListPendingToolCalls`**, which returns only approval-required, unresolved calls on the **current** assistant turn (not auto-approved tools and not stale turns).
4. The application calls **`ApproveToolCall`** or **`RejectToolCall`** as separate operations.

On a turn with multiple tool calls, calls that do not require approval (or are already approved) may execute in the same pass while approval-required calls remain pending.

### Approve

`ApproveToolCall` sets `ApprovalStatus` to `approved` on the requested call and broadcasts a `message_updated` event. The agent **does not run** until every `ToolCall` on that assistant message is settled (approved or rejected). Once all outstanding calls are settled, the agent runs once: executes approved calls, appends `tool` result messages, and continues to the next model turn when the tool turn is complete.

### Reject

`RejectToolCall` is a single operation that:

1. Sets `ApprovalStatus` to `rejected` on the assistant message and broadcasts a `message_updated` event.
2. Runs a tool-only pass (`invokeModel=false`) that appends a `tool` message with a structured denial payload.
3. **Stops without calling the model.**

User feedback after a rejection is a **separate API call** via `RunWithUserInput`. That appends a normal `user` message and then runs the agent, so the model sees the rejection `tool` message and the user's follow-up in one turn.

```
RejectToolCall(chatID, messageID, toolCallID)
  → assistant ToolCall marked rejected
  → tool message appended
  → agent stops

RunWithUserInput(chatID, "Please suggest an alternative instead.")
  → user message appended
  → model called with full history
```

Rejected calls are never executed. Approval-only status changes are workflow mutations and are not sent to the provider.
