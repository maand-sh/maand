// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"fmt"
	"os"
	"path"

	"maand/bucket"

	_ "github.com/mattn/go-sqlite3"
)

const sqliteBusyTimeoutMS = 5000

// DatabasePath returns the path to maand.db under the bucket data directory.
func DatabasePath() string {
	return path.Join(bucket.Location, "data", "maand.db")
}

// DatabaseExists reports whether the bucket database file is present.
func DatabaseExists() bool {
	_, err := os.Stat(DatabasePath())
	return err == nil
}

// OpenDatabase opens the bucket SQLite database.
// When requireExists is true, returns bucket.ErrNotInitialized if maand.db is missing.
func OpenDatabase(requireExists bool) (*sql.DB, error) {
	dbPath := DatabasePath()
	if requireExists {
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return nil, bucket.ErrNotInitialized
		}
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_busy_timeout=%d&_journal_mode=WAL", dbPath, sqliteBusyTimeoutMS))
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	return db, nil
}
