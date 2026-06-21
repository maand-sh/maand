// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/promconfig"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

func initPrometheusCat() error {
	return bucket.ResolveRoot()
}

func Prometheus(jobsCSV string) error {
	if err := initPrometheusCat(); err != nil {
		return err
	}

	db, err := data.OpenDatabase(true)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	jobsFilter := parseCSVFilter(jobsCSV)
	if err := validatePrometheusJobFilter(tx, jobsFilter); err != nil {
		return err
	}

	dbSummaries, err := data.ListPrometheusJobSummaries(tx, jobsFilter)
	if err != nil {
		return err
	}
	wsSummaries, err := promconfig.ListWorkspacePrometheusSummaries(jobsFilter)
	if err != nil {
		return err
	}
	summaries := promconfig.MergePrometheusSummaries(dbSummaries, wsSummaries)
	if len(summaries) == 0 {
		return bucket.NotFoundError("prometheus")
	}

	t := utils.GetTable(table.Row{"job", "scrape", "alerts", "runbooks"})
	for _, summary := range summaries {
		t.AppendRows([]table.Row{{
			summary.Job,
			boolMark(summary.Scrape),
			strconv.Itoa(summary.Alerts),
			strconv.Itoa(summary.Runbooks),
		}})
	}
	t.Render()

	return tx.Commit()
}

func PrometheusGet(job, relPath string) error {
	if err := initPrometheusCat(); err != nil {
		return err
	}

	relPath = promconfig.NormalizePrometheusRelPath(relPath)

	content, wsErr := promconfig.ReadWorkspacePrometheusFile(job, relPath)
	if wsErr == nil {
		return writePrometheusContent(content)
	}
	if !isNotFound(wsErr) {
		return wsErr
	}

	db, err := data.OpenDatabase(true)
	if err != nil {
		return wsErr
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := validatePrometheusJobFilter(tx, []string{job}); err != nil {
		return err
	}

	content, err = data.GetPrometheusFileContent(tx, job, relPath)
	if err != nil {
		return wsErr
	}

	return writePrometheusContent(content)
}

func PrometheusScrape(jobsCSV string) error {
	if err := initPrometheusCat(); err != nil {
		return err
	}

	db, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	jobsFilter := parseCSVFilter(jobsCSV)
	if err := validatePrometheusJobFilter(tx, jobsFilter); err != nil {
		return err
	}

	if err := kv.Initialize(tx); err != nil {
		return err
	}

	yamlFragment, err := promconfig.RenderScrapeConfigsYAML(tx, jobsFilter)
	if err != nil {
		return err
	}
	if strings.TrimSpace(yamlFragment) == "" {
		return fmt.Errorf("%w: no expanded scrape configs (run maand build first)", bucket.ErrNotFound)
	}

	_, err = os.Stdout.WriteString(strings.TrimPrefix(yamlFragment, "\n"))
	return err
}

func writePrometheusContent(content string) error {
	_, err := os.Stdout.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		_, _ = os.Stdout.WriteString("\n")
	}
	return err
}

func validatePrometheusJobFilter(tx *sql.Tx, jobsFilter []string) error {
	if len(jobsFilter) == 0 {
		return nil
	}
	known, err := knownJobNames(tx)
	if err != nil {
		return err
	}
	matched := utils.Intersection(known, jobsFilter)
	for _, job := range jobsFilter {
		if promconfig.JobHasWorkspacePrometheus(job) {
			matched = utils.Unique(append(matched, job))
		}
	}
	if len(matched) == 0 {
		return fmt.Errorf("invalid input, jobs %v", jobsFilter)
	}
	return nil
}

func knownJobNames(tx *sql.Tx) ([]string, error) {
	dbJobs, err := data.GetJobs(tx)
	if err != nil {
		return nil, err
	}
	wsJobs, err := promconfig.ListWorkspaceJobNames()
	if err != nil {
		return nil, err
	}
	return utils.Unique(append(dbJobs, wsJobs...)), nil
}

func isNotFound(err error) bool {
	return errors.Is(err, bucket.ErrNotFound)
}

func boolMark(v bool) string {
	if v {
		return "yes"
	}
	return "-"
}
