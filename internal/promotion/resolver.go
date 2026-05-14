package promotion

import (
	"fmt"

	"github.com/disbeliefff/loom/internal/models"
)

type ResolvedTarget struct {
	CheckoutPath     string
	ManifestPath     string
	APIVersion       string
	Kind             string
	Name             string
	Namespace        string
	Repository       string
	RepositoryPath   string
	TagPath          string
	EventAnnotation  string
	RollbackStrategy string
	Target           string
}

func resolveField(raw map[string]string, serviceKey, field string) (string, error) {
	v := raw[field]
	if v == "" {
		return "", fmt.Errorf("service %q missing field %q", serviceKey, field)
	}
	return v, nil
}

func Resolve(promotion *models.Promotion, service models.Service) (*ResolvedTarget, error) {
	if promotion == nil {
		return nil, fmt.Errorf("promotion config is nil")
	}

	resolve := func(field string) (string, error) {
		return resolveField(service.Raw, service.Key, field)
	}

	manifestPath, err := resolve(promotion.ManifestPathField)
	if err != nil {
		return nil, err
	}
	name, err := resolve(promotion.ObjectRef.NameField)
	if err != nil {
		return nil, err
	}
	repo, err := resolve(promotion.ImageRef.RepositoryField)
	if err != nil {
		return nil, err
	}

	rt := &ResolvedTarget{
		CheckoutPath:     promotion.CheckoutPath,
		APIVersion:       promotion.ObjectRef.APIVersion,
		Kind:             promotion.ObjectRef.Kind,
		ManifestPath:     manifestPath,
		Name:             name,
		Repository:       repo,
		RepositoryPath:   promotion.ImageRef.RepositoryPath,
		TagPath:          promotion.ImageRef.TagPath,
		EventAnnotation:  promotion.ImageRef.EventAnnotation,
		RollbackStrategy: promotion.Rollback.Strategy,
		Target:           promotion.Target,
	}
	if promotion.ObjectRef.NamespaceField != "" {
		rt.Namespace = service.Raw[promotion.ObjectRef.NamespaceField]
	}
	return rt, nil
}
