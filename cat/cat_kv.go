// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"errors"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
	"strings"
)

func KV() error {

	// TODO: namespace filter

	db, err := data.GetDatabase(true)
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	count := 0
	query := "SELECT count(*) FROM key_value"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return &NotFoundError{Domain: "key values"}
	}
	if err != nil {
		return data.NewDatabaseError(err)
	}

	rows, err := tx.Query(`SELECT namespace, key, value, version, ttl, created_date, deleted FROM cat_kv`)
	if err != nil {
		return data.NewDatabaseError(err)
	}

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
			return data.NewDatabaseError(err)
		}

		if strings.HasPrefix(key, "certs/") {
			value = strings.Split(value, "\n")[0]
		}

		t.AppendRows([]table.Row{{namespace, key, value, version, ttl, createdDate, deleted}})
	}

	t.Render()

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return nil
}
