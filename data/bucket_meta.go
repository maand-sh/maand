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

// BuildJobKVNamespaces returns job KV namespaces owned by maand build (catalog sync).
func BuildJobKVNamespaces(job string) []string {
	return []string{
		fmt.Sprintf("maand/job/%s", job),
		fmt.Sprintf("vars/bucket/job/%s", job),
	}
}

// JobCommandKVNamespaces returns job KV namespaces written by deploy/job commands.
func JobCommandKVNamespaces(job string) []string {
	return []string{
		fmt.Sprintf("vars/job/%s", job),
		fmt.Sprintf("secrets/job/%s", job),
	}
}

// JobKVNamespaces returns all job-scoped KV namespaces purged by GC when a job has no active allocations.
func JobKVNamespaces(job string) []string {
	namespaces := BuildJobKVNamespaces(job)
	return append(namespaces, JobCommandKVNamespaces(job)...)
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

// UpstreamDemandKVNamespaces returns KV namespaces for jobs this job depends on via command demands.
func UpstreamDemandKVNamespaces(tx *sql.Tx, job string) ([]string, error) {
	rows, err := tx.Query(
		`SELECT DISTINCT demand_job FROM job_commands
		 WHERE job = ? AND ifnull(trim(demand_job), '') != ''`,
		job,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	namespaces := make([]string, 0)
	for rows.Next() {
		var upstreamJob string
		if err := rows.Scan(&upstreamJob); err != nil {
			return nil, bucket.DatabaseError(err)
		}
		namespaces = append(namespaces,
			fmt.Sprintf("maand/job/%s", upstreamJob),
			fmt.Sprintf("vars/job/%s", upstreamJob),
			fmt.Sprintf("secrets/job/%s", upstreamJob),
		)
	}
	if err := rowsErr(rows); err != nil {
		return nil, err
	}
	return namespaces, nil
}

// AllowedKVNamespacesWithUpstream includes read namespaces for upstream jobs referenced in demands.
func AllowedKVNamespacesWithUpstream(tx *sql.Tx, job, workerIP string) ([]string, error) {
	namespaces := AllowedKVNamespaces(job, workerIP)
	if tx == nil {
		return namespaces, nil
	}
	upstream, err := UpstreamDemandKVNamespaces(tx, job)
	if err != nil {
		return nil, err
	}
	return append(namespaces, upstream...), nil
}

// AccessibleKVNamespacesForJob returns the union of KV namespaces readable by job commands
// or templates on any allocation row for the job (including upstream demand jobs).
func AccessibleKVNamespacesForJob(tx *sql.Tx, job string) ([]string, error) {
	workerIPs, err := GetNonRemovedAllocations(tx, job)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	namespaces := make([]string, 0)
	add := func(items ...string) {
		for _, ns := range items {
			if _, ok := seen[ns]; ok {
				continue
			}
			seen[ns] = struct{}{}
			namespaces = append(namespaces, ns)
		}
	}

	for _, workerIP := range workerIPs {
		allowed, err := AllowedKVNamespacesWithUpstream(tx, job, workerIP)
		if err != nil {
			return nil, err
		}
		add(allowed...)
	}
	return namespaces, nil
}
