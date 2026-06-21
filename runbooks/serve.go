// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package runbooks

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/promconfig"

	"github.com/yuin/goldmark"
)

//go:embed style.css
var styleCSS string

// Serve starts an HTTP server that serves runbooks from job_files.
func Serve(addr string) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		writeIndex(w, db)
	})
	mux.HandleFunc("GET /{job}/{slug}", func(w http.ResponseWriter, r *http.Request) {
		serveRunbook(w, r, db, false)
	})
	mux.HandleFunc("GET /{job}/{slug}/raw", func(w http.ResponseWriter, r *http.Request) {
		serveRunbook(w, r, db, true)
	})
	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(styleCSS))
	})

	fmt.Printf("Serving runbooks at %s\n", serveURL(addr))

	return http.ListenAndServe(addr, mux)
}

func serveURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func writeIndex(w http.ResponseWriter, db *sql.DB) {
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	paths, err := data.ListRunbookFiles(tx)
	_ = tx.Rollback()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var body strings.Builder
	body.WriteString("<h1>Runbooks</h1>\n<ul>\n")
	for _, filePath := range paths {
		job, slug, ok := promconfig.RunbookSlugFromPath(filePath)
		if !ok {
			continue
		}
		fmt.Fprintf(&body, "<li><a href=\"/%s/%s\">%s / %s</a></li>\n",
			html.EscapeString(job), html.EscapeString(slug),
			html.EscapeString(job), html.EscapeString(slug))
	}
	body.WriteString("</ul>\n")
	_, _ = w.Write([]byte(renderHTMLPage("Runbooks", body.String())))
}

func serveRunbook(w http.ResponseWriter, r *http.Request, db *sql.DB, raw bool) {
	job := r.PathValue("job")
	slug := r.PathValue("slug")
	if strings.TrimSpace(job) == "" || strings.TrimSpace(slug) == "" {
		http.NotFound(w, r)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filePath, err := data.RunbookLookup(tx, job, slug)
	_ = tx.Rollback()
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	content, err := readRunbookContent(db, filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if raw {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(content))
		return
	}

	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(content), &buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(renderHTMLPage(fmt.Sprintf("%s / %s", job, slug), buf.String())))
}

func renderHTMLPage(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<link rel="stylesheet" href="/style.css">
</head>
<body>
<main>
%s</main>
</body>
</html>
`, html.EscapeString(title), body)
}

func readRunbookContent(db *sql.DB, filePath string) (string, error) {
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	return data.GetJobFileContent(tx, filePath)
}
