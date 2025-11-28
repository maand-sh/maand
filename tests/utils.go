// Package tests provides tests for maand
package tests

import (
	"database/sql"
	"io"
	"os"
	"path"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	_ "github.com/mattn/go-sqlite3"
)

func GetRow(query string) *sql.Row {
	db, err := data.GetDatabase(false)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()
	row := db.QueryRow(query)
	return row
}

func GetRows(query string) *sql.Rows {
	db, _ := data.GetDatabase(false)
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()
	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	return rows
}

func GetRowCount(query string) int {
	row := GetRow(query)
	var count int
	err := row.Scan(&count)
	if err != nil {
		panic(err)
	}
	return count
}

func GetRowValues(query string, args ...any) {
	db, _ := data.GetDatabase(false)
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()
	row := db.QueryRow(query, args...)
	err := row.Scan(args...)
	if err != nil {
		panic(err)
	}
}

func GetKey(ns, key string) (string, error) {
	db, _ := data.GetDatabase(false)
	defer func() {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}()

	tx, _ := db.Begin()
	defer func() {
		err := tx.Rollback()
		if err != nil {
			panic(err)
		}
	}()

	return kv.GetKVStore().Get(tx, ns, key)
}

func CopyFile(src, dst string) {
	sourceFile, err := os.Open(src)
	if err != nil {
		panic(err)
	}

	defer func() {
		err := sourceFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	destinationFile, err := os.Create(dst)
	if err != nil {
		panic(err)
	}

	defer func() {
		err := destinationFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		panic(err)
	}

	// Ensure the destination file is properly written
	err = destinationFile.Sync()
	if err != nil {
		panic(err)
	}
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

func createJob(name string) {
	_ = os.MkdirAll(path.Join(bucket.WorkspaceLocation, "jobs", name), os.ModePerm)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", name, "manifest.json"), []byte(`{"selectors":["worker"]}`), 0o644)
	_ = os.WriteFile(path.Join(bucket.WorkspaceLocation, "jobs", name, "Makefile"), []byte(Makefile()), 0o644)
}
