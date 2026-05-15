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

	cmdNames := make(map[string]bool)
	for _, cmd := range root.Commands() {
		cmdNames[cmd.Use] = true
	}
	assert.True(t, cmdNames["generate"])
	assert.True(t, cmdNames["validate"])
	assert.True(t, cmdNames["gitops"])
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

func TestGitOpsSubcommandsExist(t *testing.T) {
	a := newApp()
	root := newRootCommand(a)

	gitopsCmd, _, err := root.Find([]string{"gitops"})
	require.NoError(t, err)

	subNames := make(map[string]bool)
	for _, cmd := range gitopsCmd.Commands() {
		subNames[cmd.Use] = true
	}
	assert.True(t, subNames["promote"])
	assert.True(t, subNames["rollback"])
}

func TestPromoteCommandFlags(t *testing.T) {
	a := newApp()
	root := newRootCommand(a)

	promoteCmd, _, err := root.Find([]string{"gitops", "promote"})
	require.NoError(t, err)

	flags := promoteCmd.Flags()
	assert.NotNil(t, flags.Lookup("config"))
	assert.NotNil(t, flags.Lookup("services"))
	assert.NotNil(t, flags.Lookup("strategy"))
	assert.NotNil(t, flags.Lookup("service"))
	assert.NotNil(t, flags.Lookup("tag"))
	assert.NotNil(t, flags.Lookup("dry-run"))
	assert.NotNil(t, flags.Lookup("debug"))
}

func TestRollbackCommandFlags(t *testing.T) {
	a := newApp()
	root := newRootCommand(a)

	rollbackCmd, _, err := root.Find([]string{"gitops", "rollback"})
	require.NoError(t, err)

	flags := rollbackCmd.Flags()
	assert.NotNil(t, flags.Lookup("config"))
	assert.NotNil(t, flags.Lookup("services"))
	assert.NotNil(t, flags.Lookup("strategy"))
	assert.NotNil(t, flags.Lookup("service"))
	assert.NotNil(t, flags.Lookup("dry-run"))
	assert.NotNil(t, flags.Lookup("debug"))
	assert.Nil(t, flags.Lookup("tag"))
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
