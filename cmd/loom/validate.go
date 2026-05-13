package main

import (
	"fmt"
	"os"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/disbeliefff/loom/internal/engine"
	"github.com/disbeliefff/loom/internal/git"
	"github.com/disbeliefff/loom/internal/selector"
	"github.com/spf13/cobra"
)

func newValidateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validates the configuration and templates",
		RunE:  a.runValidate,
	}

	cmd.Flags().StringVarP(&a.configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	cmd.Flags().StringVarP(&a.servicesPath, "services", "s", "services.json", "Path to services.json")
	cmd.Flags().StringVarP(&a.repoRoot, "repo-root", "r", "", "Explicit Git repository root path (default: $CI_PROJECT_DIR or cwd)")
	cmd.Flags().BoolVarP(&a.debug, "debug", "d", false, "Enable debug logging")

	return cmd
}

func (a *app) runValidate(_ *cobra.Command, _ []string) error {
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
