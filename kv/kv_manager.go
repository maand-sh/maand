// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package kv provides interfaces to work with kv store
package kv

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"maand/data"

	_ "github.com/mattn/go-sqlite3"
)

const (
	MaxVersionsToKeep = 7
)

type KeyValueStore struct {
	GlobalUnix int64
	mutex      sync.Mutex
}

func (store *KeyValueStore) Put(tx *sql.Tx, namespace, key, value string, ttl int) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	var version int
	row := tx.QueryRow(
		"SELECT max(version), value, deleted, ttl FROM key_value WHERE namespace = ? AND key = ? GROUP BY key, namespace",
		namespace, key,
	)

	var currentValue string
	var currentTTL, deleted int

	err := row.Scan(&version, &currentValue, &deleted, &currentTTL)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return data.NewDatabaseError(err)
		}
	}

	if deleted == 0 && currentValue == value && currentTTL == ttl {
		return nil
	}

	_, err = tx.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key, value, namespace, version+1, ttl, store.GlobalUnix, 0,
	)
	if err != nil {
		return data.NewDatabaseError(err)
	}
	return nil
}

func (store *KeyValueStore) Get(tx *sql.Tx, namespace, key string) (string, error) {
	query := `SELECT value FROM key_value WHERE namespace = ? AND key = ? 
                    AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?) AND deleted = 0`

	var value string
	row := tx.QueryRow(query, namespace, key, namespace, key)
	err := row.Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", NewNotFoundError(namespace, key)
		}
		return "", data.NewDatabaseError(err)
	}

	return value, nil
}

func (store *KeyValueStore) GetMetadata(tx *sql.Tx, namespace, key string) (string, int, error) {
	row := tx.QueryRow(
		`SELECT value, version FROM key_value WHERE namespace = ? AND key = ? 
                    AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?) AND deleted = 0`,
		namespace, key, namespace, key,
	)

	var value string
	var version int

	if err := row.Scan(&value, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, nil
		}
		return "", 0, data.NewDatabaseError(err)
	}
	return value, version, nil
}

func (store *KeyValueStore) Delete(tx *sql.Tx, namespace, key string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	_, err := tx.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) SELECT key, value, namespace, max(version) + 1, ttl, created_date, 1 FROM key_value WHERE namespace = ? AND key = ? GROUP BY key, namespace`,
		namespace, key,
	)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	return nil
}

func (store *KeyValueStore) GetKeys(tx *sql.Tx, namespace string) ([]string, error) {
	rows, err := tx.Query(
		`SELECT key FROM (
				SELECT namespace, key, max(version), deleted, created_date FROM key_value GROUP BY key, namespace
		   ) t WHERE namespace = ? AND deleted = 0`,
		namespace,
	)
	if err != nil {
		return nil, data.NewDatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, data.NewDatabaseError(err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (store *KeyValueStore) GC(tx *sql.Tx, maxDays int) error {
	rows, err := tx.Query(
		`SELECT namespace, key, max(CAST(version AS INTEGER)), deleted, created_date FROM key_value GROUP BY key, namespace`,
	)
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	currentTime := time.Now()

	for rows.Next() {
		var namespace, key string
		var version, deleted int
		var createdDate int64

		err := rows.Scan(&namespace, &key, &version, &deleted, &createdDate)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		createdTime := time.Unix(createdDate, 0)
		if deleted == 1 && currentTime.Sub(createdTime).Hours() >= float64(maxDays*24) {
			_, err := tx.Exec(`DELETE FROM key_value WHERE namespace = ? AND key = ?`, namespace, key)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}

		if version > MaxVersionsToKeep {
			_, err := tx.Exec(`DELETE FROM key_value WHERE namespace = ? AND key = ? AND version <= ?`, namespace, key, version-MaxVersionsToKeep)
			if err != nil {
				return data.NewDatabaseError(err)
			}
		}
	}
	return nil
}

func GetKVStore() *KeyValueStore {
	return kvStore
}

var kvStore *KeyValueStore

func init() {
	kvStore = &KeyValueStore{
		GlobalUnix: time.Now().Unix(),
		mutex:      sync.Mutex{},
	}
}
