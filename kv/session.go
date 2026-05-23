// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import "database/sql"

var sessionStore *Store

// Initialize loads the KV store from tx into the process-wide session.
func Initialize(tx *sql.Tx) error {
	store, err := LoadFromTransaction(tx)
	if err != nil {
		return err
	}
	sessionStore = store
	return nil
}

// GetStore returns the session store (nil if Initialize was not called).
func GetStore() *Store {
	return sessionStore
}

// RequireStore returns the session store or ErrStoreNotInitialized.
func RequireStore() (*Store, error) {
	if sessionStore == nil {
		return nil, ErrStoreNotInitialized
	}
	return sessionStore, nil
}

// GetKVStore is deprecated; use GetStore.
func GetKVStore() *Store {
	return GetStore()
}
