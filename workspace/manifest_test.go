package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestPlacementSelectors_jobNameOnlyWhenEmpty(t *testing.T) {
	assert.Equal(t, []string{"prometheus"}, PlacementSelectors("prometheus", Manifest{}))
}

func TestPlacementSelectors_usesManifestSelectorsWhenSet(t *testing.T) {
	m := Manifest{Selectors: []string{"worker", "prod"}}
	assert.Equal(t, []string{"worker", "prod"}, PlacementSelectors("api", m))
}

func TestPlacementSelectors_deduplicatesManifestSelectors(t *testing.T) {
	m := Manifest{Selectors: []string{"web", "web"}}
	assert.Equal(t, []string{"web"}, PlacementSelectors("web", m))
}
