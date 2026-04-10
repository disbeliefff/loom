package engine

import (
	"fmt"
	"os"

	"github.com/expr-lang/expr"
)

// EvaluateCondition parses and runs the given condition using expr engine
func EvaluateCondition(condition string) (bool, error) {
	if condition == "" {
		return true, nil // Empty condition is always true
	}

	envMap := map[string]any{
		"env": func(key string) string {
			return os.Getenv(key)
		},
	}

	program, err := expr.Compile(condition, expr.Env(envMap))
	if err != nil {
		return false, fmt.Errorf("compile condition %q: %w", condition, err)
	}

	result, err := expr.Run(program, envMap)
	if err != nil {
		return false, fmt.Errorf("run condition %q: %w", condition, err)
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("condition %q did not return a boolean value", condition)
	}

	return boolResult, nil
}
