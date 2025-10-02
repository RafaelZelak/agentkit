package tools

import (
	"fmt"
	"strings"
)

var ScriptRegistry = map[string]func(args ...string) (string, error){}

func RegisterScript(name string, fn func(args ...string) (string, error)) {
	ScriptRegistry[name] = fn
}

func ExecScript(cfg ToolConfig, args ...string) (string, error) {
	fnDecl := cfg.Function
	for i, arg := range args {
		ph := fmt.Sprintf("$%d", i+1)
		fnDecl = strings.ReplaceAll(fnDecl, ph, arg)
	}

	fnName := strings.SplitN(fnDecl, "(", 2)[0]
	fnName = strings.TrimSpace(fnName)

	fn, ok := ScriptRegistry[fnName]
	if !ok {
		return "", fmt.Errorf("função '%s' não registrada no ScriptRegistry", fnName)
	}

	return fn(args...)
}
