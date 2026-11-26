package sri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidCedula(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid", "1700000001", true},
		{"valid province 30", "3000000004", true},
		{"valid province 24", "2400000002", true},
		{"check digit zero", "1700000050", true},
		{"wrong check digit", "1700000002", false},
		{"too short", "170000000", false},
		{"too long", "17000000011", false},
		{"letters", "17000000A1", false},
		{"province 00", "0000000000", false},
		{"province 25 with a correct check digit", "2500000001", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidCedula(tt.id), tt.id)
		})
	}
}

func TestValidRUC(t *testing.T) {
	tests := []struct {
		name string
		ruc  string
		want bool
	}{
		{"private company", "1790000000001", true},
		{"natural person", "1700000001001", true},
		{"public entity", "1760000000001", true},
		{"province 30", "3090000000001", true},
		// The check digit is deliberately not validated: the SRI issues RUCs that fail módulo 11.
		{"well formed but fails módulo 11", "1790000001001", true},
		{"12 digits", "179000000000", false},
		{"14 digits", "17900000000011", false},
		{"letters", "17900000000A1", false},
		{"province 00", "0090000000001", false},
		{"province 25", "2590000000001", false},
		{"does not end in 001", "1790000000002", false},
		{"final consumer", "9999999999999", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidRUC(tt.ruc), tt.ruc)
		})
	}
}

func TestValidForeignID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"AB123456", true},
		{"X-9988776", true},
		{"ABC", true},
		{"ABCDEFGHIJKLM", true},
		{"A1", false},
		{"ABCDEFGHIJKLMN", false},
		{"AB 12345", false},
		{"AB_12345", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.want, validForeignID(tt.id), tt.id)
		})
	}
}
