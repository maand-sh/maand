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

func KV() error {
	// TODO: namespace filter

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

	count := 0
	query := "SELECT count(*) FROM key_value"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return bucket.NotFoundError("key values")
	}
	if err != nil {
		return bucket.DatabaseError(err)
	}

	rows, err := tx.Query(`SELECT namespace, key, value, version, ttl, created_date, deleted FROM cat_kv`)
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

func KVGet(namespace, key string) error {
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

	if kv.IsEncryptedValue(value) {
		value = "[encrypted]"
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
