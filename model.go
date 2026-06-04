package gogent

import "context"

// Model calls an LLM provider with conversation history and returns the next assistant message.
type Model interface {
	GenerateResponse(context.Context, []Message) (Message, error)
}
