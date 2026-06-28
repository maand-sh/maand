// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dashboards

import (
	"fmt"
	"html"
	"path"
	"regexp"
	"sort"
	"strings"

	"maand/runbooks"
)

var titleTagPattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// Entry identifies one dashboard page in the catalog.
type Entry struct {
	Job   string
	Rel   string // path under dashboards/, e.g. overview.html or slo/latency.html
	Label string // link text: HTML title or filename fallback
}

// TitleFromHTML returns the trimmed contents of the first <title> tag, if any.
func TitleFromHTML(content string) string {
	match := titleTagPattern.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// LinkLabel returns the dashboard link text: HTML title when present, else the file name.
func LinkLabel(rel, content string) string {
	if title := TitleFromHTML(content); title != "" {
		return title
	}
	label := path.Base(rel)
	if label == "" || label == "." {
		return rel
	}
	return label
}

// RenderIndexHTML builds the dashboard index page for static hosting under dashboards/.
func RenderIndexHTML(entries []Entry) string {
	byJob := make(map[string][]Entry)
	jobs := make([]string, 0)
	for _, entry := range entries {
		if entry.Label == "" {
			entry.Label = LinkLabel(entry.Rel, "")
		}
		if _, ok := byJob[entry.Job]; !ok {
			jobs = append(jobs, entry.Job)
		}
		byJob[entry.Job] = append(byJob[entry.Job], entry)
	}
	sort.Strings(jobs)

	var body strings.Builder
	body.WriteString(`<div class="runbook-index">`)
	body.WriteString("<h1>Dashboards</h1>\n")
	for _, job := range jobs {
		jobEntries := byJob[job]
		sort.Slice(jobEntries, func(i, j int) bool {
			return jobEntries[i].Rel < jobEntries[j].Rel
		})
		fmt.Fprintf(&body, "<h2>%s</h2>\n<ul>\n", html.EscapeString(job))
		for _, entry := range jobEntries {
			fmt.Fprintf(&body, `<li><a href="%s/%s">%s</a></li>`,
				html.EscapeString(entry.Job), html.EscapeString(entry.Rel), html.EscapeString(entry.Label))
		}
		body.WriteString("</ul>\n")
	}
	body.WriteString("</div>\n")
	return runbooks.RenderHTMLPage("Dashboards", body.String())
}
