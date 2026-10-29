package memstore

import (
	"context"
	"sync"
	"time"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
)

// Users is an in-memory auth.Store: the test double for postgres.AuthRepo.
type Users struct {
	mu       sync.Mutex
	nextID   int64
	byName   map[string]userRow
	sessions map[string]auth.Session // by string(token hash)
}

type userRow struct {
	user auth.User
	hash string
}

func NewUsers() *Users {
	return &Users{byName: map[string]userRow{}, sessions: map[string]auth.Session{}}
}

func (s *Users) CreateUser(ctx context.Context, username, passwordHash string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byName[username]; ok {
		return auth.User{}, auth.ErrUserExists
	}
	s.nextID++
	u := auth.User{ID: s.nextID, Username: username}
	s.byName[username] = userRow{user: u, hash: passwordHash}
	return u, nil
}

func (s *Users) UserByName(ctx context.Context, username string) (auth.User, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.byName[username]
	if !ok {
		return auth.User{}, "", auth.ErrNotFound
	}
	return row.user, row.hash, nil
}

func (s *Users) CreateSession(ctx context.Context, sess auth.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[string(sess.TokenHash)] = sess
	return nil
}

func (s *Users) Session(ctx context.Context, tokenHash []byte) (auth.Session, auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[string(tokenHash)]
	if !ok {
		return auth.Session{}, auth.User{}, auth.ErrNotFound
	}
	for _, row := range s.byName {
		if row.user.ID == sess.UserID {
			return sess, row.user, nil
		}
	}
	return auth.Session{}, auth.User{}, auth.ErrNotFound
}

func (s *Users) ExtendSession(ctx context.Context, tokenHash []byte, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[string(tokenHash)]; ok {
		sess.ExpiresAt = expiresAt
		s.sessions[string(tokenHash)] = sess
	}
	return nil
}

func (s *Users) DeleteSession(ctx context.Context, tokenHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, string(tokenHash))
	return nil
}
