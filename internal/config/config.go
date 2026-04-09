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
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
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
		return nil, fmt.Errorf("failed to read services file %s: %w", path, err)
	}

	var rawServices []map[string]any
	if err := json.Unmarshal(data, &rawServices); err != nil {
		return nil, fmt.Errorf("failed to parse services file %s: %w", path, err)
	}

	var services []models.Service
	for _, raw := range rawServices {
		keyVal, ok := raw["key"].(string)
		if !ok || keyVal == "" {
			continue // Skip elements without a valid "key" field
		}

		safeKey := safeKeyRegex.ReplaceAllString(keyVal, "")

		services = append(services, models.Service{
			Key:     keyVal,
			SafeKey: safeKey,
			Raw:     raw,
		})
	}

	return services, nil
}

// MustLoad parses the YAML pipeline strategies configuration or panics on error.
func MustLoad(path string) *models.Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}
	return cfg
}

// MustLoadServices parses the JSON file containing the list of services or panics on error.
func MustLoadServices(path string) []models.Service {
	services, err := LoadServices(path)
	if err != nil {
		panic(err)
	}
	return services
}
