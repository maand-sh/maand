// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package runbooks

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed style.css
var StyleCSS []byte

// Entry identifies one runbook in the catalog.
type Entry struct {
	Job  string
	Slug string
}

var markdownConverter = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
)

// MarkdownToHTML converts runbook markdown to an HTML fragment.
func MarkdownToHTML(content string) (string, error) {
	var buf bytes.Buffer
	if err := markdownConverter.Convert([]byte(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// EscapePrometheusTemplateLiterals neutralizes {{ and }} so Prometheus console
// rendering does not execute runbook examples (e.g. docker --format "{{.Names}}").
func EscapePrometheusTemplateLiterals(s string) string {
	const open = "\x00prom_tpl_open\x00"
	const close = "\x00prom_tpl_close\x00"
	s = strings.ReplaceAll(s, "{{", open)
	s = strings.ReplaceAll(s, "}}", close)
	s = strings.ReplaceAll(s, open, `{{ "{{" }}`)
	s = strings.ReplaceAll(s, close, `{{ "}}" }}`)
	return s
}

// RenderHTMLPage wraps body HTML in a full document with embedded styles.
// CSS is inlined so runbooks render correctly when served as Prometheus consoles.
func RenderHTMLPage(title, bodyHTML string) string {
	title = EscapePrometheusTemplateLiterals(html.EscapeString(title))
	bodyHTML = EscapePrometheusTemplateLiterals(bodyHTML)
	css := EscapePrometheusTemplateLiterals(string(StyleCSS))
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
%s
</style>
</head>
<body>
<main>
%s
</main>
</body>
</html>
`, title, css, bodyHTML)
}

// RenderIndexHTML builds the runbook index page for static hosting under runbooks/.
func RenderIndexHTML(entries []Entry) string {
	byJob := make(map[string][]string)
	jobs := make([]string, 0)
	for _, entry := range entries {
		if _, ok := byJob[entry.Job]; !ok {
			jobs = append(jobs, entry.Job)
		}
		byJob[entry.Job] = append(byJob[entry.Job], entry.Slug)
	}
	sort.Strings(jobs)

	var body strings.Builder
	body.WriteString(`<div class="runbook-index">`)
	body.WriteString("<h1>Runbooks</h1>\n")
	for _, job := range jobs {
		slugs := byJob[job]
		sort.Strings(slugs)
		fmt.Fprintf(&body, "<h2>%s</h2>\n<ul>\n", html.EscapeString(job))
		for _, slug := range slugs {
			fmt.Fprintf(&body, `<li><a href="%s/%s.html">%s</a></li>`,
				html.EscapeString(job), html.EscapeString(slug), html.EscapeString(slug))
		}
		body.WriteString("</ul>\n")
	}
	body.WriteString("</div>\n")
	return RenderHTMLPage("Runbooks", body.String())
}

// RenderRunbookHTML builds one runbook page for runbooks/<job>/<slug>.html.
func RenderRunbookHTML(job, slug, markdown string) (string, error) {
	content, err := MarkdownToHTML(markdown)
	if err != nil {
		return "", err
	}

	var body strings.Builder
	fmt.Fprintf(&body, `<p class="runbook-meta"><a href="../index.html">Runbooks</a> / %s / %s</p>`,
		html.EscapeString(job), html.EscapeString(slug))
	body.WriteString(content)

	title := fmt.Sprintf("%s / %s", job, slug)
	return RenderHTMLPage(title, body.String()), nil
}
