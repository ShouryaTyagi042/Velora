package comics

import (
	"path"
	"strings"
)

// imageTypes is the page allowlist, each extension mapped to its MIME type explicitly
// (mime.TypeByExtension depends on the host's mime.types and may not know .webp).
var imageTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// IsPage reports whether name is a page by its name alone: an allowed image extension
// (case-insensitive), and not hidden. Hidden covers .DS_Store and macOS's ._001.jpg
// AppleDouble files, which are named like images but aren't.
func IsPage(name string) bool {
	_, ok := imageTypes[strings.ToLower(path.Ext(name))]
	return ok && !strings.HasPrefix(name, ".")
}

// ContentType returns a page's MIME type from its name.
func ContentType(name string) string {
	return imageTypes[strings.ToLower(path.Ext(name))]
}

// NaturalCompare orders strings the way a person reads numbers in them:
// "page-2" < "page-10" (plain string order puts "page-10" first).
// Digit runs compare by numeric value, without ever converting to an int, so a
// 25-digit run can't overflow. Full ties fall back to plain string order, so the
// result is total and deterministic ("01" vs "1").
func NaturalCompare(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			ai, bj := digitsEnd(a, i), digitsEnd(b, j)
			if c := compareNumbers(a[i:ai], b[j:bj]); c != 0 {
				return c
			}
			i, j = ai, bj
			continue
		}
		if a[i] != b[j] {
			if a[i] < b[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(a)-i < len(b)-j: // a ran out first: "page" < "page-1"
		return -1
	case len(a)-i > len(b)-j:
		return 1
	}
	return strings.Compare(a, b)
}

// compareNumbers compares two runs of ASCII digits by value.
func compareNumbers(x, y string) int {
	x, y = strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
	if len(x) != len(y) { // more significant digits = bigger number
		if len(x) < len(y) {
			return -1
		}
		return 1
	}
	return strings.Compare(x, y) // same length: digit-by-digit order is numeric order
}

func digitsEnd(s string, i int) int {
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }
