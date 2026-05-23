// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcontrol

import (
	"regexp"
	"strings"
)

// Target is a runner.py / Makefile target (built-in lifecycle actions or custom names).
type Target string

const (
	TargetStart   Target = "start"
	TargetStop    Target = "stop"
	TargetRestart Target = "restart"
	TargetStatus  Target = "status"
)

var (
	builtinTargets = []Target{TargetStart, TargetStop, TargetRestart, TargetStatus}
	targetNameRe   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
)

func validTargets() []string {
	names := make([]string, len(builtinTargets))
	for i, target := range builtinTargets {
		names[i] = string(target)
	}
	return names
}

// ParseTarget validates and normalizes a runner target string.
// Built-in targets are case-insensitive; custom Makefile targets keep their spelling.
func ParseTarget(raw string) (Target, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", &InvalidTargetError{Target: raw}
	}

	lower := strings.ToLower(trimmed)
	for _, allowed := range builtinTargets {
		if lower == string(allowed) {
			return allowed, nil
		}
	}

	if !targetNameRe.MatchString(trimmed) {
		return "", &InvalidTargetError{Target: raw}
	}

	return Target(trimmed), nil
}
