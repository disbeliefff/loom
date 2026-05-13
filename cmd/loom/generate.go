package main

import (
	"fmt"
	"os"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/disbeliefff/loom/internal/engine"
	"github.com/disbeliefff/loom/internal/git"
	"github.com/disbeliefff/loom/internal/selector"
	"github.com/disbeliefff/loom/internal/template"
	"github.com/spf13/cobra"
)

func newGenerateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generates the pipeline based on strategies and services",
		RunE:  a.runGenerate,
	}

	cmd.Flags().StringVarP(&a.configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	cmd.Flags().StringVarP(&a.servicesPath, "services", "s", "services.json", "Path to services.json")
	cmd.Flags().StringVarP(&a.outputPath, "out", "o", "", "Output file path (default is stdout)")
	cmd.Flags().StringVarP(&a.repoRoot, "repo-root", "r", "", "Explicit Git repository root path (default: $CI_PROJECT_DIR or cwd)")
	cmd.Flags().BoolVarP(&a.debug, "debug", "d", false, "Enable debug logging")

	return cmd
}

func (a *app) runGenerate(_ *cobra.Command, _ []string) error {
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
