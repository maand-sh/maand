// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/require"
)

func TestJobPortsFilter(t *testing.T) {
	root := t.TempDir()
	orig := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = orig
		bucket.UpdatePath()
	})

	require.NoError(t, initialize.Execute())

	db, err := data.OpenDatabase(false)
	require.NoError(t, err)
	defer db.Close()

	// Seed some data
	_, err = db.Exec(`INSERT INTO job (job_id, name) VALUES ('job1', 'api')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO job (job_id, name) VALUES ('job2', 'web')`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO job_ports (job_id, name, port) VALUES ('job1', 'http', 8080)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO job_ports (job_id, name, port) VALUES ('job2', 'https', 8443)`)
	require.NoError(t, err)

	t.Run("no filter", func(t *testing.T) {
		err := JobPorts("")
		require.NoError(t, err)
	})

	t.Run("filter by one job", func(t *testing.T) {
		err := JobPorts("api")
		require.NoError(t, err)
	})

	t.Run("filter by multiple jobs", func(t *testing.T) {
		err := JobPorts("api,web")
		require.NoError(t, err)
	})

	t.Run("filter by invalid job", func(t *testing.T) {
		err := JobPorts("invalid")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid input, jobs [invalid]")
	})
}
