package agent

import "github.com/RafaelZelak/agentkit/internal/openai"

type builder struct {
	system         []openai.Message
	user           openai.ContentItem
	promptCacheKey string
}

func newBuilder() *builder {
	return &builder{
		system: make([]openai.Message, 0, 6),
	}
}

func (b *builder) req(model string) *openai.ResponsesRequest {
	input := make([]openai.Message, 0, len(b.system)+1)
	input = append(input, b.system...)
	input = append(input, openai.Message{
		Type: "message",
		Role: "user",
		Content: []openai.ContentItem{
			b.user,
		},
	})
	return &openai.ResponsesRequest{
		Model:          model,
		Input:          input,
		PromptCacheKey: b.promptCacheKey,
	}
}
