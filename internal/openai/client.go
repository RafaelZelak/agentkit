package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type Message struct {
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

type ResponsesRequest struct {
	Model           string    `json:"model"`
	Input           []Message `json:"input"`
	PromptCacheKey  string    `json:"prompt_cache_key,omitempty"`
	MaxOutputTokens int       `json:"max_output_tokens,omitempty"`
}

type ResponseEnvelope struct {
	ID         string         `json:"id"`
	OutputText string         `json:"output_text"`
	Raw        map[string]any `json:"-"`
}

const DefaultEmbeddingModel = "text-embedding-3-small"

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Respond(ctx context.Context, req *ResponsesRequest) (*ResponseEnvelope, error) {
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// Retry simples para 5xx
	var resp *http.Response
	var err error
	for i := 0; i < 3; i++ {
		resp, err = c.httpClient.Do(httpReq)
		if err != nil {
			if i == 2 {
				return nil, err
			}
			time.Sleep(time.Duration(i+1) * 300 * time.Millisecond)
			continue
		}
		if resp.StatusCode >= 500 {
			if i == 2 {
				break
			}
			resp.Body.Close()
			time.Sleep(time.Duration(i+1) * 300 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("openai error: %s\n%v", resp.Status, errBody)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	out := &ResponseEnvelope{Raw: raw}
	if v, ok := raw["id"].(string); ok {
		out.ID = v
	}

	if outputArr, ok := raw["output"].([]any); ok && len(outputArr) > 0 {
		if first, ok := outputArr[0].(map[string]any); ok {
			if content, ok := first["content"].([]any); ok && len(content) > 0 {
				if c0, ok := content[0].(map[string]any); ok {
					if txt, ok := c0["text"].(string); ok {
						out.OutputText = txt
					}
				}
			}
		}
	}

	return out, nil
}

func (c *Client) Embed(ctx context.Context, model string, text string) ([]float32, error) {
	req := embeddingsRequest{
		Model: model,
		Input: []string{text},
	}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("openai embeddings error: %s\n%v", resp.Status, errBody)
	}

	var out embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}

	src := out.Data[0].Embedding
	dst := make([]float32, len(src))
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst, nil
}
