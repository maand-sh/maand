// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import "database/sql"

// WorkerCatalog is the set of worker IPs currently in workers.json / the worker table.
type WorkerCatalog map[string]struct{}

// NewWorkerCatalog builds a catalog from worker IP addresses.
func NewWorkerCatalog(workerIPs []string) WorkerCatalog {
	catalog := make(WorkerCatalog, len(workerIPs))
	for _, workerIP := range workerIPs {
		catalog[workerIP] = struct{}{}
	}
	return catalog
}

// LoadWorkerCatalog reads current workers from the database.
func LoadWorkerCatalog(tx *sql.Tx) (WorkerCatalog, error) {
	workerIPs, err := GetWorkers(tx, nil)
	if err != nil {
		return nil, err
	}
	return NewWorkerCatalog(workerIPs), nil
}

// Contains reports whether workerIP is still in the catalog.
func (c WorkerCatalog) Contains(workerIP string) bool {
	if c == nil {
		return false
	}
	_, ok := c[workerIP]
	return ok
}
