package agentkit

import (
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config representa a configuração completa de um agente.
type Config struct {
	// Identificação
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Configuração de IA
	APIKey       string `yaml:"api_key"`
	GPTModel     string `yaml:"gpt_model"`
	EmbModel     string `yaml:"embedding_model"`
	EmbeddingDim int    `yaml:"embedding_dim"`
	BaseURL      string `yaml:"base_url"` // Opcional: compatibilidade com outros providers

	// Banco de dados
	DSN    string `yaml:"dsn"`
	Schema string `yaml:"schema"`

	// Caminhos (a mágica acontece aqui!)
	PromptsDir   string `yaml:"prompts_dir"`   // Pasta base dos prompts
	BasePrompt   string `yaml:"base_prompt"`   // Prompt principal
	RouterPrompt string `yaml:"router_prompt"` // Router (opcional)
	ToolsPath    string `yaml:"tools_path"`    // Config de tools

	// Configurações extras
	Timeout time.Duration `yaml:"timeout"`
	Verbose bool          `yaml:"verbose"`
}

// MultiConfig representa múltiplas configurações de agentes.
type MultiConfig struct {
	Agents []Config `yaml:"agents"`
}

// resolveEnvVars resolve variáveis de ambiente no formato ${VAR_NAME} em uma string.
func resolveEnvVars(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	result := s
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		envVar := result[start+2 : end]
		envVal := os.Getenv(envVar)
		result = result[:start] + envVal + result[end+1:]
	}
	return result
}

// resolveConfigEnvVars resolve variáveis de ambiente em todos os campos de Config.
func resolveConfigEnvVars(cfg *Config) {
	cfg.APIKey = resolveEnvVars(cfg.APIKey)
	cfg.DSN = resolveEnvVars(cfg.DSN)
	cfg.Schema = resolveEnvVars(cfg.Schema)
	cfg.GPTModel = resolveEnvVars(cfg.GPTModel)
	cfg.EmbModel = resolveEnvVars(cfg.EmbModel)
	cfg.BaseURL = resolveEnvVars(cfg.BaseURL)
	cfg.PromptsDir = resolveEnvVars(cfg.PromptsDir)
	cfg.BasePrompt = resolveEnvVars(cfg.BasePrompt)
	cfg.RouterPrompt = resolveEnvVars(cfg.RouterPrompt)
	cfg.ToolsPath = resolveEnvVars(cfg.ToolsPath)
}

// NewConfigFromYAML carrega configurações de múltiplos agentes de um arquivo YAML.
func NewConfigFromYAML(path string) (*MultiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg MultiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Resolver variáveis de ambiente
	for i := range cfg.Agents {
		resolveConfigEnvVars(&cfg.Agents[i])

		// Defaults
		if cfg.Agents[i].Timeout == 0 {
			cfg.Agents[i].Timeout = 60 * time.Second
		}
		if cfg.Agents[i].EmbModel == "" {
			cfg.Agents[i].EmbModel = "text-embedding-3-small"
		}
		if cfg.Agents[i].EmbeddingDim == 0 {
			cfg.Agents[i].EmbeddingDim = 1536
		}
	}

	return &cfg, nil
}

// LoadAgents carrega todos os agentes de um arquivo YAML e retorna um AgentManager.
func LoadAgents(path string) (*AgentManager, error) {
	cfg, err := NewConfigFromYAML(path)
	if err != nil {
		return nil, err
	}

	manager := NewAgentManager()
	for _, agentCfg := range cfg.Agents {
		cfgCopy := agentCfg // Evitar closure sobre variável de loop
		if _, err := manager.Register(&cfgCopy); err != nil {
			manager.Close() // Limpar agentes já criados
			return nil, err
		}
	}

	return manager, nil
}

// NewConfigFromEnv cria uma configuração a partir de variáveis de ambiente.
// Mantido para retrocompatibilidade com uso de agente único.
func NewConfigFromEnv() (*Config, error) {
	embDimStr := os.Getenv("EMBEDDING_DIM")
	embDim, err := strconv.Atoi(embDimStr)
	if err != nil || embDim <= 0 {
		embDim = 1536 // Default
	}

	timeoutStr := os.Getenv("AGENT_TIMEOUT")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil || timeout <= 0 {
		timeout = 60 * time.Second
	}

	cfg := &Config{
		Name:         os.Getenv("AGENT_NAME"),
		APIKey:       os.Getenv("OPENAI_API_KEY"),
		DSN:          os.Getenv("PGSQL"),
		Schema:       os.Getenv("DB_SCHEMA"),
		EmbeddingDim: embDim,
		GPTModel:     os.Getenv("GPT_MODEL"),
		EmbModel:     os.Getenv("EMBEDDING_MODEL"),
		ToolsPath:    os.Getenv("TOOLS_PATH"),
		PromptsDir:   os.Getenv("PROMPTS_DIR"),
		BasePrompt:   os.Getenv("BASE_PROMPT"),
		RouterPrompt: os.Getenv("ROUTER_PROMPT"),
		Timeout:      timeout,
		Verbose:      os.Getenv("VERBOSE") == "true",
	}

	if cfg.Name == "" {
		cfg.Name = "default"
	}
	if cfg.ToolsPath == "" {
		cfg.ToolsPath = "tools.yml"
	}
	if cfg.EmbModel == "" {
		cfg.EmbModel = "text-embedding-3-small"
	}

	return cfg, nil
}
