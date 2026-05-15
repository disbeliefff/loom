package promotion

import (
	_ "embed"
	"errors"
	"fmt"
	"reflect"

	"github.com/disbeliefff/loom/internal/models"
	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaultsYAML []byte

var cachedDefaults *models.Promotion

var (
	supportedProviders = map[string]bool{
		"flux": true,
	}
	supportedModes = map[string]bool{
		"direct-commit": true,
	}
	supportedRollbackStrategies = map[string]bool{
		"previous-annotation": true,
		"none":                true,
	}
)

func getDefaults() *models.Promotion {
	if cachedDefaults != nil {
		return cachedDefaults
	}
	var d models.Promotion
	if err := yaml.Unmarshal(defaultsYAML, &d); err != nil {
		panic(fmt.Errorf("embedded defaults.yaml is invalid: %w", err))
	}
	cachedDefaults = &d
	return cachedDefaults
}

func ApplyDefaults(p *models.Promotion) {
	applyDefaults(reflect.ValueOf(p).Elem(), reflect.ValueOf(getDefaults()).Elem())
}

func applyDefaults(dst, src reflect.Value) {
	for i := range dst.NumField() {
		d, s := dst.Field(i), src.Field(i)
		switch d.Kind() {
		case reflect.Struct:
			applyDefaults(d, s)
		case reflect.String:
			if d.String() == "" {
				d.Set(s)
			}
		}
	}
}

func Validate(p *models.Promotion) error {
	var errs []error

	if !supportedProviders[p.Provider] {
		errs = append(errs, fmt.Errorf("unsupported promotion provider %q (supported: flux)", p.Provider))
	}
	if !supportedModes[p.Mode] {
		errs = append(errs, fmt.Errorf("unsupported promotion mode %q (supported: direct-commit)", p.Mode))
	}
	rs := p.Rollback.Strategy
	if rs != "" && !supportedRollbackStrategies[rs] {
		errs = append(errs, fmt.Errorf("unsupported rollback strategy %q (supported: previous-annotation, none)", rs))
	}

	return errors.Join(errs...)
}
