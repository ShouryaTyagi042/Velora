// Package auth answers "who is this?": users with bcrypt-hashed passwords, and opaque session
// tokens stored as hashes (ADR 008). Whether that user may do something is the handlers' call.
//
// Only this package imports bcrypt: the rule for storing passwords lives in one place.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// User is someone who can sign in.
type User struct {
	ID       int64  `json:"-"` // internal; clients know a user by their session, not an id
	Username string `json:"username"`
}

// Session is one signed-in device, as stored: the token's hash, never the token.
type Session struct {
	TokenHash []byte
	UserID    int64
	ExpiresAt time.Time
}

// Store is what auth needs from storage. Declared here, by its consumer.
type Store interface {
	// CreateUser stores a new user. ErrUserExists if the name is taken.
	CreateUser(ctx context.Context, username, passwordHash string) (User, error)
	// UserByName returns the user and their password hash, or ErrNotFound.
	UserByName(ctx context.Context, username string) (User, string, error)
	CreateSession(ctx context.Context, s Session) error
	// Session returns the session with this token hash and its user, expired or not, or ErrNotFound.
	Session(ctx context.Context, tokenHash []byte) (Session, User, error)
	ExtendSession(ctx context.Context, tokenHash []byte, expiresAt time.Time) error
	// DeleteSession removes the session; deleting one that doesn't exist is not an error.
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

var (
	// ErrNotFound is what a Store returns for a missing user or session.
	ErrNotFound = errors.New("auth: not found")
	// ErrUserExists: CreateUser with a taken username.
	ErrUserExists = errors.New("auth: username taken")
	// ErrInvalidCredentials: a wrong password and an unknown username are the same error, so the
	// answer doesn't tell an attacker which usernames exist.
	ErrInvalidCredentials = errors.New("auth: invalid username or password")
	// ErrUnauthenticated: no session, an unknown token, or an expired one.
	ErrUnauthenticated = errors.New("auth: not signed in")
	// ErrInvalidInput wraps a bad username or password at user creation; the message is for people.
	ErrInvalidInput = errors.New("auth: invalid input")
)

const (
	// SessionTTL is how long a session lasts without use. Sliding: every use pushes it forward,
	// so a phone used weekly stays signed in and a lost one stops working within a month.
	SessionTTL = 30 * 24 * time.Hour
	// extendEvery bounds the sliding to one write per session per day instead of one per request.
	extendEvery = 24 * time.Hour

	maxUsernameRunes  = 64
	minPasswordLength = 8
)

// Options tune a Service. The zero value is the production setting.
type Options struct {
	Cost int              // bcrypt cost; 0 means DefaultCost. Tests use bcrypt.MinCost.
	Now  func() time.Time // the clock; nil means time.Now. Tests move it to expire sessions.
}

// Service signs users in and out and recognises their tokens.
type Service struct {
	store Store
	cost  int
	now   func() time.Time

	dummyOnce sync.Once
	dummy     string // a hash to compare against for unknown usernames
}

func New(store Store, opts Options) *Service {
	s := &Service{store: store, cost: opts.Cost, now: opts.Now}
	if s.cost == 0 {
		s.cost = DefaultCost
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// CreateUser adds a user. Only velora-admin calls it: there is no signup endpoint on a
// personal server.
func (s *Service) CreateUser(ctx context.Context, username, password string) (User, error) {
	username = strings.TrimSpace(username)
	switch n := utf8.RuneCountInString(username); {
	case n == 0:
		return User{}, fmt.Errorf("%w: username must not be empty", ErrInvalidInput)
	case n > maxUsernameRunes:
		return User{}, fmt.Errorf("%w: username must be at most %d characters", ErrInvalidInput, maxUsernameRunes)
	}
	switch n := len(password); {
	case utf8.RuneCountInString(password) < minPasswordLength:
		return User{}, fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	case n > maxPasswordBytes:
		return User{}, fmt.Errorf("%w: password must be at most %d bytes", ErrInvalidInput, maxPasswordBytes)
	}
	hash, err := hashPassword(password, s.cost)
	if err != nil {
		return User{}, err
	}
	return s.store.CreateUser(ctx, username, hash)
}

// Login checks the password and starts a session. The token is returned once, here, and never
// stored: only its hash is.
func (s *Service) Login(ctx context.Context, username, password string) (string, User, error) {
	u, hash, err := s.store.UserByName(ctx, username)
	switch {
	case errors.Is(err, ErrNotFound):
		// Still pay for a bcrypt comparison: answering an unknown name in a millisecond and a
		// wrong password in 250 would reveal which names exist.
		checkPassword(s.dummyHash(), password)
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, err
	}
	if len(password) > maxPasswordBytes {
		return "", User{}, ErrInvalidCredentials // bcrypt refuses these; no stored password is this long
	}
	ok, err := checkPassword(hash, password)
	if err != nil {
		return "", User{}, err
	}
	if !ok {
		return "", User{}, ErrInvalidCredentials
	}

	token := newToken()
	if err := s.store.CreateSession(ctx, Session{
		TokenHash: hashToken(token), UserID: u.ID, ExpiresAt: s.now().Add(SessionTTL),
	}); err != nil {
		return "", User{}, err
	}
	return token, u, nil
}

// Authenticate returns the user a token belongs to, or ErrUnauthenticated. A session within a
// day of its last extension is used as is; an older one is pushed forward to a full TTL.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	th := hashToken(token)
	sess, u, err := s.store.Session(ctx, th)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, err
	}

	now := s.now()
	if !now.Before(sess.ExpiresAt) {
		// Expired: forget it now rather than leave it for a cleanup job (phase 17).
		if err := s.store.DeleteSession(ctx, th); err != nil {
			return User{}, err
		}
		return User{}, ErrUnauthenticated
	}
	if fresh := now.Add(SessionTTL); fresh.Sub(sess.ExpiresAt) >= extendEvery {
		if err := s.store.ExtendSession(ctx, th, fresh); err != nil {
			return User{}, err
		}
	}
	return u, nil
}

// Logout ends the session for token at once: it's a row, so revoking it is a delete (the reason
// for sessions over JWT). Logging out twice is fine.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, hashToken(token))
}

func (s *Service) dummyHash() string {
	s.dummyOnce.Do(func() {
		s.dummy, _ = hashPassword("velora-no-such-user", s.cost)
	})
	return s.dummy
}
