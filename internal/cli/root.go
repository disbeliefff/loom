package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/disbeliefff/loom/internal/engine"
	"github.com/disbeliefff/loom/internal/git"
	"github.com/disbeliefff/loom/internal/models"
	"github.com/disbeliefff/loom/internal/selector"
	"github.com/disbeliefff/loom/internal/template"
	"github.com/disbeliefff/loom/pkg/logger"
	"github.com/spf13/cobra"
)

type App struct {
	configPath   string
	servicesPath string
	outputPath   string
	repoRoot     string
	debug        bool

	logger *slog.Logger
}

func NewApp() *App {
	return &App{}
}

func (a *App) NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "loom",
		Short:         "loom is a tool for dynamically generating GitLab CI child pipelines",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			a.logger = logger.New(a.debug)
		},
	}

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generates the pipeline based on strategies and services",
		RunE:  a.runGenerate,
	}

	generateCmd.Flags().StringVarP(&a.configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	generateCmd.Flags().StringVarP(&a.servicesPath, "services", "s", "services.json", "Path to services.json")
	generateCmd.Flags().StringVarP(&a.outputPath, "out", "o", "", "Output file path (default is stdout)")
	generateCmd.Flags().StringVarP(&a.repoRoot, "repo-root", "r", "", "Explicit Git repository root path (default: $CI_PROJECT_DIR or cwd)")
	generateCmd.Flags().BoolVarP(&a.debug, "debug", "d", false, "Enable debug logging")

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validates the configuration and templates",
		RunE:  a.runValidate,
	}

	validateCmd.Flags().StringVarP(&a.configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	validateCmd.Flags().StringVarP(&a.servicesPath, "services", "s", "services.json", "Path to services.json")
	validateCmd.Flags().StringVarP(&a.repoRoot, "repo-root", "r", "", "Explicit Git repository root path (default: $CI_PROJECT_DIR or cwd)")
	validateCmd.Flags().BoolVarP(&a.debug, "debug", "d", false, "Enable debug logging")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(validateCmd)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	app := NewApp()
	if err := app.execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if app.logger != nil {
			app.logger.Error("execution failed", "error", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func (a *App) execute(args []string, stdout, stderr io.Writer) error {
	cmd := a.NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd.Execute()
}

func (a *App) runGenerate(_ *cobra.Command, _ []string) error {
	a.logger.Debug("Loading configurations", "config", a.configPath, "services", a.servicesPath)

	cfg, err := config.Load(a.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	services, err := config.LoadServices(a.servicesPath)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	ctx, err := a.getPipelineContext()
	if err != nil {
		return fmt.Errorf("get pipeline context: %w", err)
	}

	a.logger.Debug("Pipeline Context initialized", "context", fmt.Sprintf("%+v", ctx))

	evaluator := selector.NewEvaluator(a.logger, git.NewClient(a.logger))

	for _, strategy := range cfg.Strategies {
		a.logger.Debug("Evaluating strategy", "strategy", strategy.Name, "condition", strategy.Condition)
		matched, err := engine.EvaluateCondition(strategy.Condition)
		if err != nil {
			a.logger.Error("Failed to evaluate condition", "strategy", strategy.Name, "error", err)
			continue
		}

		if matched {
			a.logger.Info("Strategy matched", "strategy", strategy.Name)

			jobs, err := evaluator.Apply(strategy.Selector, ctx, services)
			if err != nil {
				return fmt.Errorf("apply selector for strategy %q: %w", strategy.Name, err)
			}

			a.logger.Info("Selected jobs", "count", len(jobs))

			err = template.RenderPipeline(strategy.Template, jobs, ctx, a.outputPath)
			if err != nil {
				return fmt.Errorf("render pipeline: %w", err)
			}
			return nil
		}
	}

	a.logger.Warn("No strategies matched the current environment")
	if a.outputPath != "" {
		fallback := "stages:\n  - no-op\nno-op:\n  stage: no-op\n  script:\n    - echo 'No strategies matched'"
		return os.WriteFile(a.outputPath, []byte(fallback), 0600)
	}
	fmt.Println("No strategies matched")
	return nil
}

func (a *App) runValidate(_ *cobra.Command, _ []string) error {
	a.logger.Info("Validating configurations", "config", a.configPath, "services", a.servicesPath)

	cfg, err := config.Load(a.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	services, err := config.LoadServices(a.servicesPath)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	ctx, err := a.getPipelineContext()
	if err != nil {
		return fmt.Errorf("get pipeline context: %w", err)
	}

	evaluator := selector.NewEvaluator(a.logger, git.NewClient(a.logger))

	for _, strategy := range cfg.Strategies {
		if strategy.Name == "" {
			return fmt.Errorf("strategy is missing a name")
		}
		if strategy.Template == "" {
			return fmt.Errorf("strategy '%s' is missing a template path", strategy.Name)
		}

		if strategy.Condition != "" {
			_, err := engine.EvaluateCondition(strategy.Condition)
			if err != nil {
				a.logger.Warn("Condition evaluation failed during validation (may be expected if missing env vars)", "strategy", strategy.Name, "error", err)
			}
		}

		_, err := evaluator.Apply(strategy.Selector, ctx, services)
		if err != nil {
			a.logger.Warn("Selector config validation issue (may rely on git repo context)", "strategy", strategy.Name, "error", err)
		}

		if _, err := os.Stat(strategy.Template); os.IsNotExist(err) {
			a.logger.Warn("Template file not found", "strategy", strategy.Name, "template", strategy.Template)
		}
	}

	a.logger.Info("Validation successful")
	return nil
}

func (a *App) getPipelineContext() (models.PipelineContext, error) {
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
