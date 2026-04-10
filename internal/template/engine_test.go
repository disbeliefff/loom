package template_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/disbeliefff/loom/internal/models"
	"github.com/disbeliefff/loom/internal/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPipeline(t *testing.T) {
	tempDir := t.TempDir()

	validTmplPath := filepath.Join(tempDir, "valid.tmpl")
	invalidTmplPath := filepath.Join(tempDir, "invalid.tmpl")
	outputPath := filepath.Join(tempDir, "output.yml")

	validTmplContent := []byte(`
jobs:
{{- range .Jobs }}
  - name: {{ .Service.SafeKey }}
    image: {{ .Service.Raw.image }}
    branch: {{ $.Context.CommitBranch }}
{{- end }}
`)
	require.NoError(t, os.WriteFile(validTmplPath, validTmplContent, 0644))

	invalidTmplContent := []byte(`{{ range .Jobs }}{{ .Missing }}{{ end }}`)
	require.NoError(t, os.WriteFile(invalidTmplPath, invalidTmplContent, 0644))

	ctx := models.PipelineContext{
		CommitBranch: "main",
	}

	jobs := []models.Job{
		{
			Service: models.Service{
				Key:     "auth-service",
				SafeKey: "authservice",
				Raw: map[string]string{
					"image": "docker.io/auth:latest",
				},
			},
		},
	}

	t.Run("Valid rendering to file", func(t *testing.T) {
		err := template.RenderPipeline(validTmplPath, jobs, ctx, outputPath)
		require.NoError(t, err)

		outputBytes, err := os.ReadFile(outputPath)
		require.NoError(t, err)

		expectedOutput := `
jobs:
  - name: authservice
    image: docker.io/auth:latest
    branch: main
`
		assert.Equal(t, expectedOutput, string(outputBytes))
	})

	t.Run("Valid rendering to stdout", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := template.RenderPipeline(validTmplPath, jobs, ctx, "")
		require.NoError(t, err)

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, err = io.Copy(&buf, r)
		require.NoError(t, err)

		expectedOutput := `
jobs:
  - name: authservice
    image: docker.io/auth:latest
    branch: main
`
		assert.Equal(t, expectedOutput, buf.String())
	})

	t.Run("Missing template file", func(t *testing.T) {
		err := template.RenderPipeline("missing.tmpl", jobs, ctx, outputPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read template file")
	})

	t.Run("Execution error in template", func(t *testing.T) {
		err := template.RenderPipeline(invalidTmplPath, jobs, ctx, outputPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "execute template")
	})
}
