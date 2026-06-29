// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workerfacts

import (
	"fmt"
	"strings"
)

func errConcurrencyTooLow() error {
	return fmt.Errorf("concurrency must be at least 1")
}

func errWorkersAndLabelsTogether() error {
	return fmt.Errorf("use either --workers or --labels, not both")
}

func errNoTargetWorkers() error {
	return fmt.Errorf("no workers matched the requested filters")
}

func errUnknownWorkers(hosts []string) error {
	return fmt.Errorf("unknown workers: %s", strings.Join(hosts, ", "))
}

func errProbeFailures(failures map[string]error) error {
	if len(failures) == 0 {
		return nil
	}

	lines := make([]string, 0, len(failures))
	for host, err := range failures {
		lines = append(lines, fmt.Sprintf("worker %s: %v", host, err))
	}
	return fmt.Errorf("worker facts probe failed:\n%s", strings.Join(lines, "\n"))
}
