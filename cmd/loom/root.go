package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/disbeliefff/loom/internal/models"
	"github.com/disbeliefff/loom/pkg/logger"
	"github.com/spf13/cobra"
)

type app struct {
	configPath   string
	servicesPath string
	outputPath   string
	repoRoot     string
	debug        bool

	logger *slog.Logger
}

func newApp() *app {
	return &app{}
}

func newRootCommand(a *app) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "loom",
		Short:         "loom is a tool for dynamically generating GitLab CI child pipelines",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			a.logger = logger.New(a.debug)
		},
	}

	rootCmd.AddCommand(newGenerateCommand(a))
	rootCmd.AddCommand(newValidateCommand(a))

	return rootCmd
}

func runExecute() {
	a := newApp()
	cmd := newRootCommand(a)
	if err := cmd.Execute(); err != nil {
		if a.logger != nil {
			a.logger.Error("execution failed", "error", err)
		} else {
			slog.Error("execution failed", "error", err)
		}
		os.Exit(1)
	}
}

func executeWithArgs(a *app, args []string, stdout, stderr io.Writer) error {
	cmd := newRootCommand(a)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd.Execute()
}

func (a *app) getPipelineContext() (models.PipelineContext, error) {
	resolvedRepoRoot := a.repoRoot
	if resolvedRepoRoot == "" {
		resolvedRepoRoot = os.Getenv("CI_PROJECT_DIR")
	}
	if resolvedRepoRoot == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return models.PipelineContext{}, fmt.Errorf("get current working directory: %w", err)
		}
		resolvedRepoRoot = pwd
	}

	return models.PipelineContext{
		CommitTag:      os.Getenv("CI_COMMIT_TAG"),
		CommitBranch:   os.Getenv("CI_COMMIT_BRANCH"),
		PipelineID:     os.Getenv("CI_PIPELINE_ID"),
		PipelineSource: os.Getenv("CI_PIPELINE_SOURCE"),
		BeforeSHA:      os.Getenv("CI_COMMIT_BEFORE_SHA"),
		CommitSHA:      os.Getenv("CI_COMMIT_SHA"),
		BuildTag:       os.Getenv("BUILD_TAG"),
		RepoRoot:       resolvedRepoRoot,
	}, nil
}
