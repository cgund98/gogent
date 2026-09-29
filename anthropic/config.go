package anthropic

type modelSettings struct {
	systemPrompt  string
	model         string
	maxTokens     *int
	temperature   *float64
	topP          *float64
	topK          *int
	stopSequences []string
	effort        *string
}
