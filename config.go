package agentkit

import (
	"os"
	"strconv"
)

type Config struct {
	APIKey       string
	DSN          string
	Schema       string
	EmbeddingDim int
	GPTModel     string
	EmbModel     string
	ToolsPath    string
}

// NewConfigFromEnv lê variáveis de ambiente e monta uma Config
func NewConfigFromEnv() (*Config, error) {
	embDimStr := os.Getenv("EMBEDDING_DIM")
	embDim, err := strconv.Atoi(embDimStr)
	if err != nil || embDim <= 0 {
		return nil, err
	}

	cfg := &Config{
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		DSN:          os.Getenv("PGSQL"),
		Schema:       os.Getenv("DB_SCHEMA"),
		EmbeddingDim: embDim,
		GPTModel:     os.Getenv("GPT_MODEL"),
		EmbModel:     os.Getenv("EMBEDDING_MODEL"),
		ToolsPath:    os.Getenv("TOOLS_PATH"),
	}

	if cfg.ToolsPath == "" {
		cfg.ToolsPath = "tools.yml"
	}
	return cfg, nil
}
