package git_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	googit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/disbeliefff/loom/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := googit.PlainInit(dir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644))
	_, err = wt.Add("README.md")
	require.NoError(t, err)

	_, err = wt.Commit("initial commit", &googit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	})
	require.NoError(t, err)

	return dir
}

func TestStageAndCommit(t *testing.T) {
	dir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.yaml"), []byte("key: value"), 0644))

	c := git.NewClient(slog.Default())
	hash, err := c.StageAndCommit(dir, "app.yaml", "add app.yaml")
	require.NoError(t, err)
	assert.NotEmpty(t, hash.String())

	repo, err := googit.PlainOpen(dir)
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)
	assert.Equal(t, hash, head.Hash())

	commit, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Equal(t, "add app.yaml", commit.Message)
}

func TestStageAndCommit_NotARepo(t *testing.T) {
	dir := t.TempDir()
	c := git.NewClient(slog.Default())
	_, err := c.StageAndCommit(dir, "file.txt", "msg")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}

func TestPush_NotARepo(t *testing.T) {
	dir := t.TempDir()
	c := git.NewClient(slog.Default())
	err := c.Push(dir)
	assert.Error(t, err)
}

func TestPushWithRetry_NoRemote(t *testing.T) {
	dir := initTestRepo(t)
	c := git.NewClient(slog.Default())
	err := c.PushWithRetry(dir, 1)
	assert.Error(t, err)
}
