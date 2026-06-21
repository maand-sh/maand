// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"fmt"
	"strings"

	"maand/bucket"
)

// ManifestHealthCheck is the manifest.json health_check section.
type ManifestHealthCheck struct {
	Checks         []HealthCheckProbe `json:"checks"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	Wait           *HealthCheckWait   `json:"wait"`
}

// HealthCheckProbe is one built-in probe (tcp, http, or ssh).
type HealthCheckProbe struct {
	Type         string `json:"type"`
	Port         string `json:"port,omitempty"`
	Command      string `json:"command,omitempty"`
	Path         string `json:"path,omitempty"`
	ExpectStatus int    `json:"expect_status,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
}

// HealthCheckWait overrides retry behavior for this job's health checks.
type HealthCheckWait struct {
	Attempts        int `json:"attempts"`
	IntervalSeconds int `json:"interval_seconds"`
}

func hasManifestHealthChecks(manifest Manifest) bool {
	return manifest.HealthCheck != nil && len(manifest.HealthCheck.Checks) > 0
}

// ValidateHealthCheck ensures manifest probes reference declared ports and known types.
// Jobs may also define health_check commands; those run after manifest probes at check time.
func ValidateHealthCheck(jobName string, manifest Manifest) error {
	if !hasManifestHealthChecks(manifest) {
		return nil
	}

	declaredPorts := manifest.Resources.Ports.Names()
	declared := make(map[string]struct{}, len(declaredPorts))
	for _, name := range declaredPorts {
		declared[name] = struct{}{}
	}

	for idx, probe := range manifest.HealthCheck.Checks {
		probeType := strings.ToLower(strings.TrimSpace(probe.Type))
		switch probeType {
		case "tcp", "http", "ssh":
		default:
			return fmt.Errorf("%w: job %s health_check.checks[%d] type %q (want tcp, http, or ssh)",
				bucket.ErrInvalidManifest, jobName, idx, probe.Type)
		}

		switch probeType {
		case "ssh":
			if strings.TrimSpace(probe.Command) == "" {
				return fmt.Errorf("%w: job %s health_check.checks[%d] ssh probe requires command",
					bucket.ErrInvalidManifest, jobName, idx)
			}
		default:
			portName := strings.TrimSpace(probe.Port)
			if portName == "" {
				return fmt.Errorf("%w: job %s health_check.checks[%d] missing port",
					bucket.ErrInvalidManifest, jobName, idx)
			}
			if _, ok := declared[portName]; !ok {
				return fmt.Errorf("%w: job %s health_check.checks[%d] port %q not in resources.ports",
					bucket.ErrInvalidManifest, jobName, idx, portName)
			}
			if probeType == "http" {
				path := probe.Path
				if path == "" {
					path = "/"
				}
				if !strings.HasPrefix(path, "/") {
					return fmt.Errorf("%w: job %s health_check.checks[%d] path must start with /",
						bucket.ErrInvalidManifest, jobName, idx)
				}
			}
		}
	}
	return nil
}
