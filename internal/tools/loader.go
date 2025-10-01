package tools

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ToolConfig struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	Type          string `yaml:"type"`
	Conn          string `yaml:"conn,omitempty"`
	QueryTemplate string `yaml:"query_template,omitempty"`

	// Para embeddings
	Table          string `yaml:"table,omitempty"`
	Column         string `yaml:"column,omitempty"`
	EmbeddingModel string `yaml:"embedding_model,omitempty"`
	TopK           int    `yaml:"top_k,omitempty"`
}

type Config struct {
	Tools []ToolConfig `yaml:"tools"`
}

var loaded Config

// LoadTools lê o arquivo YAML e substitui conexões do tipo ENV:MY_ENV_KEY pelo valor da env var
func LoadTools(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	for i := range cfg.Tools {
		if strings.HasPrefix(cfg.Tools[i].Conn, "ENV:") {
			envKey := strings.TrimPrefix(cfg.Tools[i].Conn, "ENV:")
			cfg.Tools[i].Conn = os.Getenv(envKey)
		}
	}

	loaded = cfg
	return nil
}

// GetTool retorna o ponteiro para o ToolConfig correto
// Corrigido para iterar por índice, evitando retornar o endereço da cópia do range
func GetTool(name string) *ToolConfig {
	for i := range loaded.Tools {
		if loaded.Tools[i].Name == name {
			return &loaded.Tools[i]
		}
	}
	return nil
}
