package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffFileManifests(t *testing.T) {
	previous := FileManifest{
		"Makefile":      "a",
		"config/app.toml": "b",
	}
	current := FileManifest{
		"Makefile":        "a2",
		"config/app.toml": "b",
		"rules/alerts.yaml": "c",
	}
	assert.Equal(t,
		[]string{"Makefile", "rules/alerts.yaml"},
		DiffFileManifests(previous, current),
	)
}

func TestFileManifestEncode(t *testing.T) {
	encoded, err := FileManifest{"b": "2", "a": "1"}.Encode()
	assert.NoError(t, err)
	assert.Equal(t, `{"a":"1","b":"2"}`, encoded)
}
