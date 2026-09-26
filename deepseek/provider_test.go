package deepseek

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cgund98/gogent"
)

func TestNewChatBuildsDeepSeekModel(t *testing.T) {
	model, err := NewChat("test-key", gogent.NewToolRegistry()).
		WithModel("deepseek-flash").
		WithSystemPrompt("hello").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if model.SystemPrompt() != "hello" {
		t.Fatalf("prompt = %q", model.SystemPrompt())
	}
}

func TestThinkingFieldFollowsOption(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []Option
		want any
	}{
		{"default omits thinking", nil, nil},
		{"with thinking enables it", []Option{WithThinking()}, map[string]any{"type": "enabled"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Errorf("decode body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"1","object":"chat.completion","created":0,"model":"deepseek-flash","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"hi"}}]}`)
			}))
			defer server.Close()

			opts := append([]Option{func(s *settings) { s.baseURL = server.URL }}, tc.opts...)
			model, err := NewChat("test-key", gogent.NewToolRegistry(), opts...).WithModel("deepseek-flash").Build()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := model.GenerateResponse(context.Background(), []gogent.Message{{Role: gogent.MessageRoleUser, Content: "hi"}}); err != nil {
				t.Fatal(err)
			}
			got, ok := body["thinking"]
			if tc.want == nil {
				if ok {
					t.Fatalf("thinking = %v, want omitted", got)
				}
				return
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("thinking = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}
