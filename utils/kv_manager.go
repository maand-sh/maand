package utils

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

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
	var curentTTL int
	var deleted int
	if err := row.Scan(&version, &currentValue, &deleted, &curentTTL); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if deleted == 0 && currentValue == value && curentTTL == ttl {
		return nil
	}

	_, err := tx.Exec(
		`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key, value, namespace, version+1, ttl, store.GlobalUnix, 0,
	)
	return err
}

func (store *KeyValueStore) Get(tx *sql.Tx, namespace, key string) (string, error) {
	query := `SELECT value FROM key_value WHERE namespace = ? AND key = ? AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?) AND deleted = 0`
	row := tx.QueryRow(query, namespace, key, namespace, key)
	var value string
	if err := row.Scan(&value); err != nil {
		return "", fmt.Errorf("%s %s not found", namespace, key)
	}
	return value, nil
}

func (store *KeyValueStore) GetMetadata(tx *sql.Tx, namespace, key string) (string, int, error) {
	row := tx.QueryRow(
		`SELECT value, version FROM key_value
		WHERE namespace = ? AND key = ?
		AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?)
		AND deleted = 0`,
		namespace, key, namespace, key,
	)

	var value string
	var version int
	if err := row.Scan(&value, &version); err != nil {
		if err == sql.ErrNoRows {
			return "", 0, nil
		}
		return "", 0, err
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
	return err
}

func (store *KeyValueStore) GetKeys(tx *sql.Tx, namespace string) ([]string, error) {
	rows, err := tx.Query(
		`SELECT key FROM (
				SELECT namespace, key, max(version), deleted, created_date FROM key_value GROUP BY key, namespace
		   ) t WHERE namespace = ? AND deleted = 0`,
		namespace,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
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
		return err
	}
	defer rows.Close()

	currentTime := time.Now()

	for rows.Next() {
		var namespace, key string
		var version, deleted int
		var createdDate int64

		if err := rows.Scan(&namespace, &key, &version, &deleted, &createdDate); err != nil {
			return err
		}

		createdTime := time.Unix(createdDate, 0)
		if deleted == 1 && currentTime.Sub(createdTime).Hours() >= float64(maxDays*24) {
			if _, err := tx.Exec(`DELETE FROM key_value WHERE namespace = ? AND key = ?`, namespace, key); err != nil {
				return err
			}
		}

		if version > MaxVersionsToKeep {
			if _, err := tx.Exec(
				`DELETE FROM key_value WHERE namespace = ? AND key = ? AND version <= ?`,
				namespace, key, version-MaxVersionsToKeep,
			); err != nil {
				return err
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
