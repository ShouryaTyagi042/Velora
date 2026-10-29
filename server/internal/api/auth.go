package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/ShouryaTyagi042/Velora/server/internal/auth"
)

// Authenticator is what the API needs from auth. Declared here, by its consumer.
type Authenticator interface {
	Login(ctx context.Context, username, password string) (token string, u auth.User, err error)
	Logout(ctx context.Context, token string) error
	Authenticate(ctx context.Context, token string) (auth.User, error)
}

const userKey ctxKey = 1

// requireUser answers "who is this?" for every route inside it: a valid Bearer token puts the
// user in the request context, anything else is a 401 before the handler runs. Whether that
// user may do what they ask is the handler's question (authorization), not this one's.
// It sits inside Recover, so a panic during the lookup is still a 500, not a dropped connection.
func requireUser(a Authenticator, next http.Handler) http.Handler {
	return HandlerE(func(w http.ResponseWriter, r *http.Request) error {
		token, ok := bearerToken(r)
		if !ok {
			return auth.ErrUnauthenticated
		}
		u, err := a.Authenticate(r.Context(), token)
		if err != nil {
			return err // ErrUnauthenticated is a 401; a database failure stays a 500
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
		return nil
	})
}

// bearerToken reads "Authorization: Bearer <token>". The scheme is case-insensitive (RFC 7235).
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// currentUser is the signed-in user, put there by requireUser. A handler that calls it outside
// requireUser is a wiring bug, so it panics (Recover makes that a 500) rather than act as nobody.
func currentUser(r *http.Request) auth.User {
	u, ok := r.Context().Value(userKey).(auth.User)
	if !ok {
		panic("api: " + r.URL.Path + " is not behind requireUser")
	}
	return u
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string    `json:"token"`
	User  auth.User `json:"user"`
}

// login handles POST /api/auth/login: the one endpoint worth brute-forcing, so every attempt
// counts against a per-IP limit before the body is even read.
func login(a Authenticator, limit *ipLimiter) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		if !limit.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "60")
			return tooManyRequestsError("too many sign-in attempts; wait a minute and try again")
		}
		req, err := decode[loginRequest](w, r)
		if err != nil {
			return err
		}
		if strings.TrimSpace(req.Username) == "" || req.Password == "" {
			return validationError("username and password are required")
		}
		token, u, err := a.Login(r.Context(), strings.TrimSpace(req.Username), req.Password)
		if err != nil {
			return err // ErrInvalidCredentials is a 401
		}
		w.Header().Set("Cache-Control", "no-store") // a response holding a credential
		respondJSON(w, http.StatusOK, loginResponse{Token: token, User: u})
		return nil
	}
}

// logout handles POST /api/auth/logout: this device's session is deleted, at once.
func logout(a Authenticator) HandlerE {
	return func(w http.ResponseWriter, r *http.Request) error {
		token, _ := bearerToken(r) // requireUser already checked it
		if err := a.Logout(r.Context(), token); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// me handles GET /api/auth/me: who this token belongs to. The app calls it on launch to learn
// whether its stored token still works.
func me(w http.ResponseWriter, r *http.Request) error {
	respondJSON(w, http.StatusOK, currentUser(r))
	return nil
}

// clientIP is the connection's address. Behind a reverse proxy (phase 16) that is the proxy's,
// and the limiter would need X-Forwarded-For from a proxy it trusts; never from anyone else,
// since a client can send any header it likes.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipLimiter is a token bucket per client address: burst attempts at once, then one per every.
type ipLimiter struct {
	every time.Duration
	burst int

	mu   sync.Mutex
	byIP map[string]*ipBucket
}

type ipBucket struct {
	lim  *rate.Limiter
	seen time.Time
}

// newLoginLimiter allows 5 attempts a minute per address: plenty for a typo, nothing for a
// password list.
func newLoginLimiter() *ipLimiter {
	return &ipLimiter{every: 12 * time.Second, burst: 5, byIP: map[string]*ipBucket{}}
}

func (l *ipLimiter) allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	// A bucket untouched for longer than it takes to refill is the same as a new one, so drop
	// those once the map grows: one bucket per address ever seen would grow without bound.
	if len(l.byIP) > 1024 {
		full := l.every * time.Duration(l.burst)
		for k, b := range l.byIP {
			if now.Sub(b.seen) > full {
				delete(l.byIP, k)
			}
		}
	}
	b, ok := l.byIP[ip]
	if !ok {
		b = &ipBucket{lim: rate.NewLimiter(rate.Every(l.every), l.burst)}
		l.byIP[ip] = b
	}
	b.seen = now
	return b.lim.AllowN(now, 1)
}
