package utils

import (
	"sort"
	"testing"
)

func TestUniquePreservesOneOfEach(t *testing.T) {
	got := Unique([]string{"a", "b", "a", "c"})
	sort.Strings(got)
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
}

func TestIntersection(t *testing.T) {
	got := Intersection([]string{"a", "b"}, []string{"b", "c"})
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestDifference(t *testing.T) {
	got := Difference([]string{"a", "b"}, []string{"b"})
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %#v", got)
	}
}
