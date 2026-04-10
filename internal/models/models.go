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

// Strategy represents a single generation strategy from pipeline-strategies.yaml
type Strategy struct {
	Name      string         `yaml:"name" json:"name"`
	Condition string         `yaml:"condition" json:"condition"`
	Selector  SelectorConfig `yaml:"selector" json:"selector"`
	Template  string         `yaml:"template" json:"template"`
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
