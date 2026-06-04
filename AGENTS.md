# Project Instructions

Gogent is a Go **library** for agentic apps with tool calling and human-in-the-loop guardrails. Start with [README.md](README.md), [ARCHITECTURE.md](ARCHITECTURE.md), and [DEVELOPMENT.md](DEVELOPMENT.md).

## Layout

| Path | Role |
|------|------|
| `agent.go` | Public agent API, run loop, store/broadcast helpers |
| `tool_turn.go` | Transcript scanning — unresolved turns, turn-scoped tool-result matching, `ListPendingToolCalls` filtering |
| `tool_execution.go` | Per-call execution — `processToolCalls`, `ExecuteTool` |
| `message.go`, `message_test.go` | Message types and assistant `ToolCall` helpers |
| `tool.go` | `Tool` interface, registry, error payloads |
| `model.go` | `Model` interface |
| `event.go` | `ChatEventBroadcaster` and built-in broadcasters |
| `openai/` | OpenAI Chat Completions `Model` builder and mapper |
| `inmemory/` | In-memory `MessageStore` |
| `examples/core` | Shared env/settings helpers for examples |
| `examples/simple` | Runnable OpenAI example (root module) |
| `examples/tui` | Separate module (Bubble Tea UI) |

The `internal/` tree is legacy application scaffolding and is not part of the public library API.

## Agent loop (where to change what)

Read [ARCHITECTURE.md](ARCHITECTURE.md) for the full lifecycle. In code:

- **`run` / `findAndApplyUnresolvedToolCalls`** (`agent.go`) — orchestration only. Each iteration resolves tool work first, then optionally calls the model. Tool resolution does not count toward `maxIterations`.
- **`findUnresolvedToolTurn`, `resolvedToolCallIDsForTurn`** (`tool_turn.go`) — which assistant turn is outstanding; a `tool` message counts only when it appears **after** that assistant message.
- **`processToolCalls`** (`tool_execution.go`) — approve/reject/execute/fail individual calls and append `tool` transcript messages.

Do not duplicate tool execution in `run`; new tool requests from the model are handled on the next loop iteration via `findAndApplyUnresolvedToolCalls`.

## Working rules

- Keep the agent loop and business rules in the root `gogent` package.
- Keep provider-specific code in `openai/` (wire format, SDK calls).
- Controllers and HTTP are out of scope unless explicitly requested.
- Prefer small, focused changes; run `go test ./...` for the packages you touch.
- Run `make lint` before finishing (see [DEVELOPMENT.md](DEVELOPMENT.md)).
- Format with `gofmt` before finishing.

## Adding behavior

When extending the agent or OpenAI adapter, read existing tests in `agent_test.go`, `message_test.go`, and `openai/*_test.go` and mirror their patterns.

- Use `mustRegisterTool` in tests when registering tools; `RegisterTool` returns an error on duplicate names.
- `NewAgent` takes a `*ToolRegistry` pointer — register tools on that registry before or after construction, not on a copy.
- Approval changes broadcast via `updateMessage` (`message_updated`); new transcript rows broadcast via `addMessages` (`message_added`).

## Practical preference

When unsure, follow the patterns in `examples/simple` and the tests closest to the code you are changing.
