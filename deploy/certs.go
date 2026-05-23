// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"os"
	"path"

	"maand/bucket"
	"maand/kv"
)

func updateCerts(tx *sql.Tx, job, workerIP string) error {
	store := kv.GetKVStore()
	if store == nil {
		return bucket.UnexpectedError(fmt.Errorf("kv store not initialized"))
	}

	workerDirPath := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDirPath, "jobs", job)

	rows, err := tx.Query(
		"SELECT name FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
		job,
	)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	certsDir := path.Join(jobDir, "certs")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		return err
	}

	workerKVNamespace := fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP)

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return bucket.DatabaseError(err)
		}

		pubCert, err := store.Get(workerKVNamespace, fmt.Sprintf("certs/%s.crt", name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(path.Join(certsDir, name+".crt"), []byte(pubCert.Value), 0o644); err != nil {
			return err
		}

		priCert, err := store.Get(workerKVNamespace, fmt.Sprintf("certs/%s.key", name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(path.Join(certsDir, name+".key"), []byte(priCert.Value), 0o600); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return bucket.DatabaseError(err)
	}

	caCert, err := store.Get("maand/worker", "certs/ca.crt")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(certsDir, "ca.crt"), []byte(caCert.Value), 0o644)
}
