package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
)

// AuthRepo implements auth.Store: users and their sessions.
type AuthRepo struct {
	db *sql.DB
}

func NewAuthRepo(db *sql.DB) *AuthRepo { return &AuthRepo{db: db} }

// CreateUser implements auth.Store. A taken name is the unique constraint's call, not a
// select-then-insert: two concurrent creates can't both pass a check, but only one can insert.
func (r *AuthRepo) CreateUser(ctx context.Context, username, passwordHash string) (auth.User, error) {
	u := auth.User{Username: username}
	err := r.db.QueryRowContext(ctx,
		`insert into users (username, password_hash) values ($1, $2) returning id`,
		username, passwordHash).Scan(&u.ID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return auth.User{}, auth.ErrUserExists
	}
	return u, err
}

// UserByName implements auth.Store.
func (r *AuthRepo) UserByName(ctx context.Context, username string) (auth.User, string, error) {
	u := auth.User{Username: username}
	var hash string
	err := r.db.QueryRowContext(ctx,
		`select id, password_hash from users where username = $1`, username).Scan(&u.ID, &hash)
	if isNoRows(err) {
		return auth.User{}, "", auth.ErrNotFound
	}
	return u, hash, err
}

// CreateSession implements auth.Store.
func (r *AuthRepo) CreateSession(ctx context.Context, s auth.Session) error {
	_, err := r.db.ExecContext(ctx,
		`insert into sessions (token_hash, user_id, expires_at) values ($1, $2, $3)`,
		s.TokenHash, s.UserID, s.ExpiresAt)
	return err
}

// Session implements auth.Store: one indexed lookup by the primary key, on every request.
func (r *AuthRepo) Session(ctx context.Context, tokenHash []byte) (auth.Session, auth.User, error) {
	s := auth.Session{TokenHash: tokenHash}
	var u auth.User
	err := r.db.QueryRowContext(ctx, `
		select s.user_id, s.expires_at, u.username
		from sessions s join users u on u.id = s.user_id
		where s.token_hash = $1`, tokenHash).Scan(&s.UserID, &s.ExpiresAt, &u.Username)
	if isNoRows(err) {
		return auth.Session{}, auth.User{}, auth.ErrNotFound
	}
	u.ID = s.UserID
	return s, u, err
}

// ExtendSession implements auth.Store.
func (r *AuthRepo) ExtendSession(ctx context.Context, tokenHash []byte, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`update sessions set expires_at = $2 where token_hash = $1`, tokenHash, expiresAt)
	return err
}

// DeleteSession implements auth.Store.
func (r *AuthRepo) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.db.ExecContext(ctx, `delete from sessions where token_hash = $1`, tokenHash)
	return err
}

// HasUsers reports whether anyone can sign in yet. Not part of auth.Store: only startup asks.
func (r *AuthRepo) HasUsers(ctx context.Context) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `select exists (select 1 from users)`).Scan(&ok)
	return ok, err
}
