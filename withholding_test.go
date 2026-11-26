package sri

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleWithholding withholds 1.75 % of income tax on 100 and 30 % of the 15
// of VAT of a supplier's invoice that totals 115.
func sampleWithholding() Withholding {
	return Withholding{
		Header:       testHeader(),
		Issuer:       testIssuer(),
		Supplier:     Party{IDType: IDRUC, ID: "1790000000001", Name: "PROVEEDOR & HIJOS S.A.", Email: "proveedor@ejemplo.com"},
		FiscalPeriod: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Support: SupportDocument{
			DocType: DocInvoice, Number: "001-001-000000123",
			IssueDate:     time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
			Authorization: "1411202601179000000000110010010000001231234567811",
			SustentoCode:  "01",
			Subtotal0:     dec("0"), Subtotal15: dec("100"), IVA: dec("15"), Total: dec("115"),
			Payments: []Payment{{Method: "20", Amount: dec("115")}},
		},
		Lines: []WithholdingLine{
			{Tax: WithholdingRenta, Code: "312", Base: dec("100"), Percentage: dec("1.75"), Amount: dec("1.75")},
			{Tax: WithholdingIVA, Code: "1", Base: dec("15"), Percentage: dec("30"), Amount: dec("4.50")},
		},
		Additional: []AdditionalField{{Name: "Correo", Value: "proveedor@ejemplo.com"}},
	}
}

func TestWithholdingValidateSample(t *testing.T) {
	w := sampleWithholding()
	assert.Nil(t, w.Validate())
	w.Header.AccessKey = keyFor(t, DocWithholding, w.Header, w.Issuer.RUC)
	require.NoError(t, w.Validate())
}

func TestWithholdingValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Withholding)
		want   []string
	}{
		// Header, issuer and supplier.
		{"access key of an invoice", func(w *Withholding) {
			w.Header.AccessKey = keyFor(t, DocInvoice, w.Header, w.Issuer.RUC)
		}, []string{"header.access_key access_key_mismatch"}},
		{"issuer is checked", func(w *Withholding) { w.Issuer.RUC = "1790000002002" }, []string{"issuer.ruc issuer_ruc_invalid"}},
		{"final consumer supplier", func(w *Withholding) {
			w.Supplier = Party{IDType: IDFinalConsumer, ID: "9999999999999", Name: "CONSUMIDOR FINAL"}
		}, []string{"supplier.id_type party_id_type_invalid"}},
		{"foreign supplier (08)", func(w *Withholding) {
			w.Supplier = Party{IDType: IDForeign, ID: "X-99887766", Name: "FOREIGN SUPPLIER LTD"}
		}, []string{"supplier.id_type party_id_type_invalid"}},
		{"passport supplier (06)", func(w *Withholding) {
			w.Supplier = Party{IDType: IDPassport, ID: "AB123456", Name: "JOHN SMITH"}
		}, nil},
		{"supplier with a bad cédula", func(w *Withholding) { w.Supplier.IDType, w.Supplier.ID = IDCedula, "1700000002" }, []string{"supplier.id party_cedula_invalid"}},
		{"supplier without name", func(w *Withholding) { w.Supplier.Name = "" }, []string{"supplier.name party_name_required"}},
		{"no fiscal period", func(w *Withholding) { w.FiscalPeriod = time.Time{} }, []string{"fiscal_period withholding_period_required"}},

		// Support document.
		{"support of type 04", func(w *Withholding) { w.Support.DocType = DocCreditNote }, []string{"support.doc_type support_document_invalid"}},
		{"support number without hyphens", func(w *Withholding) { w.Support.Number = "001001000000123" }, []string{"support.number support_document_invalid"}},
		{"support without issue date", func(w *Withholding) { w.Support.IssueDate = time.Time{} }, []string{"support.issue_date support_document_invalid"}},
		{"authorization of 3 digits", func(w *Withholding) { w.Support.Authorization = "123" }, []string{"support.authorization support_document_invalid"}},
		{"authorization of 11 digits", func(w *Withholding) { w.Support.Authorization = "12345678901" }, []string{"support.authorization support_document_invalid"}},
		{"preprinted authorization of 10 digits", func(w *Withholding) { w.Support.Authorization = "1234567890" }, nil},
		{"authorization of 37 digits", func(w *Withholding) { w.Support.Authorization = strings.Repeat("1", 37) }, nil},
		{"no authorization", func(w *Withholding) { w.Support.Authorization = "" }, nil},
		{"sustento 06 is fine for an invoice", func(w *Withholding) { w.Support.SustentoCode = "06" }, nil},
		{"sustento 06 on a sales note", func(w *Withholding) {
			w.Support.DocType = DocSalesNote
			w.Support.SustentoCode = "06"
		}, []string{"support.sustento_code support_sustento_invalid"}},
		{"sustento 02 on a sales note", func(w *Withholding) {
			w.Support.DocType = DocSalesNote
			w.Support.SustentoCode = "02"
		}, nil},
		{"sustento 10 on an invoice", func(w *Withholding) { w.Support.SustentoCode = "10" }, []string{"support.sustento_code support_sustento_invalid"}},
		{"empty sustento", func(w *Withholding) { w.Support.SustentoCode = "" }, []string{"support.sustento_code support_sustento_invalid"}},
		{"support total one cent off", func(w *Withholding) {
			w.Support.Total = dec("115.01")
			w.Support.Payments[0].Amount = dec("115.01")
		}, []string{"support.total support_totals_mismatch"}},
		{"support with a negative subtotal", func(w *Withholding) {
			w.Support.Subtotal0 = dec("-1")
			w.Support.Total = dec("114")
			w.Support.Payments[0].Amount = dec("114")
		}, []string{"support.total support_totals_mismatch"}},
		{"support with VAT but no base", func(w *Withholding) {
			w.Support.Subtotal0, w.Support.Subtotal15, w.Support.IVA, w.Support.Total = dec("0"), dec("0"), dec("15"), dec("15")
			w.Support.Payments[0].Amount = dec("15")
		}, []string{"support support_totals_mismatch"}},
		{"support worth zero", func(w *Withholding) {
			w.Support.Subtotal0, w.Support.Subtotal15, w.Support.IVA, w.Support.Total = dec("0"), dec("0"), dec("0"), dec("0")
			w.Support.Payments = nil
		}, []string{"support support_totals_mismatch", "support.payments payments_required"}},
		{"support with VAT on a 0 % base only", func(w *Withholding) {
			w.Support.Subtotal0, w.Support.Subtotal15, w.Support.IVA, w.Support.Total = dec("100"), dec("0"), dec("15"), dec("115")
		}, []string{"support support_totals_mismatch"}},
		{"support with a 15 % base and no VAT is fine", func(w *Withholding) {
			w.Support.IVA, w.Support.Total = dec("0"), dec("100")
			w.Support.Payments[0].Amount = dec("100")
		}, nil},
		{"support with both subtotals", func(w *Withholding) {
			w.Support.Subtotal0 = dec("20")
			w.Support.Total = dec("135")
			w.Support.Payments[0].Amount = dec("135")
		}, nil},

		// Support payments.
		{"support without payments", func(w *Withholding) { w.Support.Payments = nil }, []string{"support.payments payments_required"}},
		{"support payment with an unknown method", func(w *Withholding) { w.Support.Payments[0].Method = "00" }, []string{"support.payments[0].method payment_method_invalid"}},
		{"support amounts with 3 decimals", func(w *Withholding) {
			w.Support.Subtotal15, w.Support.IVA = dec("100.005"), dec("14.995")
		}, []string{"support.subtotal15 amount_too_many_decimals", "support.iva amount_too_many_decimals"}},
		{"support subtotal 0 and total with 3 decimals", func(w *Withholding) {
			w.Support.Subtotal0, w.Support.Subtotal15 = dec("0.001"), dec("99.999")
		}, []string{"support.subtotal0 amount_too_many_decimals", "support.subtotal15 amount_too_many_decimals"}},
		{"support total with 3 decimals", func(w *Withholding) {
			w.Support.Subtotal15, w.Support.Total = dec("100.001"), dec("115.001")
			w.Support.Payments = []Payment{{Method: "20", Amount: dec("115"), TermDays: 0}, {Method: "01", Amount: dec("0.001")}}
		}, []string{
			"support.subtotal15 amount_too_many_decimals", "support.total amount_too_many_decimals",
			"support.payments[1].amount amount_too_many_decimals",
		}},
		{"support payment with a negative term", func(w *Withholding) { w.Support.Payments[0].TermDays = -1 }, []string{"support.payments[0].term_days payment_amount_invalid"}},
		{"support payments short of the total", func(w *Withholding) { w.Support.Payments[0].Amount = dec("114.99") }, []string{"support.payments payments_total_mismatch"}},

		// Lines.
		{"no lines", func(w *Withholding) { w.Lines = nil }, []string{"lines lines_required"}},
		{"base with 3 decimals", func(w *Withholding) { w.Lines[0].Base = dec("100.005") }, []string{"lines[0].base amount_too_many_decimals"}},
		{"amount with 3 decimals", func(w *Withholding) { w.Lines[1].Amount = dec("4.505") }, []string{"lines[1].amount amount_too_many_decimals"}},
		{"unknown tax", func(w *Withholding) { w.Lines[0].Tax = "ice" }, []string{"lines[0].tax withholding_line_invalid"}},
		{"empty code", func(w *Withholding) { w.Lines[0].Code = " " }, []string{"lines[0].code withholding_line_invalid"}},
		{"code of 6 characters", func(w *Withholding) { w.Lines[0].Code = "312345" }, []string{"lines[0].code withholding_line_invalid"}},
		{"code of 5 characters", func(w *Withholding) { w.Lines[0].Code = "3440A" }, nil},
		{"zero base", func(w *Withholding) { w.Lines[0].Base = dec("0") }, []string{"lines[0].base withholding_line_invalid"}},
		{"negative percentage", func(w *Withholding) { w.Lines[0].Percentage = dec("-1") }, []string{"lines[0].percentage withholding_line_invalid"}},
		{"percentage above 100", func(w *Withholding) { w.Lines[0].Percentage = dec("100.01") }, []string{"lines[0].percentage withholding_line_invalid"}},
		{"percentage with 3 decimals", func(w *Withholding) { w.Lines[0].Percentage = dec("1.755") }, []string{"lines[0].percentage withholding_line_invalid"}},
		{"informative income tax line at 0 %", func(w *Withholding) {
			w.Lines = []WithholdingLine{{Tax: WithholdingRenta, Code: "332", Base: dec("100"), Percentage: dec("0"), Amount: dec("0")}}
		}, nil},
		{"VAT at 25 %", func(w *Withholding) {
			w.Lines[1].Percentage = dec("25")
			w.Lines[1].Amount = dec("3.75")
		}, []string{"lines[1].percentage withholding_iva_percentage"}},
		{"VAT at 0 %", func(w *Withholding) {
			w.Lines[1].Percentage = dec("0")
			w.Lines[1].Amount = dec("0")
		}, []string{"lines[1].percentage withholding_iva_percentage"}},
		{"VAT at 100 %", func(w *Withholding) {
			w.Lines[1].Code, w.Lines[1].Percentage, w.Lines[1].Amount = "3", dec("100"), dec("15")
		}, nil},
		{"VAT at 70 % with the code of 30 %", func(w *Withholding) {
			w.Lines[1].Percentage, w.Lines[1].Amount = dec("70"), dec("10.50")
		}, []string{"lines[1].code withholding_line_invalid"}},
		{"amount one cent above", func(w *Withholding) { w.Lines[1].Amount = dec("4.51") }, nil},
		{"amount one cent below", func(w *Withholding) { w.Lines[1].Amount = dec("4.49") }, nil},
		{"amount two cents above", func(w *Withholding) { w.Lines[1].Amount = dec("4.52") }, []string{"lines[1].amount withholding_amount_mismatch"}},
		{"amount two cents below", func(w *Withholding) { w.Lines[1].Amount = dec("4.48") }, []string{"lines[1].amount withholding_amount_mismatch"}},
		{"a fraction of a cent rounds to one cent", func(w *Withholding) {
			// 0.30 × 1.75 % = 0.00525 -> 0.01
			w.Lines[0].Base, w.Lines[0].Amount = dec("0.30"), dec("0.01")
		}, nil},
		{"amount not checked on an invalid line", func(w *Withholding) {
			w.Lines[0].Base = dec("-100")
		}, []string{"lines[0].base withholding_line_invalid"}},

		// Additional fields.
		{"additional fields are checked", func(w *Withholding) { w.Additional[0].Name = "" }, []string{"additional[0] additional_field_invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := sampleWithholding()
			tt.mutate(&w)
			assert.Equal(t, tt.want, problems(t, w.Validate()))
		})
	}
}

func TestWithholdingAmountTolerance(t *testing.T) {
	// 33.33 × 30 % = 9.999 -> 10.00; 9.99 and 10.01 are within the cent.
	for amount, ok := range map[string]bool{"10.00": true, "9.99": true, "10.01": true, "9.98": false, "10.02": false} {
		w := sampleWithholding()
		w.Lines[1].Base, w.Lines[1].Amount = dec("33.33"), dec(amount)
		if ok {
			assert.NoError(t, w.Validate(), amount)
		} else {
			assert.Equal(t, []string{"lines[1].amount withholding_amount_mismatch"}, problems(t, w.Validate()), amount)
		}
	}
}
