package agentkit

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name        string `yaml:"name"             json:"name"`
	Description string `yaml:"description"      json:"description"`

	APIKey       string `yaml:"api_key"          json:"api_key"`
	GPTModel     string `yaml:"gpt_model"        json:"gpt_model"`
	EmbModel     string `yaml:"embedding_model"  json:"embedding_model"`
	EmbeddingDim int    `yaml:"embedding_dim"    json:"embedding_dim"`
	BaseURL      string `yaml:"base_url"         json:"base_url"`

	DSN    string `yaml:"dsn"    json:"dsn"`
	Schema string `yaml:"schema" json:"schema"`

	PromptsDir    string `yaml:"prompts_dir"     json:"prompts_dir"`
	BasePrompt    string `yaml:"base_prompt"     json:"base_prompt"`
	RouterPrompt  string `yaml:"router_prompt"   json:"router_prompt"`
	ToolsPath     string `yaml:"tools_path"      json:"tools_path"`
	FunctionsPath string `yaml:"functions_path"  json:"functions_path"`

	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	Verbose bool          `yaml:"verbose" json:"verbose"`
}

type MultiConfig struct {
	Agents []Config `yaml:"agents" json:"agents"`
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

func validateAndApplyDefaults(cfg *Config, index int) error {
	resolveConfigEnvVars(cfg)

	agentName := cfg.Name
	if agentName == "" {
		return fmt.Errorf("agent[%d]: name is required", index)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("agent '%s': api_key is required", agentName)
	}
	if cfg.GPTModel == "" {
		return fmt.Errorf("agent '%s': gpt_model is required", agentName)
	}
	if cfg.DSN == "" {
		return fmt.Errorf("agent '%s': dsn is required", agentName)
	}
	if cfg.Schema == "" {
		return fmt.Errorf("agent '%s': schema is required", agentName)
	}
	if cfg.PromptsDir == "" {
		return fmt.Errorf("agent '%s': prompts_dir is required", agentName)
	}
	if cfg.BasePrompt == "" {
		return fmt.Errorf("agent '%s': base_prompt is required", agentName)
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.EmbModel == "" {
		cfg.EmbModel = "text-embedding-3-small"
	}
	if cfg.EmbeddingDim == 0 {
		cfg.EmbeddingDim = 1536
	}

	return nil
}

func validateMultiConfig(cfg *MultiConfig) error {
	for agentIndex := range cfg.Agents {
		if err := validateAndApplyDefaults(&cfg.Agents[agentIndex], agentIndex); err != nil {
			return err
		}
	}
	return nil
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

	if err := validateMultiConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func NewConfigFromJSON(data []byte) (*MultiConfig, error) {
	var cfg MultiConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid json config: %w", err)
	}

	if err := validateMultiConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func NewConfigFromJSONFile(path string) (*MultiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewConfigFromJSON(data)
}

func bootManagerFromMultiConfig(cfg *MultiConfig) (*AgentManager, error) {
	manager := NewAgentManager()
	for agentIndex := range cfg.Agents {
		cfgCopy := cfg.Agents[agentIndex]
		if _, err := manager.Register(&cfgCopy); err != nil {
			manager.Close()
			return nil, err
		}
	}
	return manager, nil
}

func LoadAgents(path string) (*AgentManager, error) {
	cfg, err := NewConfigFromYAML(path)
	if err != nil {
		return nil, err
	}
	return bootManagerFromMultiConfig(cfg)
}

func LoadAgentsFromJSON(data []byte) (*AgentManager, error) {
	cfg, err := NewConfigFromJSON(data)
	if err != nil {
		return nil, err
	}
	return bootManagerFromMultiConfig(cfg)
}

func LoadAgentsFromJSONFile(path string) (*AgentManager, error) {
	cfg, err := NewConfigFromJSONFile(path)
	if err != nil {
		return nil, err
	}
	return bootManagerFromMultiConfig(cfg)
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
		Name:          os.Getenv("AGENT_NAME"),
		APIKey:        os.Getenv("OPENAI_API_KEY"),
		DSN:           os.Getenv("PGSQL"),
		Schema:        os.Getenv("DB_SCHEMA"),
		EmbeddingDim:  embDim,
		GPTModel:      os.Getenv("GPT_MODEL"),
		EmbModel:      os.Getenv("EMBEDDING_MODEL"),
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
