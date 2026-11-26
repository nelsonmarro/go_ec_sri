package sri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaymentMethods(t *testing.T) {
	want := map[string]string{
		"01": "Efectivo",
		"15": "Compensación de deudas",
		"16": "Tarjeta de débito",
		"17": "Dinero electrónico",
		"18": "Tarjeta prepago",
		"19": "Tarjeta de crédito",
		"20": "Transferencia u otros con el sistema financiero",
		"21": "Endoso de títulos",
	}
	assert.Equal(t, want, PaymentMethods())
	for code := range want {
		assert.True(t, validPaymentMethod(code), code)
	}
	for _, code := range []string{"", "1", "02", "99", " 01"} {
		assert.False(t, validPaymentMethod(code), code)
	}
}

func TestPaymentMethodsReturnsACopy(t *testing.T) {
	m := PaymentMethods()
	m["99"] = "Otro"
	delete(m, "01")
	assert.True(t, validPaymentMethod("01"))
	assert.False(t, validPaymentMethod("99"))
	assert.Len(t, PaymentMethods(), 8)
}

func TestSustentoAllowed(t *testing.T) {
	invoice := []string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "14", "15"}
	salesNote := []string{"02", "04", "05", "07", "08", "14", "15"}
	all := []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", ""}
	in := func(list []string, s string) bool {
		for _, v := range list {
			if v == s {
				return true
			}
		}
		return false
	}
	for _, code := range all {
		assert.Equal(t, in(invoice, code), sustentoAllowed(DocType("01"), code), "invoice %q", code)
		assert.Equal(t, in(salesNote, code), sustentoAllowed(DocType("02"), code), "sales note %q", code)
		assert.False(t, sustentoAllowed(DocType("04"), code), "credit note %q", code)
	}
}

func TestIVAWithholdingCode(t *testing.T) {
	tests := []struct {
		pct, code string
		ok        bool
	}{
		{"10", "9", true},
		{"20", "10", true},
		{"30", "1", true},
		{"50", "11", true},
		{"70", "2", true},
		{"100", "3", true},
		{"30.00", "1", true},
		{"0", "", false},
		{"12", "", false},
		{"15", "", false},
		{"25", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.pct, func(t *testing.T) {
			code, ok := ivaWithholdingCode(dec(tt.pct))
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.code, code)
		})
	}
}
