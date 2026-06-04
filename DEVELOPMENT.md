# Development

## Prerequisites

- Go 1.25+
- [golangci-lint](https://golangci-lint.run/) (for `make lint`)

## Quick start

```bash
git clone <repository-url>
cd gogent

go test ./...
```

## Running the OpenAI example

From the repository root, create a `.env` file with your API key:

```
OPENAI_API_KEY=sk-...
```

Then:

```bash
make example-simple
# or: go run ./examples/simple
```

The example loads `.env.local` first, then `.env`, then environment variables.

## Makefile

| Command | Description |
|---------|-------------|
| `make test` | Run all tests |
| `make format` | Format with `go fmt` |
| `make lint` | Run golangci-lint |
| `make tidy` | `go mod tidy` |
| `make verify` | `go mod verify` |
| `make example-simple` | Run the simple agent example |

### Optional Docker workspace

A dev container with Go, golangci-lint, and air is available if you prefer an isolated environment:

```bash
make workspace-build
make workspace-up
make workspace-shell
```

Inside the container, run the same `go test`, `go fmt`, and `golangci-lint` commands from `/workspace`.

## Examples

| Path | Description |
|------|-------------|
| `examples/simple` | OpenAI tool-calling demo (part of the root module) |
| `examples/tui` | Bubble Tea TUI (separate module; see its README) |

## Project layout

| Path | Purpose |
|------|---------|
| `agent.go`, `message.go`, `tool.go` | Core agent loop and types |
| `openai/` | OpenAI Chat Completions adapter |
| `inmemory/` | In-memory `MessageStore` |
| `examples/` | Sample applications |

See [ARCHITECTURE.md](ARCHITECTURE.md) for message lifecycle, tool approval, and provider mapping.
