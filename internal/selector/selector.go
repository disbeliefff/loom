package selector

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/disbeliefff/loom/internal/git"
	"github.com/disbeliefff/loom/internal/models"
)

type Evaluator struct {
	logger *slog.Logger
	gitCli *git.Client
}

func NewEvaluator(logger *slog.Logger, gitCli *git.Client) *Evaluator {
	if logger == nil {
		logger = slog.Default()
	}
	if gitCli == nil {
		gitCli = git.NewClient(logger)
	}
	return &Evaluator{
		logger: logger,
		gitCli: gitCli,
	}
}

// Apply uses the given SelectorConfig to filter services and return jobs.
// It switches behavior based on the configured "Type" (e.g., git-diff, regex-tag, env-match).
func (e *Evaluator) Apply(cfg models.SelectorConfig, ctx models.PipelineContext, services []models.Service) ([]models.Job, error) {
	if cfg.Type == "" {
		return nil, fmt.Errorf("selector type is missing")
	}

	switch cfg.Type {
	case "git-diff":
		return e.gitDiff(cfg, ctx, services)
	case "regex-tag":
		return e.regexTag(cfg, ctx, services)
	case "env-match":
		return e.envMatch(cfg, services)
	default:
		return nil, fmt.Errorf("unknown selector type: %s", cfg.Type)
	}
}

func (e *Evaluator) gitDiff(cfg models.SelectorConfig, ctx models.PipelineContext, services []models.Service) ([]models.Job, error) {
	beforeSha := renderTemplate(cfg.BeforeSHA, ctx)
	currentSha := renderTemplate(cfg.CurrentSHA, ctx)

	watchField := cfg.WatchField
	if watchField == "" {
		watchField = "watch_dir" // default
	}

	repoRoot := renderTemplate(cfg.RepoRoot, ctx)
	if repoRoot == "" {
		repoRoot = ctx.RepoRoot
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("repo_root is empty (context repo_root is missing and not configured)")
	}

	changedFiles, err := e.gitCli.GetChangedFiles(repoRoot, beforeSha, currentSha)
	if err != nil {
		return nil, fmt.Errorf("get git diff: %w", err)
	}

	jobs := make([]models.Job, 0, len(services))
	for _, svc := range services {
		watchDir, ok := svc.Raw[watchField]
		if !ok || watchDir == "" {
			continue
		}

		watchPrefix := watchDir
		if !strings.HasSuffix(watchPrefix, "/") {
			watchPrefix += "/"
		}

		matched := false
		for _, f := range changedFiles {
			if strings.HasPrefix(f, watchPrefix) {
				matched = true
				break
			}
		}

		if matched {
			jobs = append(jobs, models.Job{Service: svc})
		}
	}

	return jobs, nil
}

func (e *Evaluator) regexTag(cfg models.SelectorConfig, ctx models.PipelineContext, services []models.Service) ([]models.Job, error) {
	if cfg.Pattern == "" {
		return nil, fmt.Errorf("regex-tag selector requires a 'pattern' string")
	}

	if ctx.CommitTag == "" {
		return nil, nil // No tag, no matches
	}

	tmpl, err := template.New("pattern").Funcs(sprig.TxtFuncMap()).Parse(cfg.Pattern)
	if err != nil {
		return nil, fmt.Errorf("parse pattern template %q: %w", cfg.Pattern, err)
	}

	jobs := make([]models.Job, 0, len(services))
	for _, svc := range services {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]any{"Service": svc}); err != nil {
			return nil, fmt.Errorf("render pattern template: %w", err)
		}

		renderedPattern := buf.String()
		re, err := regexp.Compile(renderedPattern)
		if err != nil {
			return nil, fmt.Errorf("compile regex %q: %w", renderedPattern, err)
		}

		if re.MatchString(ctx.CommitTag) {
			jobs = append(jobs, models.Job{Service: svc})
		}
	}

	return jobs, nil
}

func (e *Evaluator) envMatch(cfg models.SelectorConfig, services []models.Service) ([]models.Job, error) {
	if cfg.Prefix == "" {
		return nil, fmt.Errorf("env-match selector requires a prefix string")
	}

	jobs := make([]models.Job, 0, len(services))
	for _, svc := range services {
		envKey := cfg.Prefix + strings.ToUpper(strings.ReplaceAll(svc.Key, "-", "_"))

		val := os.Getenv(envKey)
		if val == "true" {
			jobs = append(jobs, models.Job{Service: svc})
		}
	}

	return jobs, nil
}

// renderTemplate evaluates strings containing Go templates like `{{ .Context.BeforeSHA }}`
func renderTemplate(val string, ctx models.PipelineContext) string {
	if val == "" {
		return ""
	}

	tmpl, err := template.New("val").Funcs(sprig.TxtFuncMap()).Parse(val)
	if err == nil {
		var buf bytes.Buffer
		err = tmpl.Execute(&buf, map[string]any{"Context": ctx})
		if err == nil {
			return buf.String()
		}
	}
	return val
}
