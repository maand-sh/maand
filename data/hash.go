// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"
)

func UpdateHash(tx *sql.Tx, namespace, key, hash string) error {
	var dbCurrentHash string
	row := tx.QueryRow("SELECT current_hash FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	err := row.Scan(&dbCurrentHash)

	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.Exec("INSERT INTO hash (namespace, key, current_hash) VALUES (?, ?, ?)", namespace, key, hash)
		if err != nil {
			return NewDatabaseError(err)
		}
		return nil
	}

	if err != nil {
		return NewDatabaseError(err)
	}

	_, err = tx.Exec("UPDATE hash SET current_hash = ? WHERE namespace = ? AND key = ?", hash, namespace, key)
	if err != nil {
		return NewDatabaseError(err)
	}
	return nil
}

func HashChanged(tx *sql.Tx, namespace, key string) (bool, error) {
	var dbCurrentHash, dbPreviousHash string
	row := tx.QueryRow("SELECT ifnull(current_hash, '') as current_hash, ifnull(previous_hash, '') as previous_hash FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	err := row.Scan(&dbCurrentHash, &dbPreviousHash)
	if errors.Is(err, sql.ErrNoRows) || dbCurrentHash != dbPreviousHash {
		return true, nil
	}
	if err != nil {
		return false, NewDatabaseError(err)
	}
	return false, nil
}

func PromoteHash(tx *sql.Tx, namespace, key string) error {
	_, err := tx.Exec("UPDATE hash SET previous_hash = current_hash WHERE namespace = ? AND key = ?", namespace, key)
	if err != nil {
		return NewDatabaseError(err)
	}
	return nil
}

func RemoveHash(tx *sql.Tx, namespace, key string) error {
	_, err := tx.Exec("DELETE FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	if err != nil {
		return NewDatabaseError(err)
	}
	return nil
}

func GetPreviousHash(tx *sql.Tx, namespace, key string) (string, error) {
	var previousHash string
	row := tx.QueryRow("SELECT ifnull(previous_hash, '') FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	err := row.Scan(&previousHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", NewDatabaseError(err)
	}
	return previousHash, nil
}
