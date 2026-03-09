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
		return false, fmt.Errorf("failed to compile condition '%s': %w", condition, err)
	}

	result, err := expr.Run(program, envMap)
	if err != nil {
		return false, fmt.Errorf("failed to run condition '%s': %w", condition, err)
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("condition '%s' did not return a boolean value", condition)
	}

	return boolResult, nil
}
