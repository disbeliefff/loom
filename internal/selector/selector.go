package selector

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/disbeliefff/loom/internal/git"
	"github.com/disbeliefff/loom/internal/models"
)

// Apply uses the given SelectorConfig to filter services and return jobs.
// It switches behavior based on the configured "Type" (e.g., git-diff, regex-tag, env-match).
func Apply(cfg models.SelectorConfig, ctx models.PipelineContext, services []models.Service) ([]models.Job, error) {
	if cfg.Type == "" {
		return nil, fmt.Errorf("selector 'type' is missing")
	}

	switch cfg.Type {
	case "git-diff":
		return gitDiff(cfg, ctx, services)
	case "regex-tag":
		return regexTag(cfg, ctx, services)
	case "env-match":
		return envMatch(cfg, services)
	default:
		return nil, fmt.Errorf("unknown selector type: %s", cfg.Type)
	}
}

func gitDiff(cfg models.SelectorConfig, ctx models.PipelineContext, services []models.Service) ([]models.Job, error) {
	beforeSha := renderTemplate(cfg.BeforeSHA, ctx)
	currentSha := renderTemplate(cfg.CurrentSHA, ctx)

	watchField := cfg.WatchField
	if watchField == "" {
		watchField = "watch_dir" // default
	}

	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	changedFiles, err := git.GetChangedFiles(pwd, beforeSha, currentSha)
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}

	var jobs []models.Job
	for _, svc := range services {
		watchDir, ok := svc.Raw[watchField].(string)
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

func regexTag(cfg models.SelectorConfig, ctx models.PipelineContext, services []models.Service) ([]models.Job, error) {
	if cfg.Pattern == "" {
		return nil, fmt.Errorf("regex-tag selector requires a 'pattern' string")
	}

	if ctx.CommitTag == "" {
		return nil, nil // No tag, no matches
	}

	tmpl, err := template.New("pattern").Parse(cfg.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern template '%s': %w", cfg.Pattern, err)
	}

	var jobs []models.Job
	for _, svc := range services {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]any{"Service": svc}); err != nil {
			return nil, fmt.Errorf("failed to render pattern template: %w", err)
		}

		renderedPattern := buf.String()
		re, err := regexp.Compile(renderedPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid compiled regex '%s': %w", renderedPattern, err)
		}

		if re.MatchString(ctx.CommitTag) {
			jobs = append(jobs, models.Job{Service: svc})
		}
	}

	return jobs, nil
}

func envMatch(cfg models.SelectorConfig, services []models.Service) ([]models.Job, error) {
	if cfg.Prefix == "" {
		return nil, fmt.Errorf("env-match selector requires a 'prefix' string")
	}

	var jobs []models.Job
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

	tmpl, err := template.New("val").Parse(val)
	if err == nil {
		var buf bytes.Buffer
		err = tmpl.Execute(&buf, map[string]any{"Context": ctx})
		if err == nil {
			return buf.String()
		}
	}
	return val
}
