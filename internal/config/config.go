package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/disbeliefff/loom/internal/models"
	"sigs.k8s.io/yaml"
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

// LoadServices parses the JSON or YAML file containing the list of services.
func LoadServices(path string) ([]models.Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read services file %q: %w", path, err)
	}

	var rawServices []map[string]any
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &rawServices); err != nil {
			return nil, fmt.Errorf("parse yaml services file %q: %w", path, err)
		}
	} else {
		if err := json.Unmarshal(data, &rawServices); err != nil {
			return nil, fmt.Errorf("parse json services file %q: %w", path, err)
		}
	}

	services := make([]models.Service, 0, len(rawServices))
	for _, rawAny := range rawServices {
		// Convert map[string]any to map[string]string for compatibility
		raw := make(map[string]string)
		for k, v := range rawAny {
			if strVal, ok := v.(string); ok {
				raw[k] = strVal
			} else if v != nil {
				raw[k] = fmt.Sprintf("%v", v)
			}
		}

		keyVal, ok := raw["key"]
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
