package agentkit

import (
	"context"
	"fmt"
	"sync"
)

type AgentManager struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents: make(map[string]*Agent),
	}
}

func (m *AgentManager) Register(cfg *Config) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.Name == "" {
		return nil, fmt.Errorf("agent name is required")
	}

	if _, exists := m.agents[cfg.Name]; exists {
		return nil, fmt.Errorf("agent '%s' already registered", cfg.Name)
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent '%s': %w", cfg.Name, err)
	}

	m.agents[cfg.Name] = agent
	return agent, nil
}

func (m *AgentManager) Get(name string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, exists := m.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent '%s' not found", name)
	}
	return agent, nil
}

func (m *AgentManager) GetOrCreate(cfg *Config) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent, exists := m.agents[cfg.Name]; exists {
		return agent, nil
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		return nil, err
	}

	m.agents[cfg.Name] = agent
	return agent, nil
}

func (m *AgentManager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[name]
	if !exists {
		return fmt.Errorf("agent '%s' not found", name)
	}

	if err := agent.Close(); err != nil {
		return err
	}

	delete(m.agents, name)
	return nil
}

func (m *AgentManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.agents))
	for name := range m.agents {
		names = append(names, name)
	}
	return names
}

func (m *AgentManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, agent := range m.agents {
		if err := agent.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close agent '%s': %w", name, err))
		}
	}
	m.agents = make(map[string]*Agent)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing agents: %v", errs)
	}
	return nil
}

func (m *AgentManager) Chat(agentName, sessionID, message string) (string, error) {
	agent, err := m.Get(agentName)
	if err != nil {
		return "", err
	}
	return agent.Chat(context.Background(), sessionID, message)
}

func (m *AgentManager) ChatCtx(ctx context.Context, agentName, sessionID, message string) (string, error) {
	agent, err := m.Get(agentName)
	if err != nil {
		return "", err
	}
	return agent.Chat(ctx, sessionID, message)
}

func (m *AgentManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}
