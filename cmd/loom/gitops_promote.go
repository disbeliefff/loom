package main

import (
	"fmt"
	"path/filepath"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/disbeliefff/loom/internal/git"
	"github.com/disbeliefff/loom/internal/promotion"
	"github.com/spf13/cobra"
)

func newGitOpsPromoteCommand(a *app) (*cobra.Command, error) {
	var (
		strategyName string
		serviceName  string
		tag          string
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a service image tag to a GitOps target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runGitOpsPromote(strategyName, serviceName, tag, dryRun)
		},
	}

	cmd.Flags().StringVarP(&a.configPath, "config", "c", "pipeline-strategies.yaml", "Path to pipeline-strategies.yaml")
	cmd.Flags().StringVarP(&a.servicesPath, "services", "s", "services.json", "Path to services.json")
	cmd.Flags().StringVar(&strategyName, "strategy", "", "Strategy name with promotion config")
	cmd.Flags().StringVar(&serviceName, "service", "", "Service key (optional if only one service)")
	cmd.Flags().StringVar(&tag, "tag", "", "Image tag to promote")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show changes without committing")
	cmd.Flags().BoolVarP(&a.debug, "debug", "d", false, "Enable debug logging")

	if err := cmd.MarkFlagRequired("strategy"); err != nil {
		return nil, err
	}
	if err := cmd.MarkFlagRequired("tag"); err != nil {
		return nil, err
	}

	return cmd, nil
}

func (a *app) runGitOpsPromote(strategyName, serviceName, tag string, dryRun bool) error {
	a.logger.Debug("loading configurations", "config", a.configPath, "services", a.servicesPath)

	cfg, err := config.Load(a.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	services, err := config.LoadServices(a.servicesPath)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}

	strategy, err := promotion.FindStrategy(cfg, strategyName)
	if err != nil {
		return err
	}
	if strategy.Promotion == nil || !strategy.Promotion.Enabled {
		return fmt.Errorf("strategy %q does not have promotion enabled", strategyName)
	}

	promotion.ApplyDefaults(strategy.Promotion)

	service, err := promotion.ResolveService(services, serviceName)
	if err != nil {
		return err
	}

	target, err := promotion.Resolve(strategy.Promotion, *service)
	if err != nil {
		return fmt.Errorf("resolve promotion target: %w", err)
	}

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(target.CheckoutPath)
	if err != nil {
		return fmt.Errorf("find manifest: %w", err)
	}

	result, err := mutator.Promote(found, tag)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}

	a.logger.Info("promoted",
		"service", service.Key,
		"target", target.Target,
		"file", result.FilePath,
		"repository", fmt.Sprintf("%s → %s", result.OldRepo, result.NewRepo),
		"tag", fmt.Sprintf("%s → %s", result.OldTag, result.NewTag),
	)

	if dryRun {
		a.logger.Info("dry-run: changes not committed")
		return nil
	}

	if err = promotion.WriteManifest(found); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	relPath, err := filepath.Rel(target.CheckoutPath, result.FilePath)
	if err != nil {
		return fmt.Errorf("relative path: %w", err)
	}

	gitCli := git.NewClient(a.logger)
	commitMsg := fmt.Sprintf("promote(%s): %s → %s", service.Key, result.OldTag, result.NewTag)

	if _, err = gitCli.StageAndCommit(target.CheckoutPath, relPath, commitMsg); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if err := gitCli.PushWithRetry(target.CheckoutPath, 3); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	return nil
}
