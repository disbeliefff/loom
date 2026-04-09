package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteReturnsConfigLoadErrors(t *testing.T) {
	t.Run("generate returns missing config error without panicking", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		var err error
		require.NotPanics(t, func() {
			err = execute([]string{
				"generate",
				"--config", "./missing.yaml",
				"--services", "./services.json",
			}, stdout, stderr)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
		assert.Empty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("validate returns missing services error without panicking", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		var err error
		require.NotPanics(t, func() {
			err = execute([]string{
				"validate",
				"--config", "../../example/pipeline-strategies.yaml",
				"--services", "./missing-services.json",
			}, stdout, stderr)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read services file")
		assert.Empty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})
}
