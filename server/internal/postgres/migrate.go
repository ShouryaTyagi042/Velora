package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

// Migrate applies every NNNN_name.sql file in fsys that isn't recorded in schema_migrations,
// in order, each in its own transaction with its record, and returns the resulting version.
// Forward only: undoing a migration is a new migration (and a backup, phase 16).
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS) (int, error) {
	if _, err := db.ExecContext(ctx, `
		create table if not exists schema_migrations (
		  version    integer primary key,
		  applied_at timestamptz not null default now()
		)`); err != nil {
		return 0, err
	}

	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return 0, err
	}
	slices.Sort(files) // zero-padded numbers sort correctly as text

	applied := map[int]bool{}
	rows, err := db.QueryContext(ctx, `select version from schema_migrations`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return 0, err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	version := 0
	for _, name := range files {
		num, _, ok := strings.Cut(name, "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return 0, fmt.Errorf("migration %s: name must be NNNN_description.sql", name)
		}
		version = max(version, v)
		if applied[v] {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return 0, err
		}
		// Postgres DDL is transactional: a migration that fails halfway leaves nothing behind.
		err = inTx(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `insert into schema_migrations (version) values ($1)`, v)
			return err
		})
		if err != nil {
			return 0, fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return version, nil
}
