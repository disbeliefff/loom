package promotion

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/disbeliefff/loom/internal/models"
	"gopkg.in/yaml.v3"
)

const annotationDomain = "loom.disbeliefff.github.io"

const (
	annPreviousImage = annotationDomain + "/previous-image"
	annPreviousRepo  = annotationDomain + "/previous-repository"
	annPreviousTag   = annotationDomain + "/previous-tag"
	annPromotedAt    = annotationDomain + "/promoted-at"
	annPromotedBy    = annotationDomain + "/promoted-by"
)

var (
	ErrNoMetadata            = errors.New("manifest has no metadata")
	ErrNoPreviousAnnotations = errors.New("no previous promotion annotations found for rollback")
	ErrNoYAMLDocuments       = errors.New("no yaml documents found")
	ErrNoServices            = errors.New("no services found")
	ErrImagePolicyConflict   = errors.New("manifest contains $imagepolicy comments; aborting promotion (conflicts with Flux Image Automation)")
)

type NotFoundError struct {
	Kind string
	Name string
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("%s %q not found", e.Kind, e.Name) }

type ManifestLookupError struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	Path       string
	Reason     string
}

func (e *ManifestLookupError) Error() string {
	return fmt.Sprintf("%s for apiVersion=%q kind=%q name=%q namespace=%q under %q",
		e.Reason, e.APIVersion, e.Kind, e.Name, e.Namespace, e.Path)
}

type MutationResult struct {
	FilePath string
	OldRepo  string
	OldTag   string
	OldEvent string
	NewRepo  string
	NewTag   string
	NewEvent string
}

type ManifestMutator struct {
	target *ResolvedTarget
}

func NewManifestMutator(target *ResolvedTarget) *ManifestMutator {
	return &ManifestMutator{target: target}
}

type FoundManifest struct {
	FilePath   string
	Nodes      []*yaml.Node
	MatchIndex int
}

func (m *ManifestMutator) FindManifest(checkoutPath string) (*FoundManifest, error) {
	fullPath := filepath.Join(checkoutPath, m.target.ManifestPath)

	var yamlFiles []string
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("stat manifest path %q: %w", fullPath, err)
	}

	if info.IsDir() {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read directory %q: %w", fullPath, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext == ".yaml" || ext == ".yml" {
					yamlFiles = append(yamlFiles, filepath.Join(fullPath, entry.Name()))
				}
			}
		}
	} else {
		yamlFiles = []string{fullPath}
	}

	var found *FoundManifest

	for _, fp := range yamlFiles {
		data, err := os.ReadFile(fp)
		if err != nil {
			return nil, fmt.Errorf("read file %q: %w", fp, err)
		}

		nodes, err := decodeDocuments(data)
		if err != nil {
			return nil, fmt.Errorf("parse YAML in %q: %w", fp, err)
		}

		for i, node := range nodes {
			if matchesIdentity(node, m.target) {
				if found != nil {
					return nil, &ManifestLookupError{
						APIVersion: m.target.APIVersion,
						Kind:       m.target.Kind,
						Name:       m.target.Name,
						Namespace:  m.target.Namespace,
						Path:       fullPath,
						Reason:     "multiple manifests found",
					}
				}
				found = &FoundManifest{
					FilePath:   fp,
					Nodes:      nodes,
					MatchIndex: i,
				}
			}
		}
	}

	if found == nil {
		return nil, &ManifestLookupError{
			APIVersion: m.target.APIVersion,
			Kind:       m.target.Kind,
			Name:       m.target.Name,
			Namespace:  m.target.Namespace,
			Path:       fullPath,
			Reason:     "no manifest matching identity found",
		}
	}

	return found, nil
}

