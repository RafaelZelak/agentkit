package agentkit

import (
	"context"

	"github.com/RafaelZelak/agentkit/internal/agent"
	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"
)

// Agent representa uma instância de agente de IA.
// Cada agente tem sua própria configuração, memória e tools.
type Agent struct {
	name    string
	cli     *openai.Client
	cfg     *Config
	memory  *memory.Store
	tools   *tools.Registry
	verbose bool
}

// NewAgent cria um novo agente com a configuração fornecida.
func NewAgent(cfg *Config) (*Agent, error) {
	// Criar registry de tools específico para este agente
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

	// Criar store de memória específico para este agente
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

// Chat envia uma mensagem e usa router automaticamente se configurado.
func (a *Agent) Chat(ctx context.Context, sessionID, message string) (string, error) {
	if a.cfg.RouterPrompt != "" {
		return a.RouteAndRun(ctx, sessionID, a.cfg.BasePrompt, message, a.cfg.RouterPrompt)
	}
	return a.Run(ctx, sessionID, a.cfg.BasePrompt, message)
}

// ChatSimple envia uma mensagem usando apenas o prompt base (ignora router).
func (a *Agent) ChatSimple(ctx context.Context, sessionID, message string) (string, error) {
	return a.Run(ctx, sessionID, a.cfg.BasePrompt, message)
}

// Run executa o agente com um prompt específico.
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

// RouteAndRun executa o agente com roteamento de prompts.
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

// Close libera recursos do agente.
func (a *Agent) Close() error {
	if a.memory != nil {
		return a.memory.Close()
	}
	return nil
}

// Name retorna o identificador do agente.
func (a *Agent) Name() string {
	return a.name
}

// Config retorna a configuração do agente.
func (a *Agent) Config() *Config {
	return a.cfg
}

// Memory retorna o store de memória do agente.
func (a *Agent) Memory() *memory.Store {
	return a.memory
}

// Tools retorna o registry de tools do agente.
func (a *Agent) Tools() *tools.Registry {
	return a.tools
}
