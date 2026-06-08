// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
)

const (
	embeddedMaandPyModule = "maand.py"
	embeddedMaandTSModule = "maand.ts"
	certsSubdir           = "certs"
	healthCheckEvent      = "health_check"
)

func prepareWorkerWorkspaces(tx *sql.Tx, jobName string, workerIPs []string, event, commandName string) error {
	store := kv.GetKVStore()
	if store == nil {
		return bucket.UnexpectedError(fmt.Errorf("kv store not initialized"))
	}

	for _, workerIP := range workerIPs {
		var err error
		if event == healthCheckEvent {
			err = prepareHealthWorkerWorkspace(tx, jobName, commandName, workerIP, store)
		} else {
			err = prepareOneWorkerWorkspace(tx, jobName, workerIP, store)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareOneWorkerWorkspace(tx *sql.Tx, jobName, workerIP string, store *kv.Store) error {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		return err
	}

	jobRoot := path.Join(workerDir, "jobs")
	if err := data.CopyJobFiles(tx, jobName, jobRoot); err != nil {
		return err
	}

	moduleDir := path.Join(jobRoot, jobName, "_modules")
	return writeCommandSupportFiles(jobName, workerIP, moduleDir, store)
}

func prepareHealthWorkerWorkspace(tx *sql.Tx, jobName, commandName, workerIP string, store *kv.Store) error {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		return err
	}

	jobRoot := path.Join(workerDir, "jobs")
	if err := data.CopyJobCommandModule(tx, jobName, commandName, jobRoot); err != nil {
		return err
	}

	moduleDir := path.Join(jobRoot, jobName, "_modules")
	return writeCommandSupportFiles(jobName, workerIP, moduleDir, store)
}

func writeCommandSupportFiles(jobName, workerIP, moduleDir string, store *kv.Store) error {
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(moduleDir, embeddedMaandPyModule), MaandPy, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(moduleDir, embeddedMaandTSModule), MaandTS, 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(moduleDir, certsSubdir), 0o755); err != nil {
		return err
	}
	return syncWorkerCertificates(jobName, workerIP, moduleDir, store)
}

func syncWorkerCertificates(jobName, workerIP, moduleDir string, store *kv.Store) error {
	workerKVNamespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)

	keys, err := store.GetKeys(workerKVNamespace)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if !strings.HasPrefix(key, certsSubdir+"/") {
			continue
		}

		item, err := store.Get(workerKVNamespace, key)
		if err != nil {
			return err
		}

		destPath := path.Join(moduleDir, key)
		perm := os.FileMode(0o644)
		if strings.HasSuffix(key, ".key") {
			perm = 0o600
		}
		if err := os.WriteFile(destPath, []byte(item.Value), perm); err != nil {
			return err
		}
	}

	caPath := path.Join(moduleDir, certsSubdir, "ca.crt")
	if _, err := os.Stat(caPath); err == nil {
		return nil
	}

	caItem, err := store.Get("maand/worker", "certs/ca.crt")
	if err != nil {
		return err
	}

	return os.WriteFile(caPath, []byte(caItem.Value), 0o644)
}
