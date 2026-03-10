package tools

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	tools []ToolConfig
}

func NewRegistry(path string) (*Registry, error) {
	if path == "" {
		return &Registry{tools: []ToolConfig{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return NewRegistryFromData(data)
}

func NewRegistryFromData(data []byte) (*Registry, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for i := range cfg.Tools {
		if strings.HasPrefix(cfg.Tools[i].Conn, "ENV:") {
			envKey := strings.TrimPrefix(cfg.Tools[i].Conn, "ENV:")
			cfg.Tools[i].Conn = os.Getenv(envKey)
		}
	}

	return &Registry{tools: cfg.Tools}, nil
}

func NewRegistryFromConfig(tools []ToolConfig) *Registry {
	if tools == nil {
		tools = []ToolConfig{}
	}
	return &Registry{tools: tools}
}

func (r *Registry) Get(name string) *ToolConfig {
	for i := range r.tools {
		if r.tools[i].Name == name {
			return &r.tools[i]
		}
	}
	return nil
}

func (r *Registry) List() []ToolConfig {
	return r.tools
}

func (r *Registry) Add(tool ToolConfig) {
	r.tools = append(r.tools, tool)
}
