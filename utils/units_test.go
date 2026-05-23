package utils

import "testing"

func TestParseMemoryMB(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"512MB", 512},
		{"1 GB", 1024},
		{"2", 2},
		{"", 0},
	}
	for _, tc := range tests {
		got, err := ParseMemoryMB(tc.in)
		if err != nil {
			t.Fatalf("ParseMemoryMB(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseMemoryMB(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseCPUMHz(t *testing.T) {
	got, err := ParseCPUMHz("2.4 GHZ")
	if err != nil {
		t.Fatal(err)
	}
	if got != 2400 {
		t.Fatalf("got %v, want 2400", got)
	}
}

func TestParseMemoryMBInvalid(t *testing.T) {
	if _, err := ParseMemoryMB("not-a-size"); err == nil {
		t.Fatal("expected error")
	}
}
