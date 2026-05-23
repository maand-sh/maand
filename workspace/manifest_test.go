package workspace

import "testing"

func TestManifestDefaults(t *testing.T) {
	m := Manifest{}
	if m.MinMemory() != "0 mb" {
		t.Fatalf("MinMemory: %q", m.MinMemory())
	}
	if m.JobVersion() != "unknown" {
		t.Fatalf("JobVersion: %q", m.JobVersion())
	}
	if m.ParallelUpdateCount() != 1 {
		t.Fatalf("ParallelUpdateCount: %d", m.ParallelUpdateCount())
	}
}

func TestManifestListedCommands(t *testing.T) {
	m := Manifest{
		Commands: map[string]JobCommand{
			"run": {},
		},
	}
	commands := m.ListedCommands()
	if len(commands) != 1 || commands[0].Name != "run" {
		t.Fatalf("commands: %#v", commands)
	}
	if commands[0].Demands.Config == nil {
		t.Fatal("expected non-nil config map")
	}
}
