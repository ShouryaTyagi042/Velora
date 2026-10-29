package postgres

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := []struct{ dsn, want string }{
		{"postgres://velora:velora-dev@localhost:5432/velora?sslmode=disable",
			"postgres://velora:xxxxx@localhost:5432/velora?sslmode=disable"},
		{"postgres://velora@localhost/velora", "postgres://velora@localhost/velora"},
		{"postgres://localhost/velora?password=s3cret&sslmode=disable",
			"postgres://localhost/velora?password=xxxxx&sslmode=disable"},
		{"host=localhost user=velora password=s3cret dbname=velora",
			"host=localhost user=velora password=xxxxx dbname=velora"},
		{"host=localhost password = 'two words' dbname=velora",
			"host=localhost password = xxxxx dbname=velora"},
	}
	for _, tt := range tests {
		got := Redact(tt.dsn)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.dsn, got, tt.want)
		}
		if strings.Contains(got, "s3cret") || strings.Contains(got, "velora-dev") || strings.Contains(got, "two words") {
			t.Errorf("Redact(%q) leaks the password: %q", tt.dsn, got)
		}
	}
}
