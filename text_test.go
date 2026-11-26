package sri

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTextLimitsAreTheXSDOnes(t *testing.T) {
	assert.Equal(t, 300, maxTextLen)
	assert.Equal(t, 25, maxCodeLen)
}

func TestXMLText(t *testing.T) {
	tests := []struct {
		name, in string
		max      int
		want     string
	}{
		{"trims", "  Libro  ", 300, "Libro"},
		{"collapses spaces, tabs and line breaks", "Av.\nAmazonas \t N1-1\r\n", 300, "Av. Amazonas N1-1"},
		{"caps by runes, not bytes", strings.Repeat("á", 301), 300, strings.Repeat("á", 300)},
		{"exactly at the cap is kept", strings.Repeat("n", 300), 300, strings.Repeat("n", 300)},
		{"a cut that ends in a space is trimmed", "abc def", 4, "abc"},
		{"code capped at 25", strings.Repeat("C", 30), 25, strings.Repeat("C", 25)},
		{"empty stays empty", " \n ", 300, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, xmlText(tt.in, tt.max))
		})
	}
}

func TestTextTooLong(t *testing.T) {
	assert.False(t, textTooLong(strings.Repeat("ñ", 25), 25))
	assert.True(t, textTooLong(strings.Repeat("ñ", 26), 25))
	assert.False(t, textTooLong("  "+strings.Repeat("a", 25)+"\n", 25), "counted after normalizing")
	assert.False(t, textTooLong("a \n\n b", 3), "collapsed whitespace counts once")
}
