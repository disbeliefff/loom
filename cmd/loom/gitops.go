package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGitOpsCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "GitOps promotion commands for Flux-managed manifests",
	}

	promoteCmd, err := newGitOpsPromoteCommand(a)
	if err != nil {
		panic(fmt.Sprintf("gitops promote: %v", err))
	}
	rollbackCmd, err := newGitOpsRollbackCommand(a)
	if err != nil {
		panic(fmt.Sprintf("gitops rollback: %v", err))
	}

	cmd.AddCommand(promoteCmd)
	cmd.AddCommand(rollbackCmd)

	return cmd
}
