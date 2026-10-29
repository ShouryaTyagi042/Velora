// Package migrations holds Velora's schema as numbered SQL files, applied in order, each once.
package migrations

import "embed"

// FS is every migration file, compiled into the binary so the server never depends on the
// working directory to find them.
//
//go:embed *.sql
var FS embed.FS
