// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package healthcheck

import (
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"maand/data"
	"maand/worker"
	"maand/workspace"
)

const defaultProbeTimeout = 5 * time.Second

func runManifestHealthChecks(tx *sql.Tx, job string, spec *workspace.ManifestHealthCheck) error {
	workers, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return err
	}
	if len(workers) == 0 {
		return nil
	}

	timeout := probeTimeout(spec)
	for _, workerIP := range workers {
		for idx, probe := range spec.Checks {
			if err := runProbe(tx, job, workerIP, idx, probe, timeout); err != nil {
				return err
			}
		}
	}
	return nil
}

func probeTimeout(spec *workspace.ManifestHealthCheck) time.Duration {
	if spec == nil || spec.TimeoutSeconds <= 0 {
		return defaultProbeTimeout
	}
	return time.Duration(spec.TimeoutSeconds) * time.Second
}

func waitConfig(spec *workspace.ManifestHealthCheck) (attempts int, interval time.Duration) {
	attempts = waitRetryAttempts
	interval = waitRetryInterval
	if spec != nil && spec.Wait != nil {
		if spec.Wait.Attempts > 0 {
			attempts = spec.Wait.Attempts
		}
		if spec.Wait.IntervalSeconds > 0 {
			interval = time.Duration(spec.Wait.IntervalSeconds) * time.Second
		}
	}
	return attempts, interval
}

func runProbe(tx *sql.Tx, job, workerIP string, idx int, probe workspace.HealthCheckProbe, timeout time.Duration) error {
	switch strings.ToLower(strings.TrimSpace(probe.Type)) {
	case "ssh":
		return probeSSH(workerIP, probe.Command, timeout)
	case "tcp", "http":
		portName := strings.TrimSpace(probe.Port)
		port, err := data.GetJobPortNumber(tx, job, portName)
		if err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(probe.Type), "tcp") {
			return probeTCP(workerIP, port, timeout)
		}
		return probeHTTP(workerIP, port, probe, timeout)
	default:
		return fmt.Errorf("health_check.checks[%d]: unsupported type %q", idx, probe.Type)
	}
}

func probeSSH(workerIP, command string, timeout time.Duration) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("ssh %s: empty command", workerIP)
	}
	if err := worker.RemoteShellCommand(workerIP, command, timeout); err != nil {
		return fmt.Errorf("ssh %s: %w", workerIP, err)
	}
	return nil
}

func probeTCP(host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("tcp %s: %w", addr, err)
	}
	_ = conn.Close()
	return nil
}

func probeHTTP(host string, port int, probe workspace.HealthCheckProbe, timeout time.Duration) error {
	scheme := strings.ToLower(strings.TrimSpace(probe.Scheme))
	if scheme == "" {
		scheme = "http"
	}
	path := probe.Path
	if path == "" {
		path = "/"
	}
	expectStatus := probe.ExpectStatus
	if expectStatus == 0 {
		expectStatus = http.StatusOK
	}

	url := fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("http %s: %w", url, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expectStatus {
		return fmt.Errorf("http %s: status %d want %d", url, resp.StatusCode, expectStatus)
	}
	return nil
}
