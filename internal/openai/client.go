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

type ChatCompletionRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	MaxOutputTokens int       `json:"max_tokens,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ResponseEnvelope struct {
	ID         string         `json:"id"`
	OutputText string         `json:"output_text"`
	Usage      Usage          `json:"usage"`
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

func (c *Client) Respond(ctx context.Context, req *ChatCompletionRequest) (*ResponseEnvelope, error) {
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

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

	var rawResponse struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage Usage          `json:"usage"`
		Raw   map[string]any `json:"-"`
	}

	var rawMap map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rawMap); err != nil {
		return nil, err
	}

	rawBytes, _ := json.Marshal(rawMap)
	_ = json.Unmarshal(rawBytes, &rawResponse)

	out := &ResponseEnvelope{
		ID:    rawResponse.ID,
		Usage: rawResponse.Usage,
		Raw:   rawMap,
	}

	if len(rawResponse.Choices) > 0 {
		out.OutputText = rawResponse.Choices[0].Message.Content
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
