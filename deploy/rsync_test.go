// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/require"
)

func TestRsync_executesWithLocalStaging(t *testing.T) {
	env := setupDeployTestEnv(t)
	ClearTestHooks()
	t.Cleanup(ClearTestHooks)

	workerIP := "10.0.0.1"
	workerDir := bucket.GetTempWorkerPath(workerIP)
	require.NoError(t, os.MkdirAll(workerDir, 0o755))
	require.NoError(t, os.WriteFile(path.Join(workerDir, "worker.json"), []byte(`{}`), 0o644))

	filterPath := path.Join(bucket.TempLocation, "workers", workerIP+".rsync")
	require.NoError(t, os.MkdirAll(path.Dir(filterPath), 0o755))
	require.NoError(t, os.WriteFile(filterPath, []byte("- jobs/**/*.tpl\n"), 0o644))

	keyPath := path.Join(bucket.SecretLocation, "worker.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("dummy-key"), 0o600))

	rt, err := bucket.SetupRuntime(env.bucketID, bucket.NewRunContext("test", 0))
	require.NoError(t, err)
	defer func() { _ = rt.Stop() }()

	err = rsync(rt, env.bucketID, workerIP, []string{"app"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rsync failed")
	require.Contains(t, err.Error(), workerIP)
	_, statErr := os.Stat(workerDir)
	require.NoError(t, statErr)
	_, err = filepath.Abs(workerDir)
	require.NoError(t, err)
}
