package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
	"github.com/ShouryaTyagi042/Velora/server/internal/memstore"
)

// clock is a settable time for expiry tests.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newService(t *testing.T) (*auth.Service, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	s := auth.New(memstore.NewUsers(), auth.Options{Cost: bcrypt.MinCost, Now: c.now})
	if _, err := s.CreateUser(context.Background(), "shourya", "correct horse"); err != nil {
		t.Fatal(err)
	}
	return s, c
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)

	token, u, err := s.Login(ctx, "shourya", "correct horse")
	if err != nil || u.Username != "shourya" || len(token) != 43 {
		t.Fatalf("Login = %q, %+v, %v; want a 43-character token", token, u, err)
	}
	got, err := s.Authenticate(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Errorf("Authenticate = %+v, %v", got, err)
	}

	for _, tt := range []struct{ name, user, pass string }{
		{"wrong password", "shourya", "wrong horse"},
		{"unknown user", "nobody", "correct horse"},
		{"empty password", "shourya", ""},
		{"too long", "shourya", strings.Repeat("a", 73)},
	} {
		if _, _, err := s.Login(ctx, tt.user, tt.pass); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("%s: Login = %v, want ErrInvalidCredentials", tt.name, err)
		}
	}
}

func TestTokensAreRandom(t *testing.T) {
	s, _ := newService(t)
	a, _, _ := s.Login(context.Background(), "shourya", "correct horse")
	b, _, _ := s.Login(context.Background(), "shourya", "correct horse")
	if a == b {
		t.Error("two logins returned the same token")
	}
}

func TestAuthenticateRejects(t *testing.T) {
	s, _ := newService(t)
	for _, tok := range []string{"", "not-a-token", strings.Repeat("A", 43)} {
		if _, err := s.Authenticate(context.Background(), tok); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("Authenticate(%q) = %v, want ErrUnauthenticated", tok, err)
		}
	}
}

func TestSessionExpiresAndSlides(t *testing.T) {
	ctx := context.Background()
	s, c := newService(t)
	token, _, _ := s.Login(ctx, "shourya", "correct horse")

	// Used on day 29: still valid, and pushed to day 29 + 30.
	c.t = c.t.Add(29 * 24 * time.Hour)
	if _, err := s.Authenticate(ctx, token); err != nil {
		t.Fatalf("day 29: %v", err)
	}
	// Day 58: past the original expiry, inside the extended one.
	c.t = c.t.Add(29 * 24 * time.Hour)
	if _, err := s.Authenticate(ctx, token); err != nil {
		t.Fatalf("day 58 after use on day 29: %v", err)
	}
	// 31 idle days later: expired, and gone.
	c.t = c.t.Add(31 * 24 * time.Hour)
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("after 31 idle days: %v, want ErrUnauthenticated", err)
	}
}

func TestLogout(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	token, _, _ := s.Login(ctx, "shourya", "correct horse")
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("after logout: %v, want ErrUnauthenticated", err)
	}
	if err := s.Logout(ctx, token); err != nil {
		t.Errorf("second logout = %v, want nil", err)
	}
}

func TestCreateUser(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	if _, err := s.CreateUser(ctx, "shourya", "another password"); !errors.Is(err, auth.ErrUserExists) {
		t.Errorf("duplicate: %v, want ErrUserExists", err)
	}
	for _, tt := range []struct{ name, user, pass string }{
		{"blank name", "   ", "long enough"},
		{"short password", "ana", "short"},
		{"73-byte password", "ana", strings.Repeat("a", 73)},
		{"65-rune name", strings.Repeat("é", 65), "long enough"},
	} {
		if _, err := s.CreateUser(ctx, tt.user, tt.pass); !errors.Is(err, auth.ErrInvalidInput) {
			t.Errorf("%s: %v, want ErrInvalidInput", tt.name, err)
		}
	}
	u, err := s.CreateUser(ctx, "  ana  ", "long enough")
	if err != nil || u.Username != "ana" {
		t.Errorf("trimmed name: %+v, %v", u, err)
	}
}
