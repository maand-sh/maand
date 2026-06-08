// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodePeerPorts(t *testing.T) {
	portMap := map[string]string{
		"cql_port":  "9042",
		"http_port": "8080",
	}
	got := encodePeerPorts([]string{"10.0.0.2"}, portMap)
	assert.Equal(t, "10.0.0.2:cql_port:9042,10.0.0.2:http_port:8080", got)
	assert.Empty(t, encodePeerPorts(nil, portMap))
	assert.Empty(t, encodePeerPorts([]string{"10.0.0.2"}, nil))
}
