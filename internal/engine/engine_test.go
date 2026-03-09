package engine_test

import (
	"os"
	"testing"

	"github.com/disbeliefff/loom/internal/engine"
)

func TestEvaluateCondition(t *testing.T) {
	os.Setenv("TEST_VAR", "hello")
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("TEST_BOOL")

	tests := []struct {
		name      string
		condition string
		want      bool
		wantErr   bool
	}{
		{
			name:      "Empty condition should return true",
			condition: "",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "Simple true boolean",
			condition: "true",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "Simple false boolean",
			condition: "false",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "Evaluate environment variable string equality",
			condition: `env("TEST_VAR") == "hello"`,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "Evaluate environment variable string inequality",
			condition: `env("TEST_VAR") == "world"`,
			want:      false,
			wantErr:   false,
		},
		{
			name:      "Evaluate environment variable containing true",
			condition: `env("TEST_BOOL") == "true"`,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "Invalid condition syntax",
			condition: `env("TEST_VAR" == "hello"`, // missing closing parenthesis
			want:      false,
			wantErr:   true,
		},
		{
			name:      "Condition returning non-boolean",
			condition: `"hello world"`,
			want:      false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.EvaluateCondition(tt.condition)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateCondition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EvaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}