func (m *ManifestMutator) Promote(found *FoundManifest, tag string) (*MutationResult, error) {
	node := found.Nodes[found.MatchIndex]
	content := rootMapping(node)

	if hasImagePolicyComments(node) {
		return nil, ErrImagePolicyConflict
	}

	oldRepo := getNodeValue(content, m.target.RepositoryPath)
	oldTag := getNodeValue(content, m.target.TagPath)

	metadata := getMappingValue(content, "metadata")
	annotations, err := getOrEnsureAnnotations(metadata)
	if err != nil {
		return nil, fmt.Errorf("resolve annotations: %w", err)
	}
	oldEvent := getAnnotationValue(annotations, m.target.EventAnnotation)

	newEvent := m.target.Repository + ":" + tag

	if m.target.RollbackStrategy == "previous-annotation" {
		saveRollbackAnnotations(annotations, oldRepo, oldTag, oldEvent)
	}

	if err := setNodeValue(content, m.target.RepositoryPath, m.target.Repository); err != nil {
		return nil, fmt.Errorf("set repository: %w", err)
	}
	if err := setNodeValue(content, m.target.TagPath, tag); err != nil {
		return nil, fmt.Errorf("set tag: %w", err)
	}
	setMappingValue(annotations, m.target.EventAnnotation, newEvent)

	return &MutationResult{
		FilePath: found.FilePath,
		OldRepo:  oldRepo,
		OldTag:   oldTag,
		OldEvent: oldEvent,
		NewRepo:  m.target.Repository,
		NewTag:   tag,
		NewEvent: newEvent,
	}, nil
}

func (m *ManifestMutator) Rollback(found *FoundManifest) (*MutationResult, error) {
	node := found.Nodes[found.MatchIndex]
	content := rootMapping(node)

	metadata := getMappingValue(content, "metadata")
	if metadata == nil {
		return nil, ErrNoMetadata
	}
	annotations := getMappingValue(metadata, "annotations")
	if annotations == nil {
		return nil, ErrNoPreviousAnnotations
	}

	prevRepo := getAnnotationValue(annotations, annPreviousRepo)
	prevTag := getAnnotationValue(annotations, annPreviousTag)
	if prevRepo == "" || prevTag == "" {
		return nil, ErrNoPreviousAnnotations
	}

	curRepo := getNodeValue(content, m.target.RepositoryPath)
	curTag := getNodeValue(content, m.target.TagPath)
	curEvent := getAnnotationValue(annotations, m.target.EventAnnotation)

	newEvent := prevRepo + ":" + prevTag

	if err := setNodeValue(content, m.target.RepositoryPath, prevRepo); err != nil {
		return nil, fmt.Errorf("set repository: %w", err)
	}
	if err := setNodeValue(content, m.target.TagPath, prevTag); err != nil {
		return nil, fmt.Errorf("set tag: %w", err)
	}
	setMappingValue(annotations, m.target.EventAnnotation, newEvent)

	saveRollbackAnnotations(annotations, curRepo, curTag, curEvent)

	return &MutationResult{
		FilePath: found.FilePath,
		OldRepo:  curRepo,
		OldTag:   curTag,
		OldEvent: curEvent,
		NewRepo:  prevRepo,
		NewTag:   prevTag,
		NewEvent: newEvent,
	}, nil
}

func saveRollbackAnnotations(annotations *yaml.Node, repo, tag, event string) {
	setMappingValue(annotations, annPreviousImage, event)
	setMappingValue(annotations, annPreviousRepo, repo)
	setMappingValue(annotations, annPreviousTag, tag)
	setMappingValue(annotations, annPromotedAt, time.Now().UTC().Format(time.RFC3339))
	setMappingValue(annotations, annPromotedBy, getActor())
}

func WriteManifest(found *FoundManifest) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	for _, node := range found.Nodes {
		if err := encoder.Encode(node); err != nil {
			return fmt.Errorf("encode YAML: %w", err)
		}
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encode YAML: %w", err)
	}
	return os.WriteFile(found.FilePath, buf.Bytes(), 0600)
}

func decodeDocuments(data []byte) ([]*yaml.Node, error) {
	var nodes []*yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode YAML document: %w", err)
		}
		nodes = append(nodes, &node)
	}
	if len(nodes) == 0 {
		return nil, ErrNoYAMLDocuments
	}
	return nodes, nil
}

