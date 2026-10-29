// Package postgres stores Velora's state in PostgreSQL through database/sql and the pgx driver.
// SQL is written by hand: no ORM, so every query is visible and EXPLAIN-able.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

// Open returns a connection pool for dsn and checks that the database answers.
// sql.Open alone connects to nothing; the Ping is what proves the DSN works.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// *sql.DB is a pool. Each Postgres connection is a server process, so the pool is bounded
	// well under the server's max_connections (100 by default); requests beyond it queue here.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return db, nil
}

// inTx runs fn in a transaction: committed if fn returns nil, rolled back otherwise.
func inTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // a no-op after Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
