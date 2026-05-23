// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"os"
	"path/filepath"
	"testing"

	"maand/bucket"
)

func TestResolvePythonExecutableUsesVenv(t *testing.T) {
	root := t.TempDir()
	prev := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = prev
		bucket.UpdatePath()
	})

	modules := WorkspaceJobModulesDir("api")
	requireMkdir(t, filepath.Join(modules, ".venv", "bin"))
	venvPython := filepath.Join(modules, ".venv", "bin", "python3")
	requireWrite(t, venvPython, "")

	got := ResolvePythonExecutable("api")
	want, err := filepath.Abs(venvPython)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ResolvePythonExecutable() = %q, want %q", got, want)
	}
}

func TestResolvePythonExecutableFallback(t *testing.T) {
	root := t.TempDir()
	prev := bucket.Location
	bucket.Location = root
	bucket.UpdatePath()
	t.Cleanup(func() {
		bucket.Location = prev
		bucket.UpdatePath()
	})

	if got := ResolvePythonExecutable("api"); got != "python3" {
		t.Fatalf("got %q, want python3", got)
	}
}

func requireMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}
