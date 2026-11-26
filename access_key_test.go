package sri

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidAccessKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"valid invoice key", "1811202601179000000200110010010000000071234567818", true},
		{"valid key with check digit 0", "0102202601179000000200120010020000000050000004210", true},
		{"valid key with check digit 1", "0102202601179000000200120010020000000030000004211", true},
		{"wrong check digit", "1811202601179000000200110010010000000071234567816", false},
		{"48 digits", "181120260117900000020011001001000000007123456781", false},
		{"50 digits", "18112026011790000002001100100100000000712345678180", false},
		{"letters", "18112026011790000002001100100100000000712345678A8", false},
		{"empty", "", false},
		{"spaces", " 811202601179000000200110010010000000071234567818", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidAccessKey(tt.key))
		})
	}
}

func TestGenerateAccessKey(t *testing.T) {
	// 12345678 = 0x00BC614E: the 4 bytes crypto/rand.Int reads for a bound of 10^8.
	code12345678 := []byte{0x00, 0xBC, 0x61, 0x4E}
	code42 := []byte{0x00, 0x00, 0x00, 0x2A}

	tests := []struct {
		name          string
		date          time.Time
		doc           DocType
		env           Environment
		establishment string
		emissionPoint string
		sequential    string
		random        []byte
		want          string
	}{
		{"invoice", time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), DocInvoice, EnvironmentTest, "001", "001", "000000007", code12345678,
			"1811202601179000000200110010010000000071234567818"},
		{"check digit 0", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), DocInvoice, EnvironmentProduction, "001", "002", "000000005", code42,
			"0102202601179000000200120010020000000050000004210"},
		{"check digit 1", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), DocInvoice, EnvironmentProduction, "001", "002", "000000003", code42,
			"0102202601179000000200120010020000000030000004211"},
		{"credit note", time.Date(2025, 4, 20, 0, 0, 0, 0, time.UTC), DocCreditNote, EnvironmentProduction, "001", "002", "000000015", []byte{0x00, 0x01, 0x86, 0x9F},
			"2004202504179000000200120010020000000150009999910"},
		{"withholding", time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC), DocWithholding, EnvironmentTest, "001", "002", "000000015", code12345678,
			"1611202607179000000200110010020000000151234567819"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateAccessKey(tt.date, tt.doc, "1790000002001", tt.env, tt.establishment, tt.emissionPoint, tt.sequential, bytes.NewReader(tt.random))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.True(t, ValidAccessKey(got))
		})
	}
}

func TestGenerateAccessKeyRejectsBadInput(t *testing.T) {
	date := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		doc           DocType
		ruc           string
		env           Environment
		establishment string
		emissionPoint string
		sequential    string
	}{
		{"short ruc", DocInvoice, "179000000200", EnvironmentTest, "001", "001", "000000007"},
		{"ruc not ending in 001", DocInvoice, "1790000002002", EnvironmentTest, "001", "001", "000000007"},
		{"ruc with letters", DocInvoice, "17900000020A1", EnvironmentTest, "001", "001", "000000007"},
		{"ruc with province 99", DocInvoice, "9990000000001", EnvironmentTest, "001", "001", "000000007"},
		{"ruc with province 00", DocInvoice, "0090000000001", EnvironmentTest, "001", "001", "000000007"},
		{"unknown environment", DocInvoice, "1790000002001", Environment(3), "001", "001", "000000007"},
		{"unknown doc type", DocType("99"), "1790000002001", EnvironmentTest, "001", "001", "000000007"},
		{"short establishment", DocInvoice, "1790000002001", EnvironmentTest, "01", "001", "000000007"},
		{"long emission point", DocInvoice, "1790000002001", EnvironmentTest, "001", "0001", "000000007"},
		{"short sequential", DocInvoice, "1790000002001", EnvironmentTest, "001", "001", "7"},
		{"sequential with letters", DocInvoice, "1790000002001", EnvironmentTest, "001", "001", "00000000A"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateAccessKey(date, tt.doc, tt.ruc, tt.env, tt.establishment, tt.emissionPoint, tt.sequential, bytes.NewReader([]byte{0, 0, 0, 1}))
			require.Error(t, err)
			assert.Empty(t, got)
		})
	}
}

func TestGenerateAccessKeyRandomFailure(t *testing.T) {
	date := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	_, err := GenerateAccessKey(date, DocInvoice, "1790000002001", EnvironmentTest, "001", "001", "000000007", strings.NewReader(""))
	require.Error(t, err)
}

func TestParseAccessKey(t *testing.T) {
	got, err := ParseAccessKey("1811202601179000000200110010010000000071234567818")
	require.NoError(t, err)
	assert.Equal(t, AccessKeyParts{
		IssueDate:     time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC),
		Doc:           DocInvoice,
		RUC:           "1790000002001",
		Environment:   EnvironmentTest,
		Establishment: "001",
		EmissionPoint: "001",
		Sequential:    "000000007",
		NumericCode:   "12345678",
		EmissionType:  "1",
	}, got)

	_, err = ParseAccessKey("1811202601179000000200110010010000000071234567816")
	require.Error(t, err, "wrong check digit")
	_, err = ParseAccessKey("123")
	require.Error(t, err, "too short")
	_, err = ParseAccessKey("3002202601179000000200110010010000000071234567818")
	require.Error(t, err, "impossible date 30/02")
}

func TestParseAccessKeyRoundTrip(t *testing.T) {
	date := time.Date(2025, 4, 20, 0, 0, 0, 0, time.UTC)
	key, err := GenerateAccessKey(date, DocCreditNote, "1790000002001", EnvironmentProduction, "001", "002", "000000015", bytes.NewReader([]byte{0x00, 0x01, 0x86, 0x9F}))
	require.NoError(t, err)
	parts, err := ParseAccessKey(key)
	require.NoError(t, err)
	assert.Equal(t, date, parts.IssueDate)
	assert.Equal(t, DocCreditNote, parts.Doc)
	assert.Equal(t, EnvironmentProduction, parts.Environment)
	assert.Equal(t, "00099999", parts.NumericCode)
	assert.Equal(t, "000000015", parts.Sequential)
}
