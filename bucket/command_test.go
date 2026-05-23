package bucket

import (
	"strings"
	"testing"
)

func TestBuildCommandScriptExportsEnv(t *testing.T) {
	script := BuildCommandScript([]string{"echo ok"}, []string{"FOO=bar", "export BAZ=qux"})
	if !strings.Contains(script, "export FOO=bar") {
		t.Fatalf("missing export: %q", script)
	}
	if !strings.Contains(script, "export BAZ=qux") {
		t.Fatalf("missing export: %q", script)
	}
	if !strings.Contains(script, "set -e") {
		t.Fatalf("missing set -e: %q", script)
	}
}
