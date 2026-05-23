// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"
	"fmt"

	"maand/bucket"
)

// BucketInitialized reports whether the bucket table has a bucket_id row.
func BucketInitialized(tx *sql.Tx) (bool, error) {
	var bucketID string
	err := tx.QueryRow(`SELECT bucket_id FROM bucket LIMIT 1`).Scan(&bucketID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, bucket.DatabaseError(err)
	}
	return bucketID != "", nil
}

// InsertBucketRecord creates the singleton bucket row for a new installation.
func InsertBucketRecord(tx *sql.Tx, bucketID string) error {
	_, err := tx.Exec(`INSERT INTO bucket (bucket_id, update_seq) VALUES (?, 0)`, bucketID)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func GetBucketID(tx *sql.Tx) (string, error) {
	var bucketID string
	err := tx.QueryRow(`SELECT bucket_id FROM bucket LIMIT 1`).Scan(&bucketID)
	if err != nil {
		return "", bucket.DatabaseError(err)
	}
	return bucketID, nil
}

func GetBucketUpdateSeq(tx *sql.Tx) (int, error) {
	var updateSeq int
	err := tx.QueryRow(`SELECT update_seq FROM bucket`).Scan(&updateSeq)
	if err != nil {
		return -1, bucket.DatabaseError(err)
	}
	return updateSeq, nil
}

func SetBucketUpdateSeq(tx *sql.Tx, updateSeq int) error {
	_, err := tx.Exec(`UPDATE bucket SET update_seq = ?`, updateSeq)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func GetMaxDeploymentSeq(tx *sql.Tx) (int, error) {
	var maxSeq int
	err := tx.QueryRow(`SELECT ifnull(max(deployment_seq), 0) FROM allocations`).Scan(&maxSeq)
	if err != nil {
		return -1, bucket.DatabaseError(err)
	}
	return maxSeq, nil
}

func AllowedKVNamespaces(job, workerIP string) []string {
	return []string{
		"maand",
		"vars/bucket",
		"maand/worker",
		fmt.Sprintf("maand/worker/%s", workerIP),
		fmt.Sprintf("maand/worker/%s/tags", workerIP),
		fmt.Sprintf("maand/job/%s", job),
		fmt.Sprintf("vars/bucket/job/%s", job),
		fmt.Sprintf("vars/job/%s", job),
		fmt.Sprintf("secrets/job/%s", job),
		fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP),
	}
}
