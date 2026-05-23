// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package tests provides tests for maand
package tests

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	_ "github.com/mattn/go-sqlite3"
)

func withDatabase(fn func(*sql.DB) error) error {
	db, err := data.OpenDatabase(false)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()
	return fn(db)
}

// MustQueryRow runs a query and scans one row; fails the test on error.
func MustQueryRow(t *testing.T, query string, dest ...any) {
	t.Helper()
	err := withDatabase(func(db *sql.DB) error {
		return db.QueryRow(query).Scan(dest...)
	})
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
}

// MustQueryCount returns the integer from a COUNT query.
func MustQueryCount(t *testing.T, query string, args ...any) int {
	t.Helper()
	var count int
	err := withDatabase(func(db *sql.DB) error {
		return db.QueryRow(query, args...).Scan(&count)
	})
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return count
}

// ScanQueryRows runs query and passes open rows to scanFn (rows are closed before return).
func ScanQueryRows(t *testing.T, query string, scanFn func(*sql.Rows) error, args ...any) {
	t.Helper()
	err := withDatabase(func(db *sql.DB) error {
		rows, err := db.Query(query, args...)
		if err != nil {
			return err
		}
		defer func() {
			_ = rows.Close()
		}()
		return scanFn(rows)
	})
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
}

// GetRowCount returns the result of a COUNT query. Prefer MustQueryCount in new tests.
func GetRowCount(query string) int {
	var count int
	err := withDatabase(func(db *sql.DB) error {
		return db.QueryRow(query).Scan(&count)
	})
	if err != nil {
		panic(err)
	}
	return count
}

// GetRowValues scans one row. Prefer MustQueryRow in new tests.
func GetRowValues(query string, dest ...any) {
	err := withDatabase(func(db *sql.DB) error {
		return db.QueryRow(query).Scan(dest...)
	})
	if err != nil {
		panic(err)
	}
}

// GetKey retrieves a value from the in-memory KV store after build.
func GetKey(ns, key string) (string, error) {
	store := kv.GetKVStore()
	if store == nil {
		return "", kv.ErrValueNotFound
	}
	item, err := store.Get(ns, key)
	if err != nil {
		return "", err
	}
	return item.Value, nil
}

func createJob(name string) {
	_ = os.MkdirAll(path.Join(bucket.WorkspaceLocation, "jobs", name), os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", name, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", name, "Makefile"), []byte(Makefile()), 0o644)
}

func Makefile() string {
	return `
.PHONY: start stop restart

dir:
	mkdir -p ./data
	mkdir -p ./logs
	mkdir -p ./bin

start: dir
	@echo $$(($(shell cat ./data/start 2>/dev/null || echo 0) + 1)) > ./data/start

stop:
	mkdir -p ./data
	@echo $$(($(shell cat ./data/stop 2>/dev/null || echo 0) + 1)) > ./data/stop

restart:
	mkdir -p ./data
	@echo $$(($(shell cat ./data/restart 2>/dev/null || echo 0) + 1)) > ./data/restart
`
}
