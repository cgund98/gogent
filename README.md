# Gogent

A Go library for building chatbot-style agents with tool calling and human-in-the-loop approval. The API and design are still in active R&D and may change without a stable release guarantee.

## Purpose

Gogent targets applications where a user converses with an agent in a chat interface: load history, call a model, run tools, persist results, and pause for human approval when needed. It stays intentionally small while including the pieces you need for a working app:

- **Pluggable interfaces** — swap the LLM provider (`Model`), persistence (`MessageStore`), and tools (`ToolRegistry`) without changing the agent loop.
- **Core agent loop** — orchestrates history, model turns, tool execution, and transcript updates until the model finishes or a guardrail stops the run.
- **Human-in-the-loop** — require explicit approval before sensitive tools run; workflow state stays separate from what the model sees on the next turn.

Provider-specific wiring lives in separate packages (for example `openai`). See [ARCHITECTURE.md](ARCHITECTURE.md) for message types, lifecycle, and approval flow.

## Examples

Two runnable examples ship with the repo:

| Example | Description | Run |
|---------|-------------|-----|
| [`examples/simple`](examples/simple) | Minimal OpenAI demo: auto-executed tools and approval for sensitive tools | `make example-simple` (requires `OPENAI_API_KEY` in `.env` at repo root) |
| [`examples/tui`](examples/tui) | Terminal chat UI (Bubble Tea) with the same agent patterns | `go run ./examples/tui` |

Details: [examples/simple/README.md](examples/simple/README.md), [examples/tui/README.md](examples/tui/README.md). Setup and tests: [DEVELOPMENT.md](DEVELOPMENT.md).

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) — message types, agent lifecycle, approval flow
- [DEVELOPMENT.md](DEVELOPMENT.md) — setup, tests, running examples
