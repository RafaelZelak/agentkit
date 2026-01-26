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
		// Process template functions in the prompt
		processedPrompt, funcsUsed, err := functions.ProcessTemplateWithTracking(prompt)
		if err != nil {
			// On error, use original prompt
			processedPrompt = prompt
		} else {
			// Merge functions used into builder's tracking
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
		// Process template functions in the context
		processedText, funcsUsed, err := functions.ProcessTemplateWithTracking(text)
		if err != nil {
			// On error, use original text
			processedText = text
		} else {
			// Merge functions used into builder's tracking
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
