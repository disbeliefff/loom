package main

import (
	"bytes"
	"testing"

	"github.com/disbeliefff/loom/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCommand(t *testing.T) {
	a := newApp()
	root := newRootCommand(a)

	assert.Equal(t, "loom", root.Use)
	assert.Equal(t, "generate", root.Commands()[0].Use)
	assert.Equal(t, "validate", root.Commands()[1].Use)
}

func TestGenerateCommandFlags(t *testing.T) {
	a := newApp()
	root := newRootCommand(a)

	genCmd, _, err := root.Find([]string{"generate"})
	require.NoError(t, err)

	flags := genCmd.Flags()
	assert.NotNil(t, flags.Lookup("config"))
	assert.NotNil(t, flags.Lookup("services"))
	assert.NotNil(t, flags.Lookup("out"))
	assert.NotNil(t, flags.Lookup("repo-root"))
	assert.NotNil(t, flags.Lookup("debug"))
}

func TestValidateCommandFlags(t *testing.T) {
	a := newApp()
	root := newRootCommand(a)

	valCmd, _, err := root.Find([]string{"validate"})
	require.NoError(t, err)

	flags := valCmd.Flags()
	assert.NotNil(t, flags.Lookup("config"))
	assert.NotNil(t, flags.Lookup("services"))
	assert.NotNil(t, flags.Lookup("repo-root"))
	assert.NotNil(t, flags.Lookup("debug"))
	assert.Nil(t, flags.Lookup("out"))
}

func TestExecuteReturnsConfigLoadErrors(t *testing.T) {
	t.Run("generate returns missing config error without panicking", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		var err error
		require.NotPanics(t, func() {
			a := newApp()
			a.logger = logger.New(false)
			err = executeWithArgs(a, []string{
				"generate",
				"--config", "./missing.yaml",
				"--services", "./services.json",
			}, stdout, stderr)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read config file")
		assert.Empty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("validate returns missing services error without panicking", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		var err error
		require.NotPanics(t, func() {
			a := newApp()
			a.logger = logger.New(false)
			err = executeWithArgs(a, []string{
				"validate",
				"--config", "../../example/pipeline-strategies.yaml",
				"--services", "./missing-services.json",
			}, stdout, stderr)
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read services file")
		assert.Empty(t, stdout.String())
		assert.Empty(t, stderr.String())
	})
}
