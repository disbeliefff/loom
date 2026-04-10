package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/disbeliefff/loom/internal/engine"
	"github.com/disbeliefff/loom/internal/models"
	"github.com/disbeliefff/loom/internal/selector"
	"github.com/disbeliefff/loom/internal/template"
	"github.com/disbeliefff/loom/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	configPath   string
	servicesPath string
	outputPath   string
	repoRoot     string
	debug        bool
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "loom",
		Short:         "loom is a tool for dynamically generating GitLab CI child pipelines",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generates the pipeline based on strategies and services",
		RunE:  runGenerate,
	}

	generateCmd.Flags().StringVarP(&configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	generateCmd.Flags().StringVarP(&servicesPath, "services", "s", "services.json", "Path to services.json")
	generateCmd.Flags().StringVarP(&outputPath, "out", "o", "", "Output file path (default is stdout)")
	generateCmd.Flags().StringVarP(&repoRoot, "repo-root", "r", "", "Explicit Git repository root path (default: $CI_PROJECT_DIR or cwd)")
	generateCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validates the configuration and templates",
		RunE:  runValidate,
	}

	validateCmd.Flags().StringVarP(&configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	validateCmd.Flags().StringVarP(&servicesPath, "services", "s", "services.json", "Path to services.json")
	validateCmd.Flags().StringVarP(&repoRoot, "repo-root", "r", "", "Explicit Git repository root path (default: $CI_PROJECT_DIR or cwd)")
	validateCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(validateCmd)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		slog.Error("execution failed", "error", err)
		os.Exit(1)
	}
}

func execute(args []string, stdout, stderr io.Writer) error {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd.Execute()
}

func runGenerate(_ *cobra.Command, _ []string) error {
	logger.Init(debug)

	slog.Debug("Loading configurations", "config", configPath, "services", servicesPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	services, err := config.LoadServices(servicesPath)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	ctx, err := getPipelineContext()
	if err != nil {
		return fmt.Errorf("get pipeline context: %w", err)
	}

	slog.Debug("Pipeline Context initialized", "context", fmt.Sprintf("%+v", ctx))

	for _, strategy := range cfg.Strategies {
		slog.Debug("Evaluating strategy", "strategy", strategy.Name, "condition", strategy.Condition)
		matched, err := engine.EvaluateCondition(strategy.Condition)
		if err != nil {
			slog.Error("Failed to evaluate condition", "strategy", strategy.Name, "error", err)
			continue
		}

		if matched {
			slog.Info("Strategy matched", "strategy", strategy.Name)

			jobs, err := selector.Apply(strategy.Selector, ctx, services)
			if err != nil {
				return fmt.Errorf("apply selector for strategy %q: %w", strategy.Name, err)
			}

			slog.Info("Selected jobs", "count", len(jobs))

			err = template.RenderPipeline(strategy.Template, jobs, ctx, outputPath)
			if err != nil {
				return fmt.Errorf("render pipeline: %w", err)
			}
			return nil
		}
	}

	slog.Warn("No strategies matched the current environment")
	if outputPath != "" {
		fallback := "stages:\n  - no-op\nno-op:\n  stage: no-op\n  script:\n    - echo 'No strategies matched'"
		return os.WriteFile(outputPath, []byte(fallback), 0600)
	}
	fmt.Println("No strategies matched")
	return nil
}

func runValidate(_ *cobra.Command, _ []string) error {
	logger.Init(debug)

	slog.Info("Validating configurations", "config", configPath, "services", servicesPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	services, err := config.LoadServices(servicesPath)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}
	ctx, err := getPipelineContext()
	if err != nil {
		return fmt.Errorf("get pipeline context: %w", err)
	}

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
				slog.Warn("Condition evaluation failed during validation (may be expected if missing env vars)", "strategy", strategy.Name, "error", err)
			}
		}

		_, err := selector.Apply(strategy.Selector, ctx, services)
		if err != nil {
			slog.Warn("Selector config validation issue (may rely on git repo context)", "strategy", strategy.Name, "error", err)
		}

		if _, err := os.Stat(strategy.Template); os.IsNotExist(err) {
			slog.Warn("Template file not found", "strategy", strategy.Name, "template", strategy.Template)
		}
	}

	slog.Info("Validation successful")
	return nil
}

func getPipelineContext() (models.PipelineContext, error) {
	resolvedRepoRoot := repoRoot
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
