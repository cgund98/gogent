# Gogent

A Go library for building agents with tool calling and human-in-the-loop approval.

## Features

- Provider-agnostic agent loop (`Model`, `MessageStore`, `ToolRegistry`)
- OpenAI adapter (`openai` package)
- Human approval for sensitive tools before execution
- Dual-layer state: workflow metadata on assistant messages, transcript messages for the model

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) — message types, agent lifecycle, approval flow
- [DEVELOPMENT.md](DEVELOPMENT.md) — setup, tests, running examples

## Quick example

```bash
# Set OPENAI_API_KEY in .env at the repo root
make example-simple
```

## Status

The core library and OpenAI integration are usable. Examples and docs are evolving.
