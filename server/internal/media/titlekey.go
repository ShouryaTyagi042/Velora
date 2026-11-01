package media

import "strings"

// titleDigits is how wide TitleKey pads a number: wider than any int64.
const titleDigits = 20

// TitleKey is what titles sort by: natural order, so "Episode 2" comes before "Episode 10".
// It lower-cases ASCII letters and replaces every run of digits with its value zero-padded to 20
// places, so plain byte order on keys is natural order. It is the Go twin of the SQL function
// velora_title_key (migration 0010), byte for byte: the in-memory store sorts exactly like
// Postgres. Only ASCII letters are lowered, as Postgres's lower() does under collation "C".
func TitleKey(title string) string {
	var b strings.Builder
	b.Grow(len(title) + titleDigits)
	for i := 0; i < len(title); {
		c := title[i]
		if '0' <= c && c <= '9' {
			j := i
			for j < len(title) && '0' <= title[j] && title[j] <= '9' {
				j++
			}
			n := strings.TrimLeft(title[i:j], "0")
			for k := len(n); k < titleDigits; k++ {
				b.WriteByte('0')
			}
			b.WriteString(n)
			i = j
			continue
		}
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}
