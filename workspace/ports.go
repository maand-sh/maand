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

// ManifestPortBinding is one port entry in a job manifest.
// Fixed is nil when maand should assign from the bucket port pool (manifest value {}).
// Fixed is non-nil when the manifest sets an explicit port number.
type ManifestPortBinding struct {
	Fixed *int
}

// Provisioned reports whether maand assigns the port number at build time.
func (b ManifestPortBinding) Provisioned() bool {
	return b.Fixed == nil
}

// ManifestPorts lists named ports declared in a job manifest.
type ManifestPorts map[string]ManifestPortBinding

// UnmarshalJSON accepts {} (maand-provisioned) or a JSON integer (fixed port).
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
		if err := ValidatePortKey(name); err != nil {
			return err
		}

		trimmed := strings.TrimSpace(string(value))
		if trimmed == "{}" {
			ports[name] = ManifestPortBinding{}
			continue
		}

		var port int
		if err := json.Unmarshal(value, &port); err != nil {
			return fmt.Errorf("%w: port %q must be {} or an integer, not %s", bucket.ErrInvalidManifestPort, name, trimmed)
		}
		ports[name] = ManifestPortBinding{Fixed: &port}
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
