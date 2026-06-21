// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"fmt"
	"sort"

	"maand/bucket"
	"maand/data"
	"maand/workspace"
)

type portAllocator struct {
	range_      bucket.PortRange
	existing    data.JobPortAssignments
	usedNumbers map[int]struct{}
}

func newPortAllocator(existing data.JobPortAssignments, portRange bucket.PortRange) *portAllocator {
	if existing == nil {
		existing = make(data.JobPortAssignments)
	}
	used := make(map[int]struct{})
	for _, ports := range existing {
		for _, port := range ports {
			used[port] = struct{}{}
		}
	}
	return &portAllocator{
		range_:      portRange,
		existing:    existing,
		usedNumbers: used,
	}
}

func (a *portAllocator) claimPort(jobName, portName string, port int) error {
	if prev, ok := a.existing[jobName][portName]; ok && prev == port {
		a.usedNumbers[port] = struct{}{}
		return nil
	}

	if _, taken := a.usedNumbers[port]; taken {
		return fmt.Errorf("%w: port %d (job %s port %q)", bucket.ErrPortCollision, port, jobName, portName)
	}

	a.usedNumbers[port] = struct{}{}
	if a.existing[jobName] == nil {
		a.existing[jobName] = make(map[string]int)
	}
	a.existing[jobName][portName] = port
	return nil
}

func (a *portAllocator) releaseAssignedNumber(jobName, portName string) {
	prev, ok := a.existing[jobName][portName]
	if !ok {
		return
	}
	delete(a.usedNumbers, prev)
}

// assignProvisioned picks or reuses a port from the bucket pool for manifest entry {}.
func (a *portAllocator) assignProvisioned(jobName string, portName string) (int, error) {
	if err := workspace.ValidatePortKey(portName); err != nil {
		return 0, err
	}

	// Reuse the number already stored in job_ports when it lies in the bucket pool.
	// Numbers outside port_min–port_max (e.g. a former fixed port like 9500) are
	// released and reassigned from the pool when the manifest uses {}.
	if prev, ok := a.existing[jobName][portName]; ok {
		if a.range_.Contains(prev) {
			a.usedNumbers[prev] = struct{}{}
			return prev, nil
		}
		a.releaseAssignedNumber(jobName, portName)
	}

	for port := a.range_.Min; port <= a.range_.Max; port++ {
		if _, taken := a.usedNumbers[port]; taken {
			continue
		}
		if err := a.claimPort(jobName, portName, port); err != nil {
			return 0, err
		}
		return port, nil
	}

	return 0, fmt.Errorf("%w: no free ports in range %d-%d", bucket.ErrPortRangeExhausted, a.range_.Min, a.range_.Max)
}

func (a *portAllocator) assignFixed(jobName, portName string, port int) (int, error) {
	if err := workspace.ValidatePortKey(portName); err != nil {
		return 0, err
	}
	if prev, ok := a.existing[jobName][portName]; ok && prev != port {
		a.releaseAssignedNumber(jobName, portName)
	}
	if err := a.claimPort(jobName, portName, port); err != nil {
		return 0, err
	}
	return port, nil
}

func (a *portAllocator) resolve(jobName, portName string, binding workspace.ManifestPortBinding) (int, error) {
	if binding.Provisioned() {
		return a.assignProvisioned(jobName, portName)
	}
	return a.assignFixed(jobName, portName, *binding.Fixed)
}

func (a *portAllocator) releaseRemoved(jobName string, keepNames []string) {
	prev := a.existing[jobName]
	if len(prev) == 0 {
		return
	}
	keep := make(map[string]struct{}, len(keepNames))
	for _, name := range keepNames {
		keep[name] = struct{}{}
	}
	for name, port := range prev {
		if _, still := keep[name]; still {
			continue
		}
		delete(a.usedNumbers, port)
		delete(prev, name)
	}
}

func syncJobPorts(
	tx *sql.Tx,
	jobID, jobName string,
	ports workspace.ManifestPorts,
	allocator *portAllocator,
) error {
	portNames := ports.Names()
	allocator.releaseRemoved(jobName, portNames)

	_, err := tx.Exec("DELETE FROM job_ports WHERE job_id = ?", jobID)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	for _, portName := range portNames {
		port, err := allocator.resolve(jobName, portName, ports[portName])
		if err != nil {
			return err
		}

		_, err = tx.Exec(
			"INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)",
			jobID, portName, port,
		)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}

	return nil
}

func buildPortAllocator(tx *sql.Tx, workspaceJobNames []string, jobWorkspace *workspace.DefaultWorkspace) (*portAllocator, error) {
	needsPorts := false
	for _, jobName := range workspaceJobNames {
		manifest, err := jobWorkspace.GetJobManifest(jobName)
		if err != nil {
			return nil, err
		}
		if len(manifest.Resources.Ports) > 0 {
			needsPorts = true
			break
		}
	}

	portRange, err := bucket.LoadPortRange()
	if err != nil {
		return nil, err
	}
	if needsPorts {
		if err := portRange.Validate(); err != nil {
			return nil, err
		}
	}

	existing, err := data.GetAllJobPortAssignments(tx)
	if err != nil {
		return nil, err
	}

	return newPortAllocator(existing, portRange), nil
}

func sortedJobNames(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}
