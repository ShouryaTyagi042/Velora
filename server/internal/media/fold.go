package media

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Fold is how search compares text: lower case, accents removed, so "amelie" matches "Amélie".
// NFD splits "é" into "e" plus a combining accent (a nonspacing mark, category Mn), which is
// then dropped. Postgres does the same with lower(unaccent(…)) (velora_fold, migration 0008);
// the two can differ on rare characters, which only the in-memory test double would notice.
func Fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
