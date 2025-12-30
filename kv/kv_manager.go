// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package kv provides kv functions
package kv

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"maand/bucket"
)

// ErrValueNotFound is returned when a key is requested but is not found
// or has been marked as deleted.
var (
	ErrValueNotFound  = errors.New("key not found or deleted")
	MaxVersionsToKeep = 7
)

type KeyValueItem struct {
	Value            string
	Version          int
	TTL              int  // Time-to-Live in seconds (currently unused for expiration logic)
	Deleted          int  // True if the item has been marked for deletion
	Changed          bool // True if the item has been modified since last persistence/sync
	LastModifiedTime int64
}

type Store struct {
	mu sync.RWMutex

	db map[string]map[string]*KeyValueItem
}

func NewStore() *Store {
	return &Store{
		db: make(map[string]map[string]*KeyValueItem),
	}
}

func (store *Store) Put(namespace, key, value string, ttl int) {
	store.mu.Lock()
	defer store.mu.Unlock()

	value = strings.Trim(value, " ")

	ns, ok := store.db[namespace]
	if !ok {
		ns = make(map[string]*KeyValueItem)
		store.db[namespace] = ns
	}

	item, ok := ns[key]
	if !ok || item.Deleted == 1 {
		// New item, or reviving a deleted item.
		ns[key] = &KeyValueItem{
			Value:   value,
			Version: 1,
			Changed: true,
			TTL:     ttl,
			Deleted: 0,
		}
		return
	}

	item.Changed = false

	if item.TTL != ttl {
		item.TTL = ttl
		item.Changed = true
	}

	if item.Value != value {
		item.Value = value
		item.Changed = true
	}

	if item.Deleted == 1 {
		item.Changed = true
	}

	if item.Changed {
		item.Version++
	}
}

func (store *Store) Delete(namespace, key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	ns, ok := store.db[namespace]
	if !ok {
		return fmt.Errorf("namespace not found: %s", namespace)
	}

	item, ok := ns[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	item.Deleted = 1
	item.Changed = true
	item.Version++

	return nil
}

func (store *Store) Get(namespace, key string) (KeyValueItem, error) {
	store.mu.RLock() // Use RLock for read-only access
	defer store.mu.RUnlock()

	ns, ok := store.db[namespace]
	if !ok {
		return KeyValueItem{}, ErrValueNotFound
	}

	item, ok := ns[key]
	if !ok || item.Deleted == 1 {
		return KeyValueItem{}, ErrValueNotFound
	}

	return KeyValueItem{
		Value:            item.Value,
		Version:          item.Version,
		TTL:              item.TTL,
		Deleted:          item.Deleted,
		LastModifiedTime: item.LastModifiedTime,
		Changed:          item.Changed,
	}, nil
}

func (store *Store) GetKeys(namespace string) ([]string, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	ns, ok := store.db[namespace]
	if !ok {
		return []string{}, nil
	}

	keys := make([]string, 0, len(ns))
	for key, item := range ns {
		// Only return keys that are not marked as deleted.
		if item.Deleted != 1 {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func (store *Store) GetNamespaces() []string {
	store.mu.RLock()
	defer store.mu.RUnlock()

	namespaces := make([]string, 0, len(store.db))
	for ns := range store.db {
		namespaces = append(namespaces, ns)
	}
	return namespaces
}

func (store *Store) GC(tx *sql.Tx, maxDays int) error {
	if maxDays < 0 {
		maxDays = 0
	}

	_, err := tx.Exec(
		`
		DELETE FROM key_value WHERE EXISTS (
				SELECT 1 FROM (
					SELECT key, namespace, MAX(version) AS latest_version, deleted, created_date FROM key_value GROUP BY key, namespace
				) kv2 WHERE key_value.key = kv2.key AND key_value.namespace = kv2.namespace AND (key_value.version < kv2.latest_version - ? OR kv2.deleted = 1)
						AND kv2.created_date < strftime('%s','now') - ?*24*60*60
		)`, MaxVersionsToKeep, maxDays)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}

func NewKeyValueStore(tx *sql.Tx) (*Store, error) {
	store := NewStore()

	rows, err := tx.Query("SELECT namespace, key, value, max(version), ttl, deleted, created_date FROM key_value GROUP BY key, namespace")
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var namespace, key, value string
		var version, ttl, deleted int
		var lastModifiedDate int64
		err = rows.Scan(&namespace, &key, &value, &version, &ttl, &deleted, &lastModifiedDate)
		if err != nil {
			return nil, bucket.DatabaseError(err)
		}

		ns, ok := store.db[namespace]
		if !ok {
			ns = make(map[string]*KeyValueItem)
			store.db[namespace] = ns
		}

		ns[key] = &KeyValueItem{
			Value:            value,
			Version:          version,
			TTL:              ttl,
			Changed:          false,
			Deleted:          deleted,
			LastModifiedTime: lastModifiedDate,
		}
	}

	return store, nil
}

func SaveKeyValueStore(tx *sql.Tx, store *Store) error {
	epoch := time.Now().Unix()

	store.mu.RLock()
	defer store.mu.RUnlock()

	for namespace, nsMap := range store.db {
		for key, item := range nsMap {
			if !item.Changed {
				continue
			}
			_, err := tx.Exec(
				`INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				key, item.Value, namespace, item.Version, item.TTL, epoch, item.Deleted,
			)
			if err != nil {
				return bucket.DatabaseError(err)
			}
			item.Changed = false
		}
	}

	return nil
}

var store *Store

func Initialize(tx *sql.Tx) error {
	s, err := NewKeyValueStore(tx)
	if err != nil {
		return err
	}
	store = s
	return nil
}

func GetKVStore() *Store {
	return store
}
