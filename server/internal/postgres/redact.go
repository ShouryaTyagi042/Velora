package postgres

import (
	"net/url"
	"regexp"
)

// keywordPassword matches the password in a keyword/value DSN: password=secret or password='a b'.
var keywordPassword = regexp.MustCompile(`(password\s*=\s*)('(?:[^'\\]|\\.)*'|\S+)`)

// Redact returns dsn safe to log: the password replaced, everything else kept, so a log line
// still says which host, port, user and database were used. It handles both DSN forms:
// postgres://user:pw@host/db and host=... user=... password=....
func Redact(dsn string) string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		if q := u.Query(); q.Has("password") { // a URL may carry it as a query parameter too
			q.Set("password", "xxxxx")
			u.RawQuery = q.Encode()
		}
		return u.Redacted()
	}
	return keywordPassword.ReplaceAllString(dsn, "${1}xxxxx")
}
