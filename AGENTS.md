# Project Instructions

Gogent is a Go **library** for agentic apps with tool calling and human-in-the-loop guardrails. Start with [README.md](README.md), [ARCHITECTURE.md](ARCHITECTURE.md), and [DEVELOPMENT.md](DEVELOPMENT.md).

## Layout

| Path | Role |
|------|------|
| `agent.go`, `tool_turn.go`, `tool_execution.go`, `message.go`, `tool.go` | Core agent, tool turns, execution, messages, tools |
| `openai/` | OpenAI Chat Completions `Model` builder and mapper |
| `inmemory/` | In-memory `MessageStore` |
| `examples/simple` | Runnable OpenAI example |
| `examples/tui` | Separate module (Bubble Tea UI) |

The `internal/` tree is legacy application scaffolding and is not part of the public library API.

## Working rules

- Keep the agent loop and business rules in the root `gogent` package.
- Keep provider-specific code in `openai/` (wire format, SDK calls).
- Controllers and HTTP are out of scope unless explicitly requested.
- Prefer small, focused changes; run `go test ./...` for the packages you touch.
- Format with `gofmt` before finishing.

## Adding behavior

When extending the agent or OpenAI adapter, read existing tests in `agent_test.go`, `message_test.go`, and `openai/*_test.go` and mirror their patterns.

## Practical preference

When unsure, follow the patterns in `examples/simple` and the tests closest to the code you are changing.
