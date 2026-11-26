package sri

import (
	"maps"
	"slices"

	"github.com/shopspring/decimal"
)

// paymentMethods is the SRI payment-method table (ficha técnica, tabla 24).
var paymentMethods = map[string]string{
	"01": "Efectivo",
	"15": "Compensación de deudas",
	"16": "Tarjeta de débito",
	"17": "Dinero electrónico",
	"18": "Tarjeta prepago",
	"19": "Tarjeta de crédito",
	"20": "Transferencia u otros con el sistema financiero",
	"21": "Endoso de títulos",
}

// PaymentMethods returns the SRI payment-method catalog: code → Spanish name.
// The map is a copy; changing it does not change the catalog.
func PaymentMethods() map[string]string { return maps.Clone(paymentMethods) }

func validPaymentMethod(code string) bool {
	_, ok := paymentMethods[code]
	return ok
}

// sustentosByDoc is the sustento code (ATS tabla 5) each support document
// accepts, from the DIMM 1.19.0 table IVA_SECUENCIALES_COMPROBANTES, rows 01
// (invoice) and 02 (sales note).
var sustentosByDoc = map[DocType][]string{
	DocInvoice:   {"01", "02", "03", "04", "05", "06", "07", "08", "09", "14", "15"},
	DocSalesNote: {"02", "04", "05", "07", "08", "14", "15"},
}

// sustentoAllowed reports whether a withholding's support document of type
// doc accepts the sustento code.
func sustentoAllowed(doc DocType, code string) bool {
	return slices.Contains(sustentosByDoc[doc], code)
}

// ivaWithholdingCodes maps each VAT withholding percentage to its SRI code
// (ATS tabla 11).
var ivaWithholdingCodes = []struct {
	pct  decimal.Decimal
	code string
}{
	{decimal.NewFromInt(10), "9"}, {decimal.NewFromInt(20), "10"}, {decimal.NewFromInt(30), "1"},
	{decimal.NewFromInt(50), "11"}, {decimal.NewFromInt(70), "2"}, {decimal.NewFromInt(100), "3"},
}

// ivaWithholdingCode returns the SRI code of a VAT withholding percentage;
// ok is false for any percentage other than 10, 20, 30, 50, 70 or 100.
func ivaWithholdingCode(pct decimal.Decimal) (string, bool) {
	for _, c := range ivaWithholdingCodes {
		if c.pct.Equal(pct) {
			return c.code, true
		}
	}
	return "", false
}
