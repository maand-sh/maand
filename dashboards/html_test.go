package dashboards

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTitleFromHTML(t *testing.T) {
	assert.Equal(t, "Worker detail", TitleFromHTML(`<!DOCTYPE html><html><head><title>Worker detail</title></head></html>`))
	assert.Equal(t, "Overview", TitleFromHTML(`<TITLE> Overview </TITLE>`))
	assert.Equal(t, "", TitleFromHTML(`<html><body>no title</body></html>`))
}

func TestLinkLabel(t *testing.T) {
	withTitle := `<html><head><title>All workers</title></head></html>`
	assert.Equal(t, "All workers", LinkLabel("workers.html", withTitle))
	assert.Equal(t, "overview.html", LinkLabel("overview.html", `<html></html>`))
	assert.Equal(t, "latency.html", LinkLabel("slo/latency.html", ""))
}

func TestRenderIndexHTML(t *testing.T) {
	htmlPage := RenderIndexHTML([]Entry{
		{Job: "api", Rel: "overview.html", Label: "API overview"},
		{Job: "api", Rel: "slo/latency.html", Label: "SLO latency"},
		{Job: "postgres", Rel: "connections.html", Label: "connections.html"},
	})
	assert.Contains(t, htmlPage, `href="api/overview.html"`)
	assert.Contains(t, htmlPage, ">API overview</a>")
	assert.Contains(t, htmlPage, `href="api/slo/latency.html"`)
	assert.Contains(t, htmlPage, ">SLO latency</a>")
	assert.Contains(t, htmlPage, `href="postgres/connections.html"`)
	assert.Contains(t, htmlPage, ">connections.html</a>")
	assert.Contains(t, htmlPage, `<h2>api</h2>`)
	assert.Contains(t, htmlPage, "<h1>Dashboards</h1>")
	assert.Contains(t, htmlPage, `<style>`)
}

func TestRenderIndexHTML_usesTitleFromContent(t *testing.T) {
	htmlPage := RenderIndexHTML([]Entry{
		{
			Job:   "coroot_node_agent",
			Rel:   "workers.html",
			Label: LinkLabel("workers.html", `<html><title>Workers</title></html>`),
		},
	})
	assert.Contains(t, htmlPage, ">Workers</a>")
	assert.NotContains(t, htmlPage, ">workers.html</a>")
}
