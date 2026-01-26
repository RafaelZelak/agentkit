package agentkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RafaelZelak/agentkit/internal/agent"
	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"
)

type Agent struct {
	name    string
	cli     *openai.Client
	cfg     *Config
	memory  *memory.Store
	tools   *tools.Registry
	verbose bool
}

func NewAgent(cfg *Config) (*Agent, error) {
	// Validate functions_path if provided (optional)
	if cfg.FunctionsPath != "" {
		funcsPath := cfg.FunctionsPath
		if !filepath.IsAbs(funcsPath) {
			// If relative path, make it relative to current working directory
			wd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get working directory: %w", err)
			}
			funcsPath = filepath.Join(wd, funcsPath)
		}
		
		info, err := os.Stat(funcsPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("functions_path '%s' does not exist for agent '%s'", cfg.FunctionsPath, cfg.Name)
			}
			return nil, fmt.Errorf("failed to check functions_path '%s': %w", cfg.FunctionsPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("functions_path '%s' is not a directory for agent '%s'", cfg.FunctionsPath, cfg.Name)
		}
	}

	var toolReg *tools.Registry
	var err error
	if cfg.ToolsPath != "" {
		toolReg, err = tools.NewRegistry(cfg.ToolsPath)
		if err != nil {
			return nil, err
		}
	} else {
		toolReg = tools.NewRegistryFromConfig(nil)
	}

	memStore, err := memory.NewStore(memory.Config{
		DSN:          cfg.DSN,
		Schema:       cfg.Schema,
		EmbeddingDim: cfg.EmbeddingDim,
	})
	if err != nil {
		return nil, err
	}

	return &Agent{
		name:    cfg.Name,
		cli:     openai.NewClient(cfg.APIKey),
		cfg:     cfg,
		memory:  memStore,
		tools:   toolReg,
		verbose: cfg.Verbose,
	}, nil
}

func (a *Agent) Chat(ctx context.Context, sessionID, message string) (string, error) {
	if a.cfg.RouterPrompt != "" {
		return a.RouteAndRun(ctx, sessionID, a.cfg.BasePrompt, message, a.cfg.RouterPrompt)
	}
	return a.Run(ctx, sessionID, a.cfg.BasePrompt, message)
}

func (a *Agent) ChatSimple(ctx context.Context, sessionID, message string) (string, error) {
	return a.Run(ctx, sessionID, a.cfg.BasePrompt, message)
}

func (a *Agent) Run(ctx context.Context, sessionID, basePromptPath, userMessage string) (string, error) {
	out, _, err := agent.Run(
		ctx,
		a.cli,
		a.cfg.GPTModel,
		a.cfg.EmbModel,
		sessionID,
		basePromptPath,
		userMessage,
		a.verbose,
		a.memory,
		a.tools,
	)
	return out, err
}

func (a *Agent) RouteAndRun(ctx context.Context, sessionID, basePromptPath, userMessage, routerPath string) (string, error) {
	out, _, err := agent.RouteAndRun(
		ctx,
		a.cli,
		a.cfg.GPTModel,
		a.cfg.EmbModel,
		sessionID,
		basePromptPath,
		userMessage,
		routerPath,
		a.verbose,
		a.memory,
		a.tools,
	)
	return out, err
}

func (a *Agent) Close() error {
	if a.memory != nil {
		return a.memory.Close()
	}
	return nil
}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Config() *Config {
	return a.cfg
}

func (a *Agent) Memory() *memory.Store {
	return a.memory
}

func (a *Agent) Tools() *tools.Registry {
	return a.tools
}
