package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/disbeliefff/loom/internal/models"
	"gopkg.in/yaml.v3"
)

var safeKeyRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Load parses the YAML pipeline strategies configuration.
// Relative template paths are resolved against the config file location.
func Load(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	baseDir := filepath.Dir(path)
	for i := range cfg.Strategies {
		if cfg.Strategies[i].Template == "" || filepath.IsAbs(cfg.Strategies[i].Template) {
			continue
		}
		cfg.Strategies[i].Template = filepath.Clean(filepath.Join(baseDir, cfg.Strategies[i].Template))
	}

	return &cfg, nil
}

// LoadServices parses the JSON file containing the list of services.
func LoadServices(path string) ([]models.Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read services file %q: %w", path, err)
	}

	var rawServices []map[string]any
	if err := json.Unmarshal(data, &rawServices); err != nil {
		return nil, fmt.Errorf("parse services file %q: %w", path, err)
	}

	services := make([]models.Service, 0, len(rawServices))
	for _, raw := range rawServices {
		keyVal, ok := raw["key"].(string)
		if !ok || keyVal == "" {
			continue // Skip elements without a valid "key" field
		}

		services = append(services, models.Service{
			Key:     keyVal,
			SafeKey: safeKeyRegex.ReplaceAllString(keyVal, ""),
			Raw:     raw,
		})
	}

	return services, nil
}
