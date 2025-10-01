package agent

import (
	"crypto/sha1"
	"encoding/hex"

	"github.com/RafaelZelak/agentkit/internal/openai"
)

type Option func(*builder)

func WithSystemPrompt(prompt string) Option {
	return func(b *builder) {
		if prompt == "" {
			return
		}
		b.system = append(b.system, openai.Message{
			Type: "message",
			Role: "system",
			Content: []openai.ContentItem{
				{Type: "input_text", Text: prompt},
			},
		})
	}
}

func WithCachedContext(text string) Option {
	return func(b *builder) {
		if text == "" {
			return
		}
		b.system = append(b.system, openai.Message{
			Type: "message",
			Role: "system",
			Content: []openai.ContentItem{
				{Type: "input_text", Text: text},
			},
		})
		if b.promptCacheKey == "" {
			sum := sha1.Sum([]byte(text))
			b.promptCacheKey = "ctx-" + hex.EncodeToString(sum[:])
		}
	}
}
