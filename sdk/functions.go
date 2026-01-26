package sdk

import (
	"github.com/RafaelZelak/agentkit/internal/functions"
)

// RegisterFunction registers a function that can be called from templates
// name should be in format "package.function" (e.g., "time.now", "math.add")
// fn should accept variadic interface{} arguments and return (string, error)
func RegisterFunction(name string, fn functions.FunctionType) {
	functions.RegisterFunction(name, fn)
}

// RegisterGoFunction registers a Go function with any signature
// It automatically wraps the function to match the FunctionType interface
// Examples:
//   - func() string
//   - func() (string, error)
//   - func(name string) string
//   - func(a, b int) (string, error)
func RegisterGoFunction(name string, fn interface{}) error {
	return functions.RegisterGoFunction(name, fn)
}
