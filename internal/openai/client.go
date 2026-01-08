package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

const (
	defaultTimeout     = 60 * time.Second
	defaultMaxAttempts = 3
	retryBaseDelay     = 300 * time.Millisecond
)

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
	apiKey         string
	baseURL        string
	chatPath       string
	embeddingsPath string
	httpClient     *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:         apiKey,
		baseURL:        os.Getenv("OPENAI_BASE_URL"),
		chatPath:       os.Getenv("OPENAI_CHAT_PATH"),
		embeddingsPath: os.Getenv("OPENAI_EMBEDDINGS_PATH"),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (c *Client) validate() error {
	if c.baseURL == "" || c.chatPath == "" || c.embeddingsPath == "" {
		return fmt.Errorf("openai env vars not configured")
	}
	return nil
}

func (c *Client) Respond(ctx context.Context, req *ChatCompletionRequest) (*ResponseEnvelope, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	body, _ := json.Marshal(req)

	url := c.baseURL + c.chatPath
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	var err error

	for attempt := 1; attempt <= defaultMaxAttempts; attempt++ {
		resp, err = c.httpClient.Do(httpReq)
		if err == nil && resp.StatusCode < http.StatusInternalServerError {
			break
		}

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		if attempt == defaultMaxAttempts {
			break
		}

		time.Sleep(time.Duration(attempt) * retryBaseDelay)
	}

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
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
		Usage Usage `json:"usage"`
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
	if err := c.validate(); err != nil {
		return nil, err
	}

	req := embeddingsRequest{
		Model: model,
		Input: []string{text},
	}
	body, _ := json.Marshal(req)

	url := c.baseURL + c.embeddingsPath
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
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
