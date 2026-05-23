package utils

import "testing"

func TestSliceRange(t *testing.T) {
	items := []string{"a", "b", "c"}
	got, err := SliceRange(items, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("unexpected slice: %#v", got)
	}
}

func TestNewBatchIterator(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	next := NewBatchIterator(items, 2)
	var batches [][]string
	for {
		batch, ok := next()
		if !ok {
			break
		}
		batches = append(batches, batch)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
}
