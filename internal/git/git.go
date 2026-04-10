package git

import (
	"fmt"
	"log/slog"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const emptyGitHash = "0000000000000000000000000000000000000000"

type Client struct {
	logger *slog.Logger
}

func NewClient(logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{logger: logger}
}

// GetChangedFiles returns a list of files changed between beforeSha and currentSha.
// If beforeSha is empty or "0000000000000000000000000000000000000000",
// it falls back to comparing HEAD with its parent (HEAD~1).
func (c *Client) GetChangedFiles(repoPath, beforeSha, currentSha string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open git repository %q: %w", repoPath, err)
	}

	var currentObj *object.Commit
	if currentSha == "" {
		headRef, errHead := repo.Head()
		if errHead != nil {
			return nil, fmt.Errorf("get HEAD: %w", errHead)
		}
		currentObj, err = repo.CommitObject(headRef.Hash())
		if err != nil {
			return nil, fmt.Errorf("get commit object for HEAD: %w", err)
		}
	} else {
		hash := plumbing.NewHash(currentSha)
		currentObj, err = repo.CommitObject(hash)
		if err != nil {
			return nil, fmt.Errorf("get commit object for %s: %w", currentSha, err)
		}
	}

	var beforeObj *object.Commit

	isInvalidBeforeSha := beforeSha == "" || beforeSha == emptyGitHash
	if isInvalidBeforeSha {
		c.logger.Info("Invalid or empty before_sha provided, falling back to HEAD~1", "before_sha", beforeSha)

		if currentObj.NumParents() == 0 {
			// Initial commit has no parents, so all files are "changed"
			return getInitialCommitFiles(currentObj)
		}

		parentHash := currentObj.ParentHashes[0]
		beforeObj, err = repo.CommitObject(parentHash)
		if err != nil {
			return nil, fmt.Errorf("get parent commit object: %w", err)
		}
	} else {
		hash := plumbing.NewHash(beforeSha)
		beforeObj, err = repo.CommitObject(hash)
		if err != nil {
			// If we can't find the explicit beforeSha, log warning and try parent fallback as requested
			c.logger.Warn("Could not find commit for before_sha, falling back to HEAD~1", "before_sha", beforeSha, "error", err)
			if currentObj.NumParents() > 0 {
				parentHash := currentObj.ParentHashes[0]
				beforeObj, err = repo.CommitObject(parentHash)
				if err != nil {
					return nil, fmt.Errorf("get parent commit object: %w", err)
				}
			} else {
				return getInitialCommitFiles(currentObj)
			}
		}
	}

	return getDiff(currentObj, beforeObj)
}

func getInitialCommitFiles(commit *object.Commit) ([]string, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var files []string
	err = tree.Files().ForEach(func(f *object.File) error {
		files = append(files, f.Name)
		return nil
	})
	return files, err
}

func getDiff(current, before *object.Commit) ([]string, error) {
	currentTree, err := current.Tree()
	if err != nil {
		return nil, err
	}

	beforeTree, err := before.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := beforeTree.Diff(currentTree)
	if err != nil {
		return nil, err
	}

	changedFiles := make([]string, 0, len(changes))
	for _, change := range changes {
		if change.To.Name != "" {
			changedFiles = append(changedFiles, change.To.Name)
		} else if change.From.Name != "" {
			changedFiles = append(changedFiles, change.From.Name)
		}
	}

	return changedFiles, nil
}
