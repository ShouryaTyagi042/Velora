package scanner

import (
	"slices"
	"testing"
)

func TestNaturalCompareSortsPages(t *testing.T) {
	got := []string{"page-10.png", "page-2.png", "page-1.png", "page-9.png", "page-100.png"}
	slices.SortFunc(got, naturalCompare)
	want := []string{"page-1.png", "page-2.png", "page-9.png", "page-10.png", "page-100.png"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNaturalCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"page-2", "page-10", -1},
		{"page-10", "page-2", 1},
		{"001", "002", -1},
		{"002", "010", -1},
		{"2", "010", -1}, // value, not length: 2 < 10
		{"01", "1", -1},  // equal value: plain string order breaks the tie
		{"a", "a", 0},
		{"page", "page-1", -1}, // prefix sorts first
		{"cover.jpg", "page-1.jpg", -1},
		// 25-digit runs: compared as text, so nothing overflows
		{"page-9999999999999999999999999", "page-10000000000000000000000000", -1},
	}
	for _, tt := range tests {
		if got := naturalCompare(tt.a, tt.b); sign(got) != tt.want {
			t.Errorf("naturalCompare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
