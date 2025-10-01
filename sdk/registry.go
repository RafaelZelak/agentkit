package sdk

import "github.com/RafaelZelak/agentkit/internal/tools"

func RegisterScript(name string, fn func(args ...string) (string, error)) {
	tools.RegisterScript(name, fn)
}
