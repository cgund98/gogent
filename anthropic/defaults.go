package anthropic

const (
	defaultBaseURL   = "https://api.anthropic.com"
	defaultModel     = "claude-sonnet-5-5"
	defaultMaxTokens = 8192
)

// effortNone is a gogent-level sentinel: it means "send no output_config.effort".
// It is not a valid Anthropic wire value, so it is never forwarded to the SDK.
const effortNone = "none"
