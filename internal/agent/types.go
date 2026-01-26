package agent

import "github.com/RafaelZelak/agentkit/internal/openai"

type builder struct {
	system         []openai.Message
	user           openai.ContentItem
	promptCacheKey string
	functionsUsed  map[string]string
}

func newBuilder() *builder {
	return &builder{
		system:        make([]openai.Message, 0, 6),
		functionsUsed: make(map[string]string),
	}
}

func (b *builder) req(model string) *openai.ChatCompletionRequest {
	messages := make([]openai.Message, 0, len(b.system)+1)
	messages = append(messages, b.system...)
	messages = append(messages, openai.Message{
		Type: "message",
		Role: "user",
		Content: []openai.ContentItem{
			b.user,
		},
	})
	return &openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}
}
