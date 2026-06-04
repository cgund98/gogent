package openai

import (
	"encoding/json"
)

type modelSettings struct {
	systemPrompt      string
	model             string
	temperature       *float64
	maxTokens         *int
	topP              *float64
	frequencyPenalty  *float64
	presencePenalty   *float64
	stopSequences     []string
	toolChoice        *string
	seed              *int64
	user              string
	parallelToolCalls *bool
	responseFormat    json.RawMessage
}
