package promotion

import (
	_ "embed"
	"reflect"

	"github.com/disbeliefff/loom/internal/models"
	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaultsYAML []byte

var cachedDefaults *models.Promotion

func getDefaults() *models.Promotion {
	if cachedDefaults != nil {
		return cachedDefaults
	}
	var d models.Promotion
	if err := yaml.Unmarshal(defaultsYAML, &d); err != nil {
		panic("embedded defaults.yaml is invalid: " + err.Error())
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
