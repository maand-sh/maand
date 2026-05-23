package prereq

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceUsesBun(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "jobs", "api", "_modules")
	require.NoError(t, os.MkdirAll(jobs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jobs, "command_x.py"), []byte("print(1)"), 0o644))

	uses, err := workspaceUsesBunIn(filepath.Join(root, "jobs"))
	require.NoError(t, err)
	assert.False(t, uses)

	require.NoError(t, os.WriteFile(filepath.Join(jobs, "command_y.ts"), []byte("console.log(1)"), 0o644))
	uses, err = workspaceUsesBunIn(filepath.Join(root, "jobs"))
	require.NoError(t, err)
	assert.True(t, uses)
}

func workspaceUsesBunIn(jobsRoot string) (bool, error) {
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		uses, err := modulesDirUsesBun(filepath.Join(jobsRoot, entry.Name(), "_modules"))
		if err != nil {
			return false, err
		}
		if uses {
			return true, nil
		}
	}
	return false, nil
}
