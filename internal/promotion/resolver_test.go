package promotion_test

import (
	"testing"

	"github.com/disbeliefff/loom/internal/models"
	"github.com/disbeliefff/loom/internal/promotion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDefaults(t *testing.T) {
	p := &models.Promotion{Enabled: true}
	promotion.ApplyDefaults(p)

	assert.Equal(t, "helm.toolkit.fluxcd.io/v2", p.ObjectRef.APIVersion)
	assert.Equal(t, "HelmRelease", p.ObjectRef.Kind)
	assert.Equal(t, "key", p.ObjectRef.NameField)
	assert.Equal(t, "image", p.ImageRef.RepositoryField)
	assert.Equal(t, "spec.values.image.repository", p.ImageRef.RepositoryPath)
	assert.Equal(t, "spec.values.image.tag", p.ImageRef.TagPath)
	assert.Equal(t, "event.toolkit.fluxcd.io/image", p.ImageRef.EventAnnotation)
	assert.Equal(t, "previous-annotation", p.Rollback.Strategy)
}

func TestApplyDefaults_DoesNotOverrideExplicit(t *testing.T) {
	p := &models.Promotion{
		Enabled: true,
		ObjectRef: models.ObjectRef{
			APIVersion: "custom.io/v1",
			Kind:       "CustomResource",
			NameField:  "helm_release",
		},
		ImageRef: models.ImageRef{
			RepositoryPath: "spec.image",
			TagPath:        "spec.version",
		},
	}
	promotion.ApplyDefaults(p)

	assert.Equal(t, "custom.io/v1", p.ObjectRef.APIVersion)
	assert.Equal(t, "CustomResource", p.ObjectRef.Kind)
	assert.Equal(t, "helm_release", p.ObjectRef.NameField)
	assert.Equal(t, "spec.image", p.ImageRef.RepositoryPath)
	assert.Equal(t, "spec.version", p.ImageRef.TagPath)
	assert.Equal(t, "image", p.ImageRef.RepositoryField)
	assert.Equal(t, "event.toolkit.fluxcd.io/image", p.ImageRef.EventAnnotation)
	assert.Equal(t, "previous-annotation", p.Rollback.Strategy)
}

func TestResolve(t *testing.T) {
	p := &models.Promotion{
		Enabled:           true,
		CheckoutPath:      "../gitops",
		ManifestPathField: "kustomize",
		ObjectRef: models.ObjectRef{
			APIVersion:     "helm.toolkit.fluxcd.io/v2",
			Kind:           "HelmRelease",
			NameField:      "helm_release",
			NamespaceField: "helm_namespace",
		},
		ImageRef: models.ImageRef{
			RepositoryField: "image",
			RepositoryPath:  "spec.values.image.repository",
			TagPath:         "spec.values.image.tag",
			EventAnnotation: "event.toolkit.fluxcd.io/image",
		},
		Rollback: models.Rollback{Strategy: "previous-annotation"},
		Target:   "production",
	}

	svc := models.Service{
		Key: "wallet-api",
		Raw: map[string]string{
			"key":           "wallet-api",
			"image":         "registry.example.com/platform/wallet-api",
			"kustomize":     "clusters/production/apps/wallet-api",
			"helm_release":  "wallet-api",
			"helm_namespace": "apps",
		},
	}

	rt, err := promotion.Resolve(p, svc)
	require.NoError(t, err)

	assert.Equal(t, "../gitops", rt.CheckoutPath)
	assert.Equal(t, "clusters/production/apps/wallet-api", rt.ManifestPath)
	assert.Equal(t, "helm.toolkit.fluxcd.io/v2", rt.APIVersion)
	assert.Equal(t, "HelmRelease", rt.Kind)
	assert.Equal(t, "wallet-api", rt.Name)
	assert.Equal(t, "apps", rt.Namespace)
	assert.Equal(t, "registry.example.com/platform/wallet-api", rt.Repository)
	assert.Equal(t, "spec.values.image.repository", rt.RepositoryPath)
	assert.Equal(t, "spec.values.image.tag", rt.TagPath)
	assert.Equal(t, "event.toolkit.fluxcd.io/image", rt.EventAnnotation)
	assert.Equal(t, "previous-annotation", rt.RollbackStrategy)
	assert.Equal(t, "production", rt.Target)
}

func TestResolve_NilPromotion(t *testing.T) {
	_, err := promotion.Resolve(nil, models.Service{Key: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "promotion config is nil")
}

func TestResolve_MissingManifestPathField(t *testing.T) {
	p := &models.Promotion{
		Enabled:           true,
		ManifestPathField: "kustomize",
		ObjectRef:         models.ObjectRef{NameField: "key"},
		ImageRef:          models.ImageRef{RepositoryField: "image"},
	}
	svc := models.Service{Key: "svc", Raw: map[string]string{"key": "svc"}}
	_, err := promotion.Resolve(p, svc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing field")
}

func TestResolve_MissingNameField(t *testing.T) {
	p := &models.Promotion{
		Enabled:           true,
		ManifestPathField: "kustomize",
		ObjectRef:         models.ObjectRef{NameField: "helm_release"},
		ImageRef:          models.ImageRef{RepositoryField: "image"},
	}
	svc := models.Service{
		Key: "svc",
		Raw: map[string]string{"kustomize": "some/path"},
	}
	_, err := promotion.Resolve(p, svc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing field")
}

func TestResolve_MissingRepositoryField(t *testing.T) {
	p := &models.Promotion{
		Enabled:           true,
		ManifestPathField: "kustomize",
		ObjectRef:         models.ObjectRef{NameField: "key"},
		ImageRef:          models.ImageRef{RepositoryField: "image"},
	}
	svc := models.Service{
		Key: "svc",
		Raw: map[string]string{"key": "svc", "kustomize": "some/path"},
	}
	_, err := promotion.Resolve(p, svc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing field")
}

func TestFindStrategy(t *testing.T) {
	cfg := &models.Config{
		Strategies: []models.Strategy{
			{Name: "dev-build"},
			{Name: "tag-release"},
		},
	}

	s, err := promotion.FindStrategy(cfg, "tag-release")
	require.NoError(t, err)
	assert.Equal(t, "tag-release", s.Name)

	_, err = promotion.FindStrategy(cfg, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveService(t *testing.T) {
	services := []models.Service{
		{Key: "svc-a"},
		{Key: "svc-b"},
	}

	t.Run("explicit service", func(t *testing.T) {
		s, err := promotion.ResolveService(services, "svc-b")
		require.NoError(t, err)
		assert.Equal(t, "svc-b", s.Key)
	})

	t.Run("unknown service", func(t *testing.T) {
		_, err := promotion.ResolveService(services, "svc-c")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("multiple services requires name", func(t *testing.T) {
		_, err := promotion.ResolveService(services, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--service is required")
	})

	t.Run("single service no name needed", func(t *testing.T) {
		s, err := promotion.ResolveService([]models.Service{{Key: "only"}}, "")
		require.NoError(t, err)
		assert.Equal(t, "only", s.Key)
	})

	t.Run("empty services", func(t *testing.T) {
		_, err := promotion.ResolveService(nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no services")
	})
}
