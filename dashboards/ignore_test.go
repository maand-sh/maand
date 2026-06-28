package dashboards

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseIgnoreFile(t *testing.T) {
	patterns := ParseIgnoreFile("# partials\n\n_partial.html\npartials/**\n")
	assert.Equal(t, []string{"_partial.html", "partials/**"}, patterns)
}

func TestIgnoredFromIndex(t *testing.T) {
	patterns := ParseIgnoreFile("_partial.html\npartials/**\n*.fragment.html\n")

	assert.True(t, IgnoredFromIndex(".dashboardignore", patterns))
	assert.True(t, IgnoredFromIndex("_partial.html", patterns))
	assert.True(t, IgnoredFromIndex("partials/header.html", patterns))
	assert.True(t, IgnoredFromIndex("foo.fragment.html", patterns))
	assert.False(t, IgnoredFromIndex("workers.html", patterns))
	assert.False(t, IgnoredFromIndex("slo/latency.html", patterns))
}

func TestShouldListInIndex(t *testing.T) {
	patterns := ParseIgnoreFile("_partial.html\n")
	assert.False(t, ShouldListInIndex(".dashboardignore", patterns))
	assert.False(t, ShouldListInIndex(".hidden.html", patterns))
	assert.False(t, ShouldListInIndex("_partial.html", patterns))
	assert.True(t, ShouldListInIndex("workers.html", patterns))
}

func TestIgnoredFromIndex_workerDetail(t *testing.T) {
	patterns := ParseIgnoreFile("worker_detail.html\n")
	assert.True(t, IgnoredFromIndex("worker_detail.html", patterns))
	assert.False(t, ShouldListInIndex("worker_detail.html", patterns))
	assert.True(t, ShouldListInIndex("workers.html", patterns))
}

func TestIgnoredFromIndex_doubleStarSuffix(t *testing.T) {
	patterns := ParseIgnoreFile("**/worker_detail.html\n")
	assert.True(t, IgnoredFromIndex("worker_detail.html", patterns))
	assert.True(t, IgnoredFromIndex("nested/worker_detail.html", patterns))
	assert.False(t, IgnoredFromIndex("workers.html", patterns))
}
