package tools

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ToolConfig representa a configuração de uma tool
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

	// Para scripts
	Path     string `yaml:"path,omitempty"`
	Function string `yaml:"function,omitempty"`
}

// Config representa a configuração do arquivo tools.yml
type Config struct {
	Tools []ToolConfig `yaml:"tools"`
}

var loaded Config

// LoadTools carrega tools de um arquivo YAML para a variável global.
// Deprecated: Use tools.NewRegistry para criar instâncias independentes por agente.
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

// GetTool retorna uma tool por nome da variável global.
// Deprecated: Use Registry.Get para instâncias independentes por agente.
func GetTool(name string) *ToolConfig {
	for i := range loaded.Tools {
		if loaded.Tools[i].Name == name {
			return &loaded.Tools[i]
		}
	}
	return nil
}
