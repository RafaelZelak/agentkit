package agentkit

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	APIKey       string `yaml:"api_key"`
	GPTModel     string `yaml:"gpt_model"`
	EmbModel     string `yaml:"embedding_model"`
	EmbeddingDim int    `yaml:"embedding_dim"`
	BaseURL      string `yaml:"base_url"`

	DSN    string `yaml:"dsn"`
	Schema string `yaml:"schema"`

	PromptsDir   string `yaml:"prompts_dir"`
	BasePrompt   string `yaml:"base_prompt"`
	RouterPrompt string `yaml:"router_prompt"`
	ToolsPath    string `yaml:"tools_path"`
	FunctionsPath string `yaml:"functions_path"`

	Timeout time.Duration `yaml:"timeout"`
	Verbose bool          `yaml:"verbose"`
}

type MultiConfig struct {
	Agents []Config `yaml:"agents"`
}

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
	cfg.FunctionsPath = resolveEnvVars(cfg.FunctionsPath)
}

func NewConfigFromYAML(path string) (*MultiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg MultiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for i := range cfg.Agents {
		resolveConfigEnvVars(&cfg.Agents[i])

		agentName := cfg.Agents[i].Name
		if agentName == "" {
			return nil, fmt.Errorf("agent[%d]: name is required", i)
		}

		// Validate required fields
		if cfg.Agents[i].APIKey == "" {
			return nil, fmt.Errorf("agent '%s': api_key is required", agentName)
		}
		if cfg.Agents[i].GPTModel == "" {
			return nil, fmt.Errorf("agent '%s': gpt_model is required", agentName)
		}
		if cfg.Agents[i].DSN == "" {
			return nil, fmt.Errorf("agent '%s': dsn is required", agentName)
		}
		if cfg.Agents[i].Schema == "" {
			return nil, fmt.Errorf("agent '%s': schema is required", agentName)
		}
		if cfg.Agents[i].PromptsDir == "" {
			return nil, fmt.Errorf("agent '%s': prompts_dir is required", agentName)
		}
		if cfg.Agents[i].BasePrompt == "" {
			return nil, fmt.Errorf("agent '%s': base_prompt is required", agentName)
		}

		// Set defaults for optional fields
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

func LoadAgents(path string) (*AgentManager, error) {
	cfg, err := NewConfigFromYAML(path)
	if err != nil {
		return nil, err
	}

	manager := NewAgentManager()
	for _, agentCfg := range cfg.Agents {
		cfgCopy := agentCfg
		if _, err := manager.Register(&cfgCopy); err != nil {
			manager.Close()
			return nil, err
		}
	}

	return manager, nil
}

func NewConfigFromEnv() (*Config, error) {
	embDimStr := os.Getenv("EMBEDDING_DIM")
	embDim, err := strconv.Atoi(embDimStr)
	if err != nil || embDim <= 0 {
		embDim = 1536
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
		ToolsPath:     os.Getenv("TOOLS_PATH"),
		FunctionsPath: os.Getenv("FUNCTIONS_PATH"),
		PromptsDir:    os.Getenv("PROMPTS_DIR"),
		BasePrompt:    os.Getenv("BASE_PROMPT"),
		RouterPrompt:  os.Getenv("ROUTER_PROMPT"),
		Timeout:       timeout,
		Verbose:       os.Getenv("VERBOSE") == "true",
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
