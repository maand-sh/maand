// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

func KV(jobsCSV string, activeOnly, deletedOnly bool) error {
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
	if err := validateKVJobFilter(tx, jobsFilter); err != nil {
		return err
	}

	namespaces, err := jobKVListNamespaces(tx, jobsFilter)
	if err != nil {
		return err
	}
	if len(jobsFilter) > 0 && len(namespaces) == 0 {
		return bucket.NotFoundError("key values")
	}

	where, err := kvListWhere(activeOnly, deletedOnly, namespaces)
	if err != nil {
		return err
	}

	count := 0
	row := tx.QueryRow("SELECT count(*) FROM cat_kv" + where)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return bucket.NotFoundError("key values")
	}
	if err != nil {
		return bucket.DatabaseError(err)
	}

	rows, err := tx.Query(`SELECT namespace, key, value, version, ttl, created_date, deleted FROM cat_kv` + where)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	t := utils.GetTable(table.Row{"Namespace", "Key", "Value", "Version", "ttl", "createdDate", "deleted"})

	for rows.Next() {
		var namespace string
		var key string
		var value string
		var version string
		var ttl int
		var createdDate string
		var deleted int

		err = rows.Scan(&namespace, &key, &value, &version, &ttl, &createdDate, &deleted)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		if strings.HasPrefix(key, "certs/") {
			value = strings.Split(value, "\n")[0]
		}
		if kv.IsEncryptedValue(value) {
			value = "[encrypted]"
		}

		t.AppendRows([]table.Row{{namespace, key, value, version, ttl, createdDate, deleted}})
	}
	if err := data.RowsErr(rows); err != nil {
		return err
	}

	t.Render()

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}

func KVGet(namespace, key string, reveal bool) error {
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

	var value string
	var version, ttl, deleted int
	var createdDate string

	row := tx.QueryRow(`
		SELECT value, max(version), ttl, created_date, deleted
		FROM key_value
		WHERE namespace = ? AND key = ?
		GROUP BY namespace, key`, namespace, key)
	err = row.Scan(&value, &version, &ttl, &createdDate, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return bucket.KeyNotFoundError(namespace, key)
	}
	if err != nil {
		return bucket.DatabaseError(err)
	}
	if deleted == 1 {
		return bucket.KeyNotFoundError(namespace, key)
	}

	value, err = formatKVValue(namespace, key, value, reveal)
	if err != nil {
		return err
	}

	fmt.Printf("namespace: %s\n", namespace)
	fmt.Printf("key: %s\n", key)
	fmt.Printf("value: %s\n", value)
	fmt.Printf("version: %d\n", version)
	fmt.Printf("ttl: %d\n", ttl)
	fmt.Printf("created_date: %s\n", createdDate)

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}

func validateKVJobFilter(tx *sql.Tx, jobsFilter []string) error {
	if len(jobsFilter) == 0 {
		return nil
	}
	allJobs, err := data.GetAllAllocatedJobs(tx)
	if err != nil {
		return err
	}
	if len(utils.Intersection(allJobs, jobsFilter)) == 0 {
		return fmt.Errorf("invalid input, jobs %v", jobsFilter)
	}
	return nil
}

func jobKVListNamespaces(tx *sql.Tx, jobsFilter []string) ([]string, error) {
	if len(jobsFilter) == 0 {
		return nil, nil
	}

	namespaces := make([]string, 0)
	for _, job := range jobsFilter {
		allowed, err := data.AccessibleKVNamespacesForJob(tx, job)
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, allowed...)
	}
	return utils.Unique(namespaces), nil
}

func kvListWhere(activeOnly, deletedOnly bool, namespaces []string) (string, error) {
	if activeOnly && deletedOnly {
		return "", fmt.Errorf("cannot use --active and --deleted together")
	}

	var conditions []string
	if len(namespaces) > 0 {
		conditions = append(conditions, fmt.Sprintf("namespace IN ('%s')", strings.Join(namespaces, "','")))
	}
	switch {
	case activeOnly:
		conditions = append(conditions, "deleted = 0")
	case deletedOnly:
		conditions = append(conditions, "deleted = 1")
	}
	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), nil
}

func formatKVValue(namespace, key, value string, reveal bool) (string, error) {
	if !kv.IsEncryptedValue(value) {
		return value, nil
	}
	if !reveal {
		return "[encrypted]", nil
	}
	plaintext, err := kv.DecryptStoredValue(value)
	if err != nil {
		return "", fmt.Errorf("decrypt %s/%s: %w", namespace, key, err)
	}
	return plaintext, nil
}
