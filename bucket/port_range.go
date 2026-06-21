// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultPortMin = 30000
	defaultPortMax = 39999
)

// PortRange is the inclusive port pool declared in workspace/bucket.conf.
type PortRange struct {
	Min int
	Max int
}

// LoadPortRange reads port_min and port_max from workspace/bucket.conf.
// Missing keys use defaults (30000–39999).
func LoadPortRange() (PortRange, error) {
	r := PortRange{Min: defaultPortMin, Max: defaultPortMax}

	confPath := path.Join(WorkspaceLocation, "bucket.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			return r, r.Validate()
		}
		return PortRange{}, fmt.Errorf("%w: %w", ErrUnexpectedError, err)
	}

	var settings map[string]string
	if err := toml.Unmarshal(data, &settings); err != nil {
		return PortRange{}, fmt.Errorf("%w: %w", ErrInvalidBucketConf, err)
	}

	if raw, ok := settings["port_min"]; ok && strings.TrimSpace(raw) != "" {
		r.Min, err = strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return PortRange{}, fmt.Errorf("%w: port_min %q", ErrInvalidPortRange, raw)
		}
	}
	if raw, ok := settings["port_max"]; ok && strings.TrimSpace(raw) != "" {
		r.Max, err = strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return PortRange{}, fmt.Errorf("%w: port_max %q", ErrInvalidPortRange, raw)
		}
	}

	return r, r.Validate()
}

// Contains reports whether port is inside the inclusive maand assignment pool.
func (r PortRange) Contains(port int) bool {
	return port >= r.Min && port <= r.Max
}

// Validate checks the port pool bounds.
func (r PortRange) Validate() error {
	if r.Min < 1 || r.Max > 65535 || r.Min > r.Max {
		return fmt.Errorf("%w: port_min=%d port_max=%d", ErrInvalidPortRange, r.Min, r.Max)
	}
	return nil
}

// DefaultBucketConf returns the initial workspace/bucket.conf content for new buckets.
func DefaultBucketConf() string {
	return fmt.Sprintf("port_min = \"%d\"\nport_max = \"%d\"\n", defaultPortMin, defaultPortMax)
}
