package sri

import (
	"strings"
	"unicode/utf8"
)

// Field lengths of the SRI schemas: names, addresses, descriptions, reasons
// and additional fields take up to 300 characters; item codes up to 25.
const (
	maxTextLen = 300
	maxCodeLen = 25
)

// normalizeText trims s and collapses every run of whitespace, line breaks
// included (the schemas forbid them), into one space.
func normalizeText(s string) string { return strings.Join(strings.Fields(s), " ") }

// xmlText normalizes s and caps it at max characters (runes), trimming a
// space the cut may leave at the end.
func xmlText(s string, max int) string {
	s = normalizeText(s)
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max]))
	}
	return s
}

// textTooLong reports whether s, once normalized, exceeds max characters.
func textTooLong(s string, max int) bool { return utf8.RuneCountInString(normalizeText(s)) > max }
