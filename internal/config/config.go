package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/disbeliefff/loom/internal/models"
	"gopkg.in/yaml.v3"
)

var safeKeyRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// MustLoad parses the YAML pipeline strategies configuration or panics on error.
func MustLoad(path string) *models.Config {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("failed to read config file %s: %w", path, err))
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic(fmt.Errorf("failed to parse config file %s: %w", path, err))
	}

	return &cfg
}

// MustLoadServices parses the JSON file containing the list of services or panics on error.
func MustLoadServices(path string) []models.Service {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("failed to read services file %s: %w", path, err))
	}

	var rawServices []map[string]any
	if err := json.Unmarshal(data, &rawServices); err != nil {
		panic(fmt.Errorf("failed to parse services file %s: %w", path, err))
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

	return services
}
