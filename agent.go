package agentkit

import (
	"context"

	"github.com/RafaelZelak/agentkit/internal/agent"
	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"
)

type Agent struct {
	cli     *openai.Client
	cfg     *Config
	verbose bool
}

// NewAgent inicializa memória, tools e cliente OpenAI
func NewAgent(cfg *Config, verbose bool) (*Agent, error) {
	if err := tools.LoadTools(cfg.ToolsPath); err != nil {
		return nil, err
	}

	if _, err := memory.Init(memory.Config{
		DSN:          cfg.DSN,
		Schema:       cfg.Schema,
		EmbeddingDim: cfg.EmbeddingDim,
	}); err != nil {
		return nil, err
	}

	cli := openai.NewClient(cfg.APIKey)

	return &Agent{
		cli:     cli,
		cfg:     cfg,
		verbose: verbose,
	}, nil
}

// Run executa um agente SEM roteador
func (a *Agent) Run(ctx context.Context, sessionID, basePromptPath, userMessage string) (string, error) {
	return agent.Run(
		ctx,
		a.cli,
		a.cfg.GPTModel,
		a.cfg.EmbModel,
		sessionID,
		basePromptPath,
		userMessage,
		a.verbose,
	)
}

// RouteAndRun executa um agente COM roteador
func (a *Agent) RouteAndRun(ctx context.Context, sessionID, basePromptPath, userMessage, routerPath string) (string, error) {
	return agent.RouteAndRun(
		ctx,
		a.cli,
		a.cfg.GPTModel,
		a.cfg.EmbModel,
		sessionID,
		basePromptPath,
		userMessage,
		routerPath,
		a.verbose,
	)
}
