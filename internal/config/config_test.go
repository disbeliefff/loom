package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disbeliefff/loom/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustLoad(t *testing.T) {
	tempDir := t.TempDir()

	validConfigPath := filepath.Join(tempDir, "valid-config.yaml")
	invalidConfigPath := filepath.Join(tempDir, "invalid-config.yaml")
	missingConfigPath := filepath.Join(tempDir, "missing-config.yaml")

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

	t.Run("Valid Config", func(t *testing.T) {
		cfg := config.MustLoad(validConfigPath)
		require.NotNil(t, cfg)
		require.Len(t, cfg.Strategies, 1)

		strategy := cfg.Strategies[0]
		assert.Equal(t, "test-strategy", strategy.Name)
		assert.Equal(t, "true", strategy.Condition)
		assert.Equal(t, "test.tmpl", strategy.Template)
		assert.Equal(t, "git-diff", strategy.Selector.Type)
		assert.Equal(t, "custom_dir", strategy.Selector.WatchField)
	})

	t.Run("Invalid Config Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			config.MustLoad(invalidConfigPath)
		})
	})

	t.Run("Missing Config Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			config.MustLoad(missingConfigPath)
		})
	})
}

func TestMustLoadServices(t *testing.T) {
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

	t.Run("Valid Services", func(t *testing.T) {
		services := config.MustLoadServices(validServicesPath)

		// The second item in JSON is missing a "key", so it should be skipped.
		require.Len(t, services, 2)

		assert.Equal(t, "AuthService", services[0].Key)
		assert.Equal(t, "AuthService", services[0].SafeKey)
		assert.Equal(t, "test_value", services[0].Raw["custom_field"])

		assert.Equal(t, "Bad!@#Key", services[1].Key)
		assert.Equal(t, "BadKey", services[1].SafeKey) // Special characters removed
		assert.Equal(t, "src/bad_key/", services[1].Raw["watch_dir"])
	})

	t.Run("Invalid Services JSON Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			config.MustLoadServices(invalidServicesPath)
		})
	})

	t.Run("Missing Services JSON Panics", func(t *testing.T) {
		assert.Panics(t, func() {
			config.MustLoadServices(missingServicesPath)
		})
	})
}
