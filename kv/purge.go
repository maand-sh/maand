// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"database/sql"

	"maand/bucket"
)

const purgeStaleVersionsQuery = `
DELETE FROM key_value
WHERE EXISTS (
	SELECT 1
	FROM (
		SELECT key, namespace, MAX(version) AS latest_version, deleted, created_date
		FROM key_value
		GROUP BY key, namespace
	) AS latest
	WHERE key_value.key = latest.key
	  AND key_value.namespace = latest.namespace
	  AND (
	    key_value.version < latest.latest_version - ?
	    OR latest.deleted = 1
	  )
	  AND latest.created_date < strftime('%s', 'now') - ? * 24 * 60 * 60
)`

// PurgeStaleVersions removes old key_value rows beyond MaxVersionsToKeep and optional age.
// retainDays is how long deleted/stale rows are kept; values below 0 are treated as 0.
func (s *Store) PurgeStaleVersions(tx *sql.Tx, retainDays int) error {
	if retainDays < 0 {
		retainDays = 0
	}
	_, err := tx.Exec(purgeStaleVersionsQuery, MaxVersionsToKeep, retainDays)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

// GC is deprecated; use PurgeStaleVersions.
func (s *Store) GC(tx *sql.Tx, retainDays int) error {
	return s.PurgeStaleVersions(tx, retainDays)
}
