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

	PromptsDir    string `yaml:"prompts_dir"`
	BasePrompt    string `yaml:"base_prompt"`
	RouterPrompt  string `yaml:"router_prompt"`
	ToolsPath     string `yaml:"tools_path"`
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
		if err := validateAndSetDefaults(&cfg.Agents[i]); err != nil {
			return nil, fmt.Errorf("agent[%d]: %w", i, err)
		}
	}

	return &cfg, nil
}

func validateAndSetDefaults(agentCfg *Config) error {
	agentName := agentCfg.Name
	if agentName == "" {
		return fmt.Errorf("name is required")
	}

	// Validate required fields
	if agentCfg.APIKey == "" {
		return fmt.Errorf("api_key is required for agent '%s'", agentName)
	}
	if agentCfg.GPTModel == "" {
		return fmt.Errorf("gpt_model is required for agent '%s'", agentName)
	}
	if agentCfg.DSN == "" {
		return fmt.Errorf("dsn is required for agent '%s'", agentName)
	}
	if agentCfg.Schema == "" {
		return fmt.Errorf("schema is required for agent '%s'", agentName)
	}
	if agentCfg.PromptsDir == "" {
		return fmt.Errorf("prompts_dir is required for agent '%s'", agentName)
	}
	if agentCfg.BasePrompt == "" {
		return fmt.Errorf("base_prompt is required for agent '%s'", agentName)
	}

	// Set defaults for optional fields
	if agentCfg.Timeout == 0 {
		agentCfg.Timeout = 60 * time.Second
	}
	if agentCfg.EmbModel == "" {
		agentCfg.EmbModel = "text-embedding-3-small"
	}
	if agentCfg.EmbeddingDim == 0 {
		agentCfg.EmbeddingDim = 1536
	}

	return nil
}

func LoadAgents(path string) (*AgentManager, error) {
	cfg, err := NewConfigFromYAML(path)
	if err != nil {
		return nil, err
	}

	manager := NewAgentManager()
	if _, err := LoadAgentsFromConfig(manager, cfg.Agents); err != nil {
		return nil, err
	}
	return manager, nil
}

// LoadAgentsFromJSON allows loading configs dynamically via JSON payload (useful for SaaS DB configs)
func LoadAgentsFromJSON(manager *AgentManager, data []byte) (*AgentManager, error) {
	var cfg MultiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil { // Unmarshals JSON natively via gopkg yaml
		return nil, fmt.Errorf("failed to unmarshal JSON configuration: %w", err)
	}

	for i := range cfg.Agents {
		resolveConfigEnvVars(&cfg.Agents[i])
		if err := validateAndSetDefaults(&cfg.Agents[i]); err != nil {
			return nil, fmt.Errorf("agent[%d]: %w", i, err)
		}
	}
	return LoadAgentsFromConfig(manager, cfg.Agents)
}

// LoadAgentsFromConfig loads an array of configurations into the Manager.
func LoadAgentsFromConfig(manager *AgentManager, agentsCfg []Config) (*AgentManager, error) {
	if manager == nil {
		manager = NewAgentManager()
	}

	for _, agentCfg := range agentsCfg {
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
