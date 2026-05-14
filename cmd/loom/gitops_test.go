package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/disbeliefff/loom/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitOpsPromote_MissingStrategy(t *testing.T) {
	tempDir := t.TempDir()

	cfgYAML := `strategies:
  - name: dev-build
    condition: 'true'
    selector:
      type: git-diff
    template: test.tmpl
`
	cfgPath := filepath.Join(tempDir, "pipeline-strategies.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	svcJSON := `[{"key": "svc-a", "image": "registry/svc-a", "kustomize": "apps/svc-a"}]`
	svcPath := filepath.Join(tempDir, "services.json")
	require.NoError(t, os.WriteFile(svcPath, []byte(svcJSON), 0644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var err error
	require.NotPanics(t, func() {
		a := newApp()
		a.logger = logger.New(false)
		err = executeWithArgs(a, []string{
			"gitops", "promote",
			"--config", cfgPath,
			"--services", svcPath,
			"--strategy", "nonexistent",
			"--tag", "v1.0.0",
		}, stdout, stderr)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGitOpsPromote_DisabledPromotion(t *testing.T) {
	tempDir := t.TempDir()

	cfgYAML := `strategies:
  - name: dev-build
    condition: 'true'
    selector:
      type: git-diff
    template: test.tmpl
`
	cfgPath := filepath.Join(tempDir, "pipeline-strategies.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	svcJSON := `[{"key": "svc-a", "image": "registry/svc-a", "kustomize": "apps/svc-a"}]`
	svcPath := filepath.Join(tempDir, "services.json")
	require.NoError(t, os.WriteFile(svcPath, []byte(svcJSON), 0644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var err error
	require.NotPanics(t, func() {
		a := newApp()
		a.logger = logger.New(false)
		err = executeWithArgs(a, []string{
			"gitops", "promote",
			"--config", cfgPath,
			"--services", svcPath,
			"--strategy", "dev-build",
			"--tag", "v1.0.0",
		}, stdout, stderr)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have promotion enabled")
}

func TestGitOpsPromote_PromotionEnabledButNoManifest(t *testing.T) {
	tempDir := t.TempDir()

	cfgYAML := `strategies:
  - name: tag-release
    condition: 'true'
    selector:
      type: regex-tag
      pattern: '^{{ .Service.Key }}/.*$'
    template: test.tmpl
    promotion:
      enabled: true
      provider: flux
      mode: direct-commit
      target: production
      checkout_path: ` + tempDir + `/gitops
      manifest_path_field: kustomize
      object_ref:
        api_version: helm.toolkit.fluxcd.io/v2
        kind: HelmRelease
        name_field: key
      image_ref:
        repository_field: image
        repository_path: spec.values.image.repository
        tag_path: spec.values.image.tag
`
	cfgPath := filepath.Join(tempDir, "pipeline-strategies.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	svcJSON := `[{"key": "svc-a", "image": "registry/svc-a", "kustomize": "apps/svc-a"}]`
	svcPath := filepath.Join(tempDir, "services.json")
	require.NoError(t, os.WriteFile(svcPath, []byte(svcJSON), 0644))

	gitopsDir := filepath.Join(tempDir, "gitops", "apps", "svc-a")
	require.NoError(t, os.MkdirAll(gitopsDir, 0755))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var err error
	require.NotPanics(t, func() {
		a := newApp()
		a.logger = logger.New(false)
		err = executeWithArgs(a, []string{
			"gitops", "promote",
			"--config", cfgPath,
			"--services", svcPath,
			"--strategy", "tag-release",
			"--service", "svc-a",
			"--tag", "v1.0.0",
		}, stdout, stderr)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest matching")
}

func TestGitOpsPromote_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	gitopsDir := filepath.Join(tempDir, "gitops", "apps", "svc-a")
	require.NoError(t, os.MkdirAll(gitopsDir, 0755))

	manifest := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: svc-a
  namespace: apps
spec:
  values:
    image:
      repository: registry/svc-a
      tag: "v0.9.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(gitopsDir, "release.yaml"), []byte(manifest), 0644))

	cfgYAML := `strategies:
  - name: tag-release
    condition: 'true'
    selector:
      type: regex-tag
    template: test.tmpl
    promotion:
      enabled: true
      provider: flux
      mode: direct-commit
      target: production
      checkout_path: ` + tempDir + `/gitops
      manifest_path_field: kustomize
      object_ref:
        name_field: key
      image_ref:
        repository_field: image
`
	cfgPath := filepath.Join(tempDir, "pipeline-strategies.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	svcJSON := `[{"key": "svc-a", "image": "registry/svc-a", "kustomize": "apps/svc-a"}]`
	svcPath := filepath.Join(tempDir, "services.json")
	require.NoError(t, os.WriteFile(svcPath, []byte(svcJSON), 0644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var err error
	require.NotPanics(t, func() {
		a := newApp()
		a.logger = logger.New(false)
		err = executeWithArgs(a, []string{
			"gitops", "promote",
			"--config", cfgPath,
			"--services", svcPath,
			"--strategy", "tag-release",
			"--service", "svc-a",
			"--tag", "v1.0.0",
			"--dry-run",
		}, stdout, stderr)
	})

	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(gitopsDir, "release.yaml"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "v0.9.0")
	assert.NotContains(t, string(data), "v1.0.0")
}

func TestGitOpsPromote_ServiceRequiredWithMultiple(t *testing.T) {
	tempDir := t.TempDir()

	cfgYAML := `strategies:
  - name: tag-release
    condition: 'true'
    selector:
      type: regex-tag
    template: test.tmpl
    promotion:
      enabled: true
      checkout_path: /tmp/gitops
`
	cfgPath := filepath.Join(tempDir, "pipeline-strategies.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	svcJSON := `[{"key": "svc-a", "image": "reg/a", "kustomize": "a"}, {"key": "svc-b", "image": "reg/b", "kustomize": "b"}]`
	svcPath := filepath.Join(tempDir, "services.json")
	require.NoError(t, os.WriteFile(svcPath, []byte(svcJSON), 0644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var err error
	require.NotPanics(t, func() {
		a := newApp()
		a.logger = logger.New(false)
		err = executeWithArgs(a, []string{
			"gitops", "promote",
			"--config", cfgPath,
			"--services", svcPath,
			"--strategy", "tag-release",
			"--tag", "v1.0.0",
		}, stdout, stderr)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--service is required")
}

func TestGitOpsRollback_NoPreviousAnnotations(t *testing.T) {
	tempDir := t.TempDir()

	gitopsDir := filepath.Join(tempDir, "gitops", "apps", "svc-a")
	require.NoError(t, os.MkdirAll(gitopsDir, 0755))

	manifest := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: svc-a
  namespace: apps
spec:
  values:
    image:
      repository: registry/svc-a
      tag: "v1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(gitopsDir, "release.yaml"), []byte(manifest), 0644))

	cfgYAML := `strategies:
  - name: tag-release
    condition: 'true'
    selector:
      type: regex-tag
    template: test.tmpl
    promotion:
      enabled: true
      checkout_path: ` + tempDir + `/gitops
      manifest_path_field: kustomize
`
	cfgPath := filepath.Join(tempDir, "pipeline-strategies.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	svcJSON := `[{"key": "svc-a", "image": "registry/svc-a", "kustomize": "apps/svc-a"}]`
	svcPath := filepath.Join(tempDir, "services.json")
	require.NoError(t, os.WriteFile(svcPath, []byte(svcJSON), 0644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var err error
	require.NotPanics(t, func() {
		a := newApp()
		a.logger = logger.New(false)
		err = executeWithArgs(a, []string{
			"gitops", "rollback",
			"--config", cfgPath,
			"--services", svcPath,
			"--strategy", "tag-release",
			"--service", "svc-a",
			"--dry-run",
		}, stdout, stderr)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no previous promotion annotations")
}
