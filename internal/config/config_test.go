package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()

	validConfigPath := filepath.Join(tempDir, "valid-config.yaml")
	invalidConfigPath := filepath.Join(tempDir, "invalid-config.yaml")
	missingConfigPath := filepath.Join(tempDir, "missing-config.yaml")
	configDir := filepath.Join(tempDir, ".ci-templates", "templates", "monorepo")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	relativeConfigPath := filepath.Join(configDir, "pipeline-strategies.yaml")

	validYAML := []byte(`
strategies:
  - name: "test-strategy"
    condition: "true"
    selector:
      type: "git-diff"
      watch_field: "custom_dir"
    template: "test.tmpl"
`)
	require.NoError(t, os.WriteFile(validConfigPath, validYAML, 0644))

	invalidYAML := []byte(`
strategies:
  - name: "test-strategy"
  condition: "true"
  selector:
    type: "git-diff"
`)
	require.NoError(t, os.WriteFile(invalidConfigPath, invalidYAML, 0644))

	relativeYAML := []byte(`
strategies:
  - name: "dev-build"
    condition: "true"
    selector:
      type: "git-diff"
      watch_field: "watch_dir"
    template: "pipeline.tmpl"
`)
	require.NoError(t, os.WriteFile(relativeConfigPath, relativeYAML, 0644))

	t.Run("Valid Config", func(t *testing.T) {
		cfg, err := config.Load(validConfigPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.Strategies, 1)

		strategy := cfg.Strategies[0]
		assert.Equal(t, "test-strategy", strategy.Name)
		assert.Equal(t, "true", strategy.Condition)
		assert.Equal(t, filepath.Join(tempDir, "test.tmpl"), strategy.Template)
		assert.Equal(t, "git-diff", strategy.Selector.Type)
		assert.Equal(t, "custom_dir", strategy.Selector.WatchField)
	})

	t.Run("Relative template path is resolved against config directory", func(t *testing.T) {
		cfg, err := config.Load(relativeConfigPath)
		require.NoError(t, err)
		require.Len(t, cfg.Strategies, 1)
		assert.Equal(t, filepath.Join(configDir, "pipeline.tmpl"), cfg.Strategies[0].Template)
	})

	t.Run("Invalid Config Returns Error", func(t *testing.T) {
		_, err := config.Load(invalidConfigPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse config file")
	})

	t.Run("Missing Config Returns Error", func(t *testing.T) {
		_, err := config.Load(missingConfigPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read config file")
	})
}

func TestLoadServices(t *testing.T) {
	tempDir := t.TempDir()

	validServicesPath := filepath.Join(tempDir, "valid-services.json")
	invalidServicesPath := filepath.Join(tempDir, "invalid-services.json")
	missingServicesPath := filepath.Join(tempDir, "missing-services.json")

	validJSON := []byte(`
[
  {
    "key": "AuthService",
    "watch_dir": "src/auth/",
    "custom_field": "test_value"
  },
  {
    "watch_dir": "src/missing_key/"
  },
  {
    "key": "Bad!@#Key",
    "watch_dir": "src/bad_key/"
  }
]
`)
	require.NoError(t, os.WriteFile(validServicesPath, validJSON, 0644))

	invalidJSON := []byte(`[ { "key": "Missing quotes } ]`)
	require.NoError(t, os.WriteFile(invalidServicesPath, invalidJSON, 0644))

	validYAMLPath := filepath.Join(tempDir, "valid-services.yaml")
	invalidYAMLPath := filepath.Join(tempDir, "invalid-services.yml")

	validYAML := []byte(`
- key: "AuthService"
  watch_dir: "src/auth/"
  custom_field: "test_value"
- watch_dir: "src/missing_key/"
- key: "Bad!@#Key"
  watch_dir: "src/bad_key/"
`)
	require.NoError(t, os.WriteFile(validYAMLPath, validYAML, 0644))

	invalidYAML := []byte(`
- key: "Missing quote
  watch_dir: "src/"
`)
	require.NoError(t, os.WriteFile(invalidYAMLPath, invalidYAML, 0644))

	t.Run("Valid Services", func(t *testing.T) {
		services, err := config.LoadServices(validServicesPath)
		require.NoError(t, err)

		// The second item in JSON is missing a "key", so it should be skipped.
		require.Len(t, services, 2)

		assert.Equal(t, "AuthService", services[0].Key)
		assert.Equal(t, "AuthService", services[0].SafeKey)
		assert.Equal(t, "test_value", services[0].Raw["custom_field"])

		assert.Equal(t, "Bad!@#Key", services[1].Key)
		assert.Equal(t, "BadKey", services[1].SafeKey) // Special characters removed
		assert.Equal(t, "src/bad_key/", services[1].Raw["watch_dir"])
	})

	t.Run("Invalid Services JSON Returns Error", func(t *testing.T) {
		_, err := config.LoadServices(invalidServicesPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse json services file")
	})

	t.Run("Missing Services JSON Returns Error", func(t *testing.T) {
		_, err := config.LoadServices(missingServicesPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read services file")
	})

	t.Run("Valid Services YAML", func(t *testing.T) {
		services, err := config.LoadServices(validYAMLPath)
		require.NoError(t, err)

		require.Len(t, services, 2)

		assert.Equal(t, "AuthService", services[0].Key)
		assert.Equal(t, "AuthService", services[0].SafeKey)
		assert.Equal(t, "test_value", services[0].Raw["custom_field"])

		assert.Equal(t, "Bad!@#Key", services[1].Key)
		assert.Equal(t, "BadKey", services[1].SafeKey)
		assert.Equal(t, "src/bad_key/", services[1].Raw["watch_dir"])
	})

	t.Run("Invalid Services YAML Returns Error", func(t *testing.T) {
		_, err := config.LoadServices(invalidYAMLPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse yaml services file")
	})
}
