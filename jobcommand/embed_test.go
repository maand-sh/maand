package jobcommand

import (
	"strings"
	"testing"
)

func TestEmbeddedMaandSDKNonEmpty(t *testing.T) {
	if len(MaandPy) == 0 {
		t.Fatal("MaandPy is empty")
	}
	if len(MaandTS) == 0 {
		t.Fatal("MaandTS is empty")
	}
}

func TestEmbeddedMaandPythonAPI(t *testing.T) {
	py := string(MaandPy)
	for _, needle := range []string{
		"JOB_COMMAND_API_HOST",
		"acquire_semaphore",
		"release_semaphore",
		"/semaphore/acquire",
		"run_runner_target",
		"load_ssh",
	} {
		if !strings.Contains(py, needle) {
			t.Fatalf("maand.py missing %q", needle)
		}
	}
}

func TestEmbeddedMaandTypeScriptAPI(t *testing.T) {
	ts := string(MaandTS)
	for _, needle := range []string{
		"JOB_COMMAND_API_HOST",
		"acquireSemaphore",
		"releaseSemaphore",
		"putJobVariable",
		"listCommandDemands",
	} {
		if !strings.Contains(ts, needle) {
			t.Fatalf("maand.ts missing %q", needle)
		}
	}
}
