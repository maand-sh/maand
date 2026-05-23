// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"maand/bucket"
)

var manifestPortKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ManifestPorts lists named ports declared in a job manifest.
// Each value must be an empty object: "database_port": {}
type ManifestPorts map[string]struct{}

// UnmarshalJSON accepts only empty JSON objects as port declarations.
func (p *ManifestPorts) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*p = nil
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	ports := make(ManifestPorts, len(raw))
	for name, value := range raw {
		trimmed := strings.TrimSpace(string(value))
		if trimmed != "{}" {
			return fmt.Errorf("%w: port %q must be {}, not %s", bucket.ErrInvalidManifestPort, name, trimmed)
		}
		ports[name] = struct{}{}
	}
	*p = ports
	return nil
}

// Names returns sorted port keys from the manifest.
func (p ManifestPorts) Names() []string {
	if len(p) == 0 {
		return nil
	}
	names := make([]string, 0, len(p))
	for name := range p {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidatePortKey checks a manifest port name.
func ValidatePortKey(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: port name cannot be empty", bucket.ErrPortKeyFormat)
	}
	if !manifestPortKeyPattern.MatchString(name) {
		return fmt.Errorf("%w: port name %q", bucket.ErrPortKeyFormat, name)
	}
	return nil
}
