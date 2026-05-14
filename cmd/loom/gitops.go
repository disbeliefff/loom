package main

import "github.com/spf13/cobra"

func newGitOpsCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "GitOps promotion commands for Flux-managed manifests",
	}

	cmd.AddCommand(newGitOpsPromoteCommand(a))
	cmd.AddCommand(newGitOpsRollbackCommand(a))

	return cmd
}
