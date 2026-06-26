package runbooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderRunbookHTML(t *testing.T) {
	htmlPage, err := RenderRunbookHTML("postgres", "deadlocks", "# Deadlocks\n\nCheck locks.\n")
	require.NoError(t, err)
	assert.Contains(t, htmlPage, "<h1>Deadlocks</h1>")
	assert.Contains(t, htmlPage, "<style>")
	assert.Contains(t, htmlPage, "fonts.googleapis.com")
	assert.Contains(t, htmlPage, `href="../index.html"`)
	assert.Contains(t, htmlPage, "<title>postgres / deadlocks</title>")
}

func TestRenderIndexHTML(t *testing.T) {
	htmlPage := RenderIndexHTML([]Entry{
		{Job: "api", Slug: "ApiDown"},
		{Job: "postgres", Slug: "deadlocks"},
		{Job: "api", Slug: "ApiSlow"},
	})
	assert.Contains(t, htmlPage, `href="api/ApiDown.html"`)
	assert.Contains(t, htmlPage, `href="postgres/deadlocks.html"`)
	assert.Contains(t, htmlPage, `<h2>api</h2>`)
	assert.Contains(t, htmlPage, `<style>`)
}

func TestEscapePrometheusTemplateLiterals(t *testing.T) {
	in := `docker ps --format "table {{.Names}}\t{{.Status}}"`
	out := EscapePrometheusTemplateLiterals(in)
	assert.NotContains(t, out, "{{.Names}}")
	assert.Contains(t, out, `{{ "{{" }}`)
	assert.Contains(t, out, `{{ "}}" }}`)
}

func TestRenderRunbookHTML_escapesPrometheusTemplates(t *testing.T) {
	md := "# Container\n\n```bash\ndocker ps --format \"table {{.Names}}\\t{{.Status}}\"\n```\n"
	htmlPage, err := RenderRunbookHTML("node_agent", "container_restarting", md)
	require.NoError(t, err)
	assert.NotContains(t, htmlPage, "{{.Names}}")
	assert.Contains(t, htmlPage, `{{ "{{" }}`)
	assert.Contains(t, htmlPage, "<pre>")
}
