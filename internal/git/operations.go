package git

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type PushError struct {
	Err error
}

func (e *PushError) Error() string { return fmt.Sprintf("push failed: %v", e.Err) }

type repoContext struct {
	repo     *git.Repository
	worktree *git.Worktree
	auth     transport.AuthMethod
}

func (c *Client) openRepo(repoPath string) (*git.Repository, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo %q: %w", repoPath, err)
	}
	return repo, nil
}

func (c *Client) repoCtx(repoPath string) (*repoContext, error) {
	repo, err := c.openRepo(repoPath)
	if err != nil {
		return nil, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}
	auth, err := c.getAuth(repo)
	if err != nil {
		return nil, fmt.Errorf("resolve auth: %w", err)
	}
	return &repoContext{repo: repo, worktree: wt, auth: auth}, nil
}

func (c *Client) StageAndCommit(repoPath, filePath, message string) (plumbing.Hash, error) {
	repo, err := c.openRepo(repoPath)
	if err != nil {
		return plumbing.Hash{}, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("get worktree: %w", err)
	}

	if _, err = wt.Add(filePath); err != nil {
		return plumbing.Hash{}, fmt.Errorf("stage %q: %w", filePath, err)
	}

	author, err := c.defaultAuthor(repo)
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("resolve author: %w", err)
	}

	hash, err := wt.Commit(message, &git.CommitOptions{Author: author})
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("commit: %w", err)
	}

	c.logger.Info("committed", "hash", hash.String(), "file", filePath)
	return hash, nil
}

func (c *Client) Push(repoPath string) error {
	repo, err := c.openRepo(repoPath)
	if err != nil {
		return err
	}

	auth, err := c.getAuth(repo)
	if err != nil {
		return fmt.Errorf("resolve auth: %w", err)
	}

	refspec, err := c.currentBranchRefSpec(repo)
	if err != nil {
		return fmt.Errorf("resolve branch refspec: %w", err)
	}

	if err := repo.Push(&git.PushOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refspec},
	}); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		return &PushError{Err: err}
	}

	c.logger.Info("pushed", "path", repoPath)
	return nil
}

func (c *Client) PushWithRetry(repoPath string, maxRetries int) error {
	err := c.Push(repoPath)
	if err == nil {
		return nil
	}

	var pushErr *PushError
	if !errors.As(err, &pushErr) {
		return err
	}

	var lastErr error
	for i := range maxRetries {
		c.logger.Info("push rejected, pulling and retrying", "attempt", i+1)

		if pullErr := c.pullOrReset(repoPath); pullErr != nil {
			return fmt.Errorf("pull before retry: %w", pullErr)
		}
		if lastErr = c.Push(repoPath); lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("push failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) pullOrReset(repoPath string) error {
	ctx, err := c.repoCtx(repoPath)
	if err != nil {
		return err
	}

	err = ctx.worktree.Pull(&git.PullOptions{Auth: ctx.auth})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}

	head, headErr := ctx.repo.Head()
	if headErr != nil {
		return fmt.Errorf("get HEAD: %w", headErr)
	}

	commit, commitErr := ctx.repo.CommitObject(head.Hash())
	if commitErr != nil {
		return fmt.Errorf("get commit: %w", commitErr)
	}
	if commit.NumParents() == 0 {
		return fmt.Errorf("pull: %w", err)
	}

	if resetErr := ctx.worktree.Reset(&git.ResetOptions{
		Commit: commit.ParentHashes[0],
		Mode:   git.SoftReset,
	}); resetErr != nil {
		return fmt.Errorf("reset before pull: %w", resetErr)
	}

	if pullErr := ctx.worktree.Pull(&git.PullOptions{Auth: ctx.auth}); pullErr != nil {
		if errors.Is(pullErr, git.NoErrAlreadyUpToDate) {
			return nil
		}
		return fmt.Errorf("pull after reset: %w", pullErr)
	}

	c.logger.Info("pulled after soft-reset", "path", repoPath)
	return nil
}

func (c *Client) getAuth(repo *git.Repository) (transport.AuthMethod, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return nil, fmt.Errorf("get remote origin: %w", err)
	}

	if len(remote.Config().URLs) == 0 {
		return nil, nil
	}

	url := remote.Config().URLs[0]

	if strings.HasPrefix(url, "ssh://") || strings.HasPrefix(url, "git@") {
		auth, err := ssh.DefaultAuthBuilder("git")
		if err != nil {
			return nil, fmt.Errorf("ssh auth: %w", err)
		}
		return auth, nil
	}

	if token := os.Getenv("GIT_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "git",
			Password: token,
		}, nil
	}

	return nil, nil
}

func (c *Client) defaultAuthor(repo *git.Repository) (*object.Signature, error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, err
	}

	name := cfg.User.Name
	email := cfg.User.Email
	if name == "" {
		name = "loom"
	}
	if email == "" {
		email = "loom@loom"
	}

	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}, nil
}

func (c *Client) currentBranchRefSpec(repo *git.Repository) (config.RefSpec, error) {
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD: %w", err)
	}
	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not on a branch")
	}
	branch := head.Name().Short()
	return config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)), nil
}
