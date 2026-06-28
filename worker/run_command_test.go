package worker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"maand/bucket"
)

func TestEnsureCommandScriptRejectsMissing(t *testing.T) {
	err := ensureCommandScript(filepath.Join(t.TempDir(), "missing.sh"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureCommandScriptRejectsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.sh")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureCommandScript(path); err == nil {
		t.Fatal("expected error for empty script")
	}
}

func TestRemoteCommandErrorIsRunCommand(t *testing.T) {
	err := remoteError("10.0.0.1", errors.New("ssh failed"))
	if !errors.Is(err, bucket.ErrRunCommand) {
		t.Fatalf("got %v", err)
	}
}

func TestExecuteFileCommandRejectsEmptyWorker(t *testing.T) {
	err := ExecuteFileCommand(nil, "  ", bucket.CommandContext{}, t.TempDir()+"/x.sh", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
