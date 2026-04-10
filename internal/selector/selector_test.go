package selector_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disbeliefff/loom/internal/models"
	"github.com/disbeliefff/loom/internal/selector"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply_EnvMatch(t *testing.T) {
	// Setup
	os.Setenv("DEPLOY_AUTH_SERVICE", "true")
	os.Setenv("DEPLOY_DB_SERVICE", "false")
	defer os.Unsetenv("DEPLOY_AUTH_SERVICE")
	defer os.Unsetenv("DEPLOY_DB_SERVICE")

	services := []models.Service{
		{Key: "auth-service"},
		{Key: "db-service"},
		{Key: "other-service"},
	}

	cfg := models.SelectorConfig{
		Type:   "env-match",
		Prefix: "DEPLOY_",
	}

	ctx := models.PipelineContext{}

	jobs, err := selector.Apply(cfg, ctx, services)
	require.NoError(t, err)

	require.Len(t, jobs, 1)
	assert.Equal(t, "auth-service", jobs[0].Service.Key)
}

func TestApply_RegexTag(t *testing.T) {
	services := []models.Service{
		{Key: "AccountingAPI"},
		{Key: "AuthAPI"},
	}

	cfg := models.SelectorConfig{
		Type:    "regex-tag",
		Pattern: `^{{ .Service.Key }}/v.*$`,
	}

	// 1. Matching Tag
	ctxMatch := models.PipelineContext{CommitTag: "AccountingAPI/v1.0.0"}
	jobs, err := selector.Apply(cfg, ctxMatch, services)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "AccountingAPI", jobs[0].Service.Key)

	// 2. Non-matching Tag
	ctxNoMatch := models.PipelineContext{CommitTag: "RandomTag/v2.0"}
	jobs, err = selector.Apply(cfg, ctxNoMatch, services)
	require.NoError(t, err)
	require.Len(t, jobs, 0)

	// 3. Empty Tag
	ctxEmpty := models.PipelineContext{CommitTag: ""}
	jobs, err = selector.Apply(cfg, ctxEmpty, services)
	require.NoError(t, err)
	require.Len(t, jobs, 0)

	// 4. Invalid Regex Pattern (e.g. missing string interpolation closure)
	cfgInvalid := models.SelectorConfig{
		Type:    "regex-tag",
		Pattern: `^{{ .Service.InvalidKey }}/v.*$`, // Accessing undefined field
	}
	_, err = selector.Apply(cfgInvalid, ctxMatch, services)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to render pattern template")
}

func createTempGitRepoWithFiles(t *testing.T) (string, []string) {
	tempDir := t.TempDir()

	repo, err := gogit.PlainInit(tempDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	commitHashes := []string{}

	commitFile := func(filename, msg string) string {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(tempDir, filename)), 0755))
		filePath := filepath.Join(tempDir, filename)
		require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

		_, err := wt.Add(filename)
		require.NoError(t, err)

		commit, err := wt.Commit(msg, &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)
		return commit.String()
	}

	commitHashes = append(commitHashes, commitFile("src/auth/main.go", "Init auth"))
	commitHashes = append(commitHashes, commitFile("src/billing/main.go", "Init billing"))

	return tempDir, commitHashes
}

func TestApply_GitDiff(t *testing.T) {
	repoPath, commits := createTempGitRepoWithFiles(t)

	services := []models.Service{
		{
			Key: "auth-service",
			Raw: map[string]any{"watch_dir": "src/auth"},
		},
		{
			Key: "billing-service",
			Raw: map[string]any{"watch_dir": "src/billing"},
		},
		{
			Key: "other-service",
			Raw: map[string]any{"watch_dir": ""}, // Missing watch dir
		},
	}

	cfg := models.SelectorConfig{
		Type:       "git-diff",
		BeforeSHA:  "{{ .Context.BeforeSHA }}",
		CurrentSHA: "{{ .Context.CommitSHA }}",
		WatchField: "watch_dir",
		RepoRoot:   "{{ env \"CUSTOM_CI_DIR\" }}",
	}

	// Mock env var
	os.Setenv("CUSTOM_CI_DIR", repoPath)
	defer os.Unsetenv("CUSTOM_CI_DIR")

	ctx := models.PipelineContext{
		BeforeSHA: commits[0],
		CommitSHA: commits[1],
		RepoRoot:  "/this/path/should/be/overridden",
	}

	// In commit 1, we added "src/billing/main.go", so billing-service should be selected.
	jobs, err := selector.Apply(cfg, ctx, services)
	require.NoError(t, err)

	require.Len(t, jobs, 1)
	assert.Equal(t, "billing-service", jobs[0].Service.Key)
}

func TestApply_GitDiff_ContextRepoRoot(t *testing.T) {
	repoPath, commits := createTempGitRepoWithFiles(t)

	services := []models.Service{
		{
			Key: "auth-service",
			Raw: map[string]any{"watch_dir": "src/auth"},
		},
		{
			Key: "billing-service",
			Raw: map[string]any{"watch_dir": "src/billing"},
		},
	}

	cfg := models.SelectorConfig{
		Type:       "git-diff",
		BeforeSHA:  "{{ .Context.BeforeSHA }}",
		CurrentSHA: "{{ .Context.CommitSHA }}",
		WatchField: "watch_dir",
	}

	ctx := models.PipelineContext{
		BeforeSHA: commits[0],
		CommitSHA: commits[1],
		RepoRoot:  repoPath,
	}

	jobs, err := selector.Apply(cfg, ctx, services)
	require.NoError(t, err)

	require.Len(t, jobs, 1)
	assert.Equal(t, "billing-service", jobs[0].Service.Key)
}

func TestApply_UnknownType(t *testing.T) {
	cfg := models.SelectorConfig{Type: "unknown"}
	_, err := selector.Apply(cfg, models.PipelineContext{}, []models.Service{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown selector type: unknown")
}

func TestApply_MissingType(t *testing.T) {
	cfg := models.SelectorConfig{Type: ""}
	_, err := selector.Apply(cfg, models.PipelineContext{}, []models.Service{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selector 'type' is missing")
}
