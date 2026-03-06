package agentkit

import "github.com/RafaelZelak/agentkit/internal/openai"

type ChatResponse struct {
	Text         string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

func newChatResponse(text string, usage *openai.Usage) *ChatResponse {
	if usage == nil {
		return &ChatResponse{Text: text}
	}
	return &ChatResponse{
		Text:         text,
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}
}
