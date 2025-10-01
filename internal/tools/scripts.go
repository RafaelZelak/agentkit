package tools

import (
	"fmt"
	"strings"
)

// ScriptRegistry armazena as funções Go registradas
// disponíveis para execução como Tools.
var ScriptRegistry = map[string]func(args ...string) (string, error){}

// RegisterScript adiciona uma função Go ao ScriptRegistry.
//
// Deve ser chamado dentro de init() em um pacote de scripts,
// normalmente importado com import anônimo (`_ "meuprojeto/scripts"`).
func RegisterScript(name string, fn func(args ...string) (string, error)) {
	ScriptRegistry[name] = fn
}

// ExecScript resolve a chamada de função configurada no tools.yml
// e executa a função Go correspondente do ScriptRegistry.
func ExecScript(cfg ToolConfig, args ...string) (string, error) {
	fnDecl := cfg.Function
	for i, arg := range args {
		ph := fmt.Sprintf("$%d", i+1)
		fnDecl = strings.ReplaceAll(fnDecl, ph, arg)
	}

	// Extrair o nome da função, ex: "CalcJuros($1, $2)" -> "CalcJuros"
	fnName := strings.SplitN(fnDecl, "(", 2)[0]
	fnName = strings.TrimSpace(fnName)

	fn, ok := ScriptRegistry[fnName]
	if !ok {
		return "", fmt.Errorf("função '%s' não registrada no ScriptRegistry", fnName)
	}

	return fn(args...)
}