func matchesIdentity(node *yaml.Node, target *ResolvedTarget) bool {
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return false
	}
	content := node.Content[0]
	if content.Kind != yaml.MappingNode {
		return false
	}
	if v := getMappingValue(content, "apiVersion"); v == nil || v.Value != target.APIVersion {
		return false
	}
	if v := getMappingValue(content, "kind"); v == nil || v.Value != target.Kind {
		return false
	}
	metadata := getMappingValue(content, "metadata")
	if metadata == nil || metadata.Kind != yaml.MappingNode {
		return false
	}
	if v := getMappingValue(metadata, "name"); v == nil || v.Value != target.Name {
		return false
	}
	if target.Namespace != "" {
		if v := getMappingValue(metadata, "namespace"); v == nil || v.Value != target.Namespace {
			return false
		}
	}
	return true
}

func rootMapping(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func getMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func getOrEnsureAnnotations(metadata *yaml.Node) (*yaml.Node, error) {
	if metadata == nil {
		return nil, fmt.Errorf("manifest metadata is nil")
	}
	annotations := getMappingValue(metadata, "annotations")
	if annotations != nil {
		if annotations.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("metadata.annotations is not a mapping (kind=%d)", annotations.Kind)
		}
		return annotations, nil
	}
	annotations = &yaml.Node{Kind: yaml.MappingNode}
	metadata.Content = append(metadata.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "annotations"},
		annotations,
	)
	return annotations, nil
}

func getAnnotationValue(annotations *yaml.Node, key string) string {
	if v := getMappingValue(annotations, key); v != nil {
		return v.Value
	}
	return ""
}

func setMappingValue(mapping *yaml.Node, key, value string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = value
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

func walkPath(node *yaml.Node, path string) (*yaml.Node, string, error) {
	keys := strings.Split(path, ".")
	current := node
	for _, key := range keys[:len(keys)-1] {
		next := getMappingValue(current, key)
		if next == nil {
			return nil, "", fmt.Errorf("key %q not found", key)
		}
		current = next
	}
	return current, keys[len(keys)-1], nil
}

func getNodeValue(node *yaml.Node, path string) string {
	parent, last, err := walkPath(node, path)
	if err != nil {
		return ""
	}
	if v := getMappingValue(parent, last); v != nil {
		return v.Value
	}
	return ""
}

func setNodeValue(node *yaml.Node, path, value string) error {
	parent, last, err := walkPath(node, path)
	if err != nil {
		return err
	}
	setMappingValue(parent, last, value)
	return nil
}

func hasImagePolicyComments(node *yaml.Node) bool {
	return containsImagePolicy(node)
}

func containsImagePolicy(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if strings.Contains(node.HeadComment, "$imagepolicy") ||
		strings.Contains(node.LineComment, "$imagepolicy") ||
		strings.Contains(node.FootComment, "$imagepolicy") {
		return true
	}
	return slices.ContainsFunc(node.Content, containsImagePolicy)
}

func getActor() string {
	if actor := os.Getenv("GITLAB_USER_LOGIN"); actor != "" {
		return actor
	}
	if actor := os.Getenv("GITHUB_ACTOR"); actor != "" {
		return actor
	}
	return "loom"
}

func findByName[T any](items []T, name string, extract func(T) string, kind string) (*T, error) {
	for i := range items {
		if extract(items[i]) == name {
			return &items[i], nil
		}
	}
	return nil, &NotFoundError{Kind: kind, Name: name}
}

func FindStrategy(cfg *models.Config, name string) (*models.Strategy, error) {
	return findByName(cfg.Strategies, name, func(s models.Strategy) string { return s.Name }, "strategy")
}

func ResolveService(services []models.Service, serviceName string) (*models.Service, error) {
	if len(services) == 0 {
		return nil, ErrNoServices
	}
	if serviceName == "" {
		if len(services) == 1 {
			return &services[0], nil
		}
		return nil, fmt.Errorf("--service is required when multiple services are defined")
	}
	return findByName(services, serviceName, func(s models.Service) string { return s.Key }, "service")
}
