package promotion_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disbeliefff/loom/internal/promotion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTarget() *promotion.ResolvedTarget {
	return &promotion.ResolvedTarget{
		ManifestPath:     "",
		APIVersion:       "helm.toolkit.fluxcd.io/v2",
		Kind:             "HelmRelease",
		Name:             "example-api",
		Namespace:        "apps",
		Repository:       "registry.example.com/platform/example-api",
		RepositoryPath:   "spec.values.image.repository",
		TagPath:          "spec.values.image.tag",
		EventAnnotation:  "event.toolkit.fluxcd.io/image",
		RollbackStrategy: "previous-annotation",
	}
}

const helmReleaseYAML = `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: example-api
  namespace: apps
  annotations:
    event.toolkit.fluxcd.io/image: "registry.example.com/platform/example-api:v1.2.2"
spec:
  values:
    replicaCount: 3
    image:
      repository: registry.example.com/platform/example-api
      tag: "v1.2.2"
`

func writeManifestDir(t *testing.T, manifestYAML string) string {
	t.Helper()
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, "apps", "example-api")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, "release.yaml"), []byte(manifestYAML), 0644))
	return dir
}

func TestFindManifest_SingleMatch(t *testing.T) {
	target := testTarget()
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, filepath.Join(dir, "apps", "example-api", "release.yaml"), found.FilePath)
}

func TestFindManifest_ZeroMatches(t *testing.T) {
	target := testTarget()
	dir := writeManifestDir(t, "apiVersion: v1\nkind: ConfigMap\n")
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	_, err := mutator.FindManifest(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest matching")
}

func TestFindManifest_MultipleMatches(t *testing.T) {
	manifestDir := t.TempDir()
	subDir := filepath.Join(manifestDir, "apps", "multi")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	yaml1 := helmReleaseYAML
	yaml2 := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: example-api
  namespace: apps
spec:
  values:
    image:
      repository: other
      tag: "v0.0.1"
`
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "a.yaml"), []byte(yaml1), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "b.yaml"), []byte(yaml2), 0644))

	target := testTarget()
	target.ManifestPath = "apps/multi"

	mutator := promotion.NewManifestMutator(target)
	_, err := mutator.FindManifest(manifestDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected exactly one")
}

func TestFindManifest_PathIsFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "manifest.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte(helmReleaseYAML), 0644))

	target := testTarget()
	target.ManifestPath = "manifest.yaml"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)
	assert.Equal(t, filePath, found.FilePath)
}

func TestPromote(t *testing.T) {
	target := testTarget()
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)

	result, err := mutator.Promote(found, "v1.2.3")
	require.NoError(t, err)

	assert.Equal(t, "registry.example.com/platform/example-api", result.OldRepo)
	assert.Equal(t, "v1.2.2", result.OldTag)
	assert.Equal(t, "registry.example.com/platform/example-api:v1.2.2", result.OldEvent)
	assert.Equal(t, "registry.example.com/platform/example-api", result.NewRepo)
	assert.Equal(t, "v1.2.3", result.NewTag)
	assert.Equal(t, "registry.example.com/platform/example-api:v1.2.3", result.NewEvent)
}

func TestPromote_WritesRollbackAnnotations(t *testing.T) {
	t.Setenv("GITHUB_ACTOR", "loom")

	target := testTarget()
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)

	_, err = mutator.Promote(found, "v1.2.3")
	require.NoError(t, err)

	require.NoError(t, promotion.WriteManifest(found))

	data, err := os.ReadFile(found.FilePath)
	require.NoError(t, err)

	assert.Contains(t, string(data), "loom.disbeliefff.github.io/previous-repository: registry.example.com/platform/example-api")
	assert.Contains(t, string(data), "loom.disbeliefff.github.io/previous-tag: v1.2.2")
	assert.Contains(t, string(data), "loom.disbeliefff.github.io/previous-image: registry.example.com/platform/example-api:v1.2.2")
	assert.Contains(t, string(data), "loom.disbeliefff.github.io/promoted-by: loom")
	assert.Contains(t, string(data), "loom.disbeliefff.github.io/promoted-at:")
	assert.Contains(t, string(data), "registry.example.com/platform/example-api:v1.2.3")
}

func TestPromote_ImagePolicyDetected(t *testing.T) {
	manifest := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: example-api
  namespace: apps
spec:
  values:
    image:
      repository: registry.example.com/platform/example-api # {"$imagepolicy": "apps:example-api:name"}
      tag: "v1.2.2" # {"$imagepolicy": "apps:example-api:tag"}
`
	target := testTarget()
	dir := writeManifestDir(t, manifest)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)

	_, err = mutator.Promote(found, "v1.2.3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "$imagepolicy")
}

func TestRollback(t *testing.T) {
	target := testTarget()
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)

	_, err = mutator.Promote(found, "v1.2.3")
	require.NoError(t, err)

	result, err := mutator.Rollback(found)
	require.NoError(t, err)

	assert.Equal(t, "registry.example.com/platform/example-api", result.OldRepo)
	assert.Equal(t, "v1.2.3", result.OldTag)
	assert.Equal(t, "v1.2.2", result.NewTag)
	assert.Equal(t, "registry.example.com/platform/example-api:v1.2.2", result.NewEvent)
}

func TestRollback_NoPreviousAnnotations(t *testing.T) {
	target := testTarget()
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)

	_, err = mutator.Rollback(found)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no previous promotion annotations")
}

func TestWriteManifest_PreservesOtherDocuments(t *testing.T) {
	multiDoc := `apiVersion: v1
kind: ConfigMap
metadata:
  name: unrelated
data:
  key: value
---
` + helmReleaseYAML

	dir := t.TempDir()
	filePath := filepath.Join(dir, "multi.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte(multiDoc), 0644))

	target := testTarget()
	target.ManifestPath = "multi.yaml"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, found.MatchIndex)

	_, err = mutator.Promote(found, "v2.0.0")
	require.NoError(t, err)

	require.NoError(t, promotion.WriteManifest(found))

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "unrelated")
	assert.Contains(t, content, "v2.0.0")
}

func TestPromote_NamespaceMismatch(t *testing.T) {
	target := testTarget()
	target.Namespace = "wrong-ns"
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	_, err := mutator.FindManifest(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest matching")
}

func TestPromote_OptionalNamespace(t *testing.T) {
	target := testTarget()
	target.Namespace = ""
	dir := writeManifestDir(t, helmReleaseYAML)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)
	assert.NotNil(t, found)
}

func TestPromote_AnnotationsNotMapping(t *testing.T) {
	manifest := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: example-api
  namespace: apps
  annotations: not-a-mapping
spec:
  values:
    image:
      repository: registry.example.com/platform/example-api
      tag: "v1.2.2"
`
	target := testTarget()
	dir := writeManifestDir(t, manifest)
	target.ManifestPath = "apps/example-api"

	mutator := promotion.NewManifestMutator(target)
	found, err := mutator.FindManifest(dir)
	require.NoError(t, err)

	_, err = mutator.Promote(found, "v1.2.3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "annotations is not a mapping")
}
