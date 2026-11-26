package sri

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleCreditNote() CreditNote {
	return CreditNote{
		Header:         testHeader(),
		Issuer:         testIssuer(),
		Buyer:          testBuyer(),
		ModifiedNumber: "001-001-000000123",
		ModifiedDate:   time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
		Reason:         "Devolución de mercadería",
		Lines:          []Line{line("1", "100", "0", 15)},
		Additional:     []AdditionalField{{Name: "Correo", Value: "cliente@ejemplo.com"}},
	}
}

func TestCreditNoteValidateSample(t *testing.T) {
	cn := sampleCreditNote()
	assert.Nil(t, cn.Validate())
	cn.Header.AccessKey = keyFor(t, DocCreditNote, cn.Header, cn.Issuer.RUC)
	require.NoError(t, cn.Validate())
}

func TestCreditNoteValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreditNote)
		want   []string
	}{
		{"modified number without hyphens", func(c *CreditNote) { c.ModifiedNumber = "001001000000123" }, []string{"modified_number credit_note_modified_invalid"}},
		{"modified number with a short sequential", func(c *CreditNote) { c.ModifiedNumber = "001-001-123" }, []string{"modified_number credit_note_modified_invalid"}},
		{"empty modified number", func(c *CreditNote) { c.ModifiedNumber = "" }, []string{"modified_number credit_note_modified_invalid"}},
		{"no modified date", func(c *CreditNote) { c.ModifiedDate = time.Time{} }, []string{"modified_date credit_note_modified_invalid"}},
		{"blank reason", func(c *CreditNote) { c.Reason = " \n" }, []string{"reason credit_note_reason_required"}},
		{"reason of 301 characters", func(c *CreditNote) { c.Reason = strings.Repeat("m", 301) }, []string{"reason credit_note_reason_required"}},
		{"reason of 300 characters", func(c *CreditNote) { c.Reason = strings.Repeat("m", 300) }, nil},
		{"final consumer buyer", func(c *CreditNote) {
			c.Buyer = Party{IDType: IDFinalConsumer, ID: "9999999999999", Name: "CONSUMIDOR FINAL"}
		}, []string{"buyer.id_type credit_note_final_consumer"}},
		{"cédula buyer", func(c *CreditNote) { c.Buyer.IDType, c.Buyer.ID = IDCedula, "1700000001" }, nil},
		{"access key of an invoice", func(c *CreditNote) {
			c.Header.AccessKey = keyFor(t, DocInvoice, c.Header, c.Issuer.RUC)
		}, []string{"header.access_key access_key_mismatch"}},
		{"issuer, header and lines are checked", func(c *CreditNote) {
			c.Issuer.MainAddress = ""
			c.Header.EmissionPoint = ""
			c.Lines = nil
		}, []string{
			"issuer.main_address issuer_address_required",
			"header.emission_point header_emission_point_invalid",
			"lines lines_required",
		}},
		{"every item worth zero", func(c *CreditNote) { c.Lines = []Line{line("1", "0", "0", 15)} }, []string{"lines total_zero"}},
		{"every item fully discounted", func(c *CreditNote) { c.Lines = []Line{line("2", "5", "10", 0)} }, []string{"lines total_zero"}},
		{"a negative item is reported once", func(c *CreditNote) { c.Lines = []Line{line("-1", "5", "0", 0)} }, []string{"lines[0].quantity line_negative_amount"}},
		{"line rules apply", func(c *CreditNote) { c.Lines[0].TaxRate = 8 }, []string{"lines[0].tax_rate line_tax_rate_unsupported"}},
		{"additional fields are checked", func(c *CreditNote) { c.Additional[0].Value = "" }, []string{"additional[0] additional_field_invalid"}},
		{"a credit note has no payments", func(c *CreditNote) {}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cn := sampleCreditNote()
			tt.mutate(&cn)
			assert.Equal(t, tt.want, problems(t, cn.Validate()))
		})
	}
}
