package agent

import (
	"crypto/sha1"
	"encoding/hex"

	"github.com/RafaelZelak/agentkit/internal/functions"
	"github.com/RafaelZelak/agentkit/internal/openai"
)

type Option func(*builder)

func WithSystemPrompt(prompt string) Option {
	return func(b *builder) {
		if prompt == "" {
			return
		}
		processedPrompt, funcsUsed, err := functions.ProcessTemplateWithTracking(prompt)
		if err != nil {
			processedPrompt = prompt
		} else {
			for k, v := range funcsUsed {
				b.functionsUsed[k] = v
			}
		}
		b.system = append(b.system, openai.Message{
			Type: "message",
			Role: "system",
			Content: []openai.ContentItem{
				{Type: "text", Text: processedPrompt},
			},
		})
	}
}

func WithCachedContext(text string) Option {
	return func(b *builder) {
		if text == "" {
			return
		}
		processedText, funcsUsed, err := functions.ProcessTemplateWithTracking(text)
		if err != nil {
			processedText = text
		} else {
			for k, v := range funcsUsed {
				b.functionsUsed[k] = v
			}
		}
		b.system = append(b.system, openai.Message{
			Type: "message",
			Role: "system",
			Content: []openai.ContentItem{
				{Type: "text", Text: processedText},
			},
		})
		if b.promptCacheKey == "" {
			sum := sha1.Sum([]byte(text))
			b.promptCacheKey = "ctx-" + hex.EncodeToString(sum[:])
		}
	}
}
