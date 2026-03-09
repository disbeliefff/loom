package template

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/disbeliefff/loom/internal/models"
)

// RenderPipeline processes the selected services and renders the Go template.
// Output is written to the provided outputFile or os.Stdout if empty.
func RenderPipeline(templatePath string, jobs []models.Job, ctx models.PipelineContext, outputPath string) error {
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file '%s': %w", templatePath, err)
	}

	tmpl, err := template.New("pipeline").Funcs(sprig.TxtFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := models.TemplateData{
		Jobs:    jobs,
		Context: ctx,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
			return fmt.Errorf("failed to write output to file '%s': %w", outputPath, err)
		}
	} else {
		fmt.Print(buf.String())
	}

	return nil
}
