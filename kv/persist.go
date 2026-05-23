// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"database/sql"
	"time"

	"maand/bucket"
	"maand/data"
)

const loadLatestEntriesQuery = `
SELECT namespace, key, value, version, ttl, deleted, created_date
FROM key_value AS outer_kv
WHERE version = (
	SELECT MAX(inner_kv.version)
	FROM key_value AS inner_kv
	WHERE inner_kv.namespace = outer_kv.namespace AND inner_kv.key = outer_kv.key
)`

// LoadFromTransaction hydrates a store from the latest row per namespace/key.
func LoadFromTransaction(tx *sql.Tx) (*Store, error) {
	store := NewStore()

	rows, err := tx.Query(loadLatestEntriesQuery)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	store.mu.Lock()
	defer store.mu.Unlock()

	for rows.Next() {
		var namespace, key, value string
		var version, ttl, deleted int
		var createdAt int64
		if err := rows.Scan(&namespace, &key, &value, &version, &ttl, &deleted, &createdAt); err != nil {
			return nil, bucket.DatabaseError(err)
		}

		ns := store.namespaceMap(namespace)
		ns[key] = &Entry{
			Value:            value,
			Version:          version,
			TTL:              ttl,
			Deleted:          deleted,
			LastModifiedTime: createdAt,
			Changed:          false,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, bucket.DatabaseError(err)
	}

	return store, nil
}

// PersistToTransaction writes changed entries as new version rows.
func PersistToTransaction(tx *sql.Tx, store *Store) error {
	createdAt := time.Now().Unix()

	store.mu.Lock()
	defer store.mu.Unlock()

	for namespace, keys := range store.namespaces {
		for key, entry := range keys {
			if !entry.Changed {
				continue
			}
			_, err := tx.Exec(
				`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				key, entry.Value, namespace, entry.Version, entry.TTL, createdAt, entry.Deleted,
			)
			if err != nil {
				return bucket.DatabaseError(err)
			}
			entry.Changed = false
			entry.LastModifiedTime = createdAt
		}
	}
	return nil
}

// NewKeyValueStore is deprecated; use LoadFromTransaction.
func NewKeyValueStore(tx *sql.Tx) (*Store, error) {
	return LoadFromTransaction(tx)
}

// SaveKeyValueStore is deprecated; use PersistToTransaction.
func SaveKeyValueStore(tx *sql.Tx, store *Store) error {
	return PersistToTransaction(tx, store)
}

// PersistToSessionTransaction writes pending session KV changes into tx.
// Use during deploy while the main catalog transaction is open; changes commit with that tx.
func PersistToSessionTransaction(tx *sql.Tx) error {
	store, err := RequireStore()
	if err != nil {
		return err
	}
	if !store.HasPendingChanges() {
		return nil
	}
	return PersistToTransaction(tx, store)
}

// PersistSession writes pending session KV changes to maand.db in a committed transaction.
// Use when no other write transaction holds the database (for example after rolling back a
// read-only or aborted transaction). During deploy, use PersistToSessionTransaction instead.
func PersistSession() error {
	store, err := RequireStore()
	if err != nil {
		return err
	}
	if !store.HasPendingChanges() {
		return nil
	}

	persistDB, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = persistDB.Close()
	}()

	tx, err := persistDB.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := PersistToTransaction(tx, store); err != nil {
		return err
	}
	return tx.Commit()
}
