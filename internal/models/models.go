package models

// Service represents a single service from services.json
type Service struct {
	// Key is the unique identifier for the service, usually parsed from "key" field.
	Key string
	// SafeKey is a sanitized version of Key for use in YAML anchors or CI job names.
	SafeKey string
	// Raw contains all fields from the JSON object to be accessible in templates.
	Raw map[string]string
}

type Strategy struct {
	Name      string         `yaml:"name" json:"name"`
	Condition string         `yaml:"condition" json:"condition"`
	Selector  SelectorConfig `yaml:"selector" json:"selector"`
	Template  string         `yaml:"template" json:"template"`
	Promotion *Promotion     `yaml:"promotion" json:"promotion"`
}

type Promotion struct {
	Enabled           bool      `yaml:"enabled" json:"enabled"`
	Provider          string    `yaml:"provider" json:"provider"`
	Mode              string    `yaml:"mode" json:"mode"`
	Target            string    `yaml:"target" json:"target"`
	CheckoutPath      string    `yaml:"checkout_path" json:"checkout_path"`
	ManifestPathField string    `yaml:"manifest_path_field" json:"manifest_path_field"`
	ObjectRef         ObjectRef `yaml:"object_ref" json:"object_ref"`
	ImageRef          ImageRef  `yaml:"image_ref" json:"image_ref"`
	Rollback          Rollback  `yaml:"rollback" json:"rollback"`
}

type ObjectRef struct {
	APIVersion     string `yaml:"api_version" json:"api_version"`
	Kind           string `yaml:"kind" json:"kind"`
	NameField      string `yaml:"name_field" json:"name_field"`
	NamespaceField string `yaml:"namespace_field" json:"namespace_field"`
}

type ImageRef struct {
	RepositoryField string `yaml:"repository_field" json:"repository_field"`
	RepositoryPath  string `yaml:"repository_path" json:"repository_path"`
	TagPath         string `yaml:"tag_path" json:"tag_path"`
	EventAnnotation string `yaml:"event_annotation" json:"event_annotation"`
}

type Rollback struct {
	Strategy string `yaml:"strategy" json:"strategy"`
}

// SelectorConfig holds configuration fields for any type of selector.
type SelectorConfig struct {
	Type       string `yaml:"type" json:"type"`
	BeforeSHA  string `yaml:"before_sha" json:"before_sha"`
	CurrentSHA string `yaml:"current_sha" json:"current_sha"`
	RepoRoot   string `yaml:"repo_root" json:"repo_root"`
	WatchField string `yaml:"watch_field" json:"watch_field"`
	Pattern    string `yaml:"pattern" json:"pattern"`
	Prefix     string `yaml:"prefix" json:"prefix"`
}

// Config represents the pipeline-strategies.yaml structure
type Config struct {
	Strategies []Strategy `yaml:"strategies" json:"strategies"`
}

// PipelineContext represents context variables (like CI variables) to be passed to templates
type PipelineContext struct {
	CommitTag      string
	CommitBranch   string
	PipelineID     string
	PipelineSource string
	RepoRoot       string
	BeforeSHA      string
	CommitSHA      string
	BuildTag       string
}

// TemplateData is the data structure passed to the Go template
type TemplateData struct {
	Jobs    []Job
	Context PipelineContext
}

// Job represents a matched service to be rendered in the template
type Job struct {
	Service Service
}
