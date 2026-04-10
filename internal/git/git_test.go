package git_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disbeliefff/loom/internal/git"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTempGitRepo(t *testing.T) (string, []string) {
	tempDir := t.TempDir()

	repo, err := gogit.PlainInit(tempDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	commitHashes := []string{}

	// Helper to commit a file
	commitFile := func(filename, content, msg string) string {
		filePath := filepath.Join(tempDir, filename)
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

		_, err := wt.Add(filename)
		require.NoError(t, err)

		commit, err := wt.Commit(msg, &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)
		return commit.String()
	}

	// Helper to remove a file
	removeFile := func(filename, msg string) string {
		_, err := wt.Remove(filename)
		require.NoError(t, err)

		commit, err := wt.Commit(msg, &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)
		return commit.String()
	}

	// 1. Initial commit (create main.go)
	commitHashes = append(commitHashes, commitFile("main.go", "package main\n", "Initial commit"))

	// 2. Second commit (create config.yaml)
	commitHashes = append(commitHashes, commitFile("config.yaml", "version: 1\n", "Add config"))

	// 3. Third commit (modify main.go)
	commitHashes = append(commitHashes, commitFile("main.go", "package main\nfunc main() {}\n", "Modify main"))

	// 4. Fourth commit (delete config.yaml)
	commitHashes = append(commitHashes, removeFile("config.yaml", "Delete config"))

	return tempDir, commitHashes
}

func TestGetChangedFiles(t *testing.T) {
	repoPath, commits := createTempGitRepo(t)

	t.Run("Diff between explicit commits", func(t *testing.T) {
		// Diff between commit 1 (main.go) and commit 2 (add config.yaml)
		changed, err := git.GetChangedFiles(repoPath, commits[0], commits[1])
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"config.yaml"}, changed)
	})

	t.Run("Diff with deletion", func(t *testing.T) {
		// Diff between commit 2 (config.yaml exists) and commit 3 (delete config.yaml)
		changed, err := git.GetChangedFiles(repoPath, commits[2], commits[3])
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"config.yaml"}, changed)
	})

	t.Run("Fallback to HEAD~1 when before_sha is empty", func(t *testing.T) {
		// Empty current_sha means HEAD. Empty before_sha means HEAD~1.
		// Our HEAD is commit 4 (deleted config.yaml).
		// HEAD~1 is commit 3 (modified main.go).
		// So diff between commit 3 and 4 should be "config.yaml"
		changed, err := git.GetChangedFiles(repoPath, "", "")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"config.yaml"}, changed)
	})

	t.Run("Fallback to HEAD~1 when before_sha is invalid 0000", func(t *testing.T) {
		changed, err := git.GetChangedFiles(repoPath, "0000000000000000000000000000000000000000", commits[2])
		require.NoError(t, err)
		// commit 2 is "Modify main".
		// Its parent is commit 1 "Add config".
		// diff between commit 1 and 2 is "main.go"
		assert.ElementsMatch(t, []string{"main.go"}, changed)
	})

	t.Run("Initial commit edge case fallback", func(t *testing.T) {
		// If current_sha is the initial commit, and before_sha is missing, it should return all files
		changed, err := git.GetChangedFiles(repoPath, "", commits[0])
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"main.go"}, changed)
	})

	t.Run("Non-existent repo path", func(t *testing.T) {
		_, err := git.GetChangedFiles("/path/does/not/exist", "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "open git repository")
	})

	t.Run("Non-existent explicit before_sha fails cleanly but falls back", func(t *testing.T) {
		// Invalid SHA (e.g. valid length but fake hash) will trigger a warning and fallback to parent
		fakeSha := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
		changed, err := git.GetChangedFiles(repoPath, fakeSha, commits[2])
		require.NoError(t, err)

		// Fallback means diff between parent of commits[2] and commits[2]
		assert.ElementsMatch(t, []string{"main.go"}, changed)
	})
}
