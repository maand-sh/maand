// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"fmt"
	"strconv"
	"strings"

	"maand/bucket"
)

// VersionConstraint holds optional min/max bounds from demands.config.
type VersionConstraint struct {
	Min *Version
	Max *Version
}

// ParseVersionConstraint reads min_version and max_version from demand config.
func ParseVersionConstraint(config map[string]interface{}) (VersionConstraint, error) {
	var constraint VersionConstraint
	if len(config) == 0 {
		return constraint, nil
	}

	if raw, ok := config["min_version"]; ok {
		v, err := parseConfigVersion(raw)
		if err != nil {
			return constraint, fmt.Errorf("demands.config.min_version: %w", err)
		}
		constraint.Min = &v
	}
	if raw, ok := config["max_version"]; ok {
		v, err := parseConfigVersion(raw)
		if err != nil {
			return constraint, fmt.Errorf("demands.config.max_version: %w", err)
		}
		constraint.Max = &v
	}

	if constraint.Min != nil && constraint.Max != nil && constraint.Min.Compare(*constraint.Max) > 0 {
		return constraint, fmt.Errorf("%w: min_version %s > max_version %s",
			bucket.ErrInvalidJobCommandConfiguration, constraint.Min, constraint.Max)
	}
	return constraint, nil
}

func parseConfigVersion(raw interface{}) (Version, error) {
	switch value := raw.(type) {
	case string:
		return ParseVersion(value)
	case float64:
		if value != float64(int64(value)) {
			return Version{}, fmt.Errorf("%w: non-integer number %v", bucket.ErrInvalidJobVersion, value)
		}
		return ParseVersion(strconv.FormatInt(int64(value), 10))
	case int:
		return ParseVersion(strconv.Itoa(value))
	case int64:
		return ParseVersion(strconv.FormatInt(value, 10))
	default:
		return Version{}, fmt.Errorf("%w: unsupported type %T", bucket.ErrInvalidJobVersion, raw)
	}
}

// ValidateDemandReference checks demand job/command pairing.
func ValidateDemandReference(jobName, commandName string, demand JobCommand) error {
	demandJob := strings.TrimSpace(demand.Demands.Job)
	demandCommand := strings.TrimSpace(demand.Demands.Command)

	switch {
	case demandJob == "" && demandCommand == "":
		return nil
	case demandJob == "" || demandCommand == "":
		return fmt.Errorf("%w: job %s command %s demands.job and demands.command must both be set",
			bucket.ErrInvalidJobCommandDemand, jobName, commandName)
	case demandJob == jobName:
		return fmt.Errorf("%w: job %s, job_command %s invalid configuration, self referencing",
			bucket.ErrInvalidJobCommandConfiguration, jobName, commandName)
	}
	return nil
}

// SatisfiesConstraint checks whether upstreamVersion meets min/max bounds.
func (c VersionConstraint) Satisfies(upstream Version) error {
	if c.Min != nil && upstream.Compare(*c.Min) < 0 {
		return fmt.Errorf("%w: upstream version %s below min_version %s",
			bucket.ErrJobCommandDemandVersionMismatch, upstream, c.Min)
	}
	if c.Max != nil && upstream.Compare(*c.Max) > 0 {
		return fmt.Errorf("%w: upstream version %s above max_version %s",
			bucket.ErrJobCommandDemandVersionMismatch, upstream, c.Max)
	}
	return nil
}

func (v Version) GoString() string {
	return v.String()
}