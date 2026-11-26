package sri

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// code12345678 is the 4 bytes crypto/rand.Int reads to draw 12345678.
var code12345678 = []byte{0x00, 0xBC, 0x61, 0x4E}

func testIssuer() Issuer {
	return Issuer{RUC: "1790000002001", BusinessName: "NEGOCIO DE PRUEBA S.A.", MainAddress: "Av. Amazonas N1-1"}
}

func testHeader() Header {
	return Header{
		Environment: EnvironmentTest, Establishment: "001", EmissionPoint: "001", Sequential: "7",
		IssueDate: time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC),
	}
}

func testBuyer() Party {
	return Party{IDType: IDRUC, ID: "1790000000001", Name: "CLIENTE & HIJOS S.A.", Email: "cliente@ejemplo.com"}
}

// sampleInvoice totals 123.50: 20 at 0 % plus 90 at 15 % (13.50 of VAT).
func sampleInvoice() Invoice {
	return Invoice{
		Header:     testHeader(),
		Issuer:     testIssuer(),
		Buyer:      testBuyer(),
		Lines:      []Line{line("2", "10", "0", 0), line("1", "100", "10", 15)},
		Payments:   []Payment{{Method: "01", Amount: dec("123.50")}},
		Additional: []AdditionalField{{Name: "Correo", Value: "cliente@ejemplo.com"}},
	}
}

// keyFor generates the access key the header and issuer of a document imply.
func keyFor(t *testing.T, doc DocType, h Header, ruc string) string {
	t.Helper()
	seq := strings.Repeat("0", 9-len(h.Sequential)) + h.Sequential
	key, err := GenerateAccessKey(h.IssueDate, doc, ruc, h.Environment, h.Establishment, h.EmissionPoint, seq, bytes.NewReader(code12345678))
	require.NoError(t, err)
	return key
}

// problems returns "field code" for each problem and checks every message
// is the one of its code.
func problems(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var errs ValidationErrors
	require.True(t, errors.As(err, &errs), "error is %T, want ValidationErrors", err)
	out := make([]string, len(errs))
	for i, e := range errs {
		assert.Equal(t, validationMessages[e.Code], e.Message, e.Code)
		assert.NotEmpty(t, e.Message, e.Code)
		out[i] = e.Field + " " + e.Code
	}
	return out
}

// payTotal makes the single payment equal the invoice total, so a case that
// changes the lines reports only its own problem.
func payTotal(i *Invoice) { i.Payments = []Payment{{Method: "01", Amount: i.Totals().Total}} }

func TestInvoiceValidateSample(t *testing.T) {
	inv := sampleInvoice()
	require.NoError(t, inv.Validate())
	assert.Nil(t, inv.Validate())
	inv.Header.AccessKey = keyFor(t, DocInvoice, inv.Header, inv.Issuer.RUC)
	assert.Equal(t, "1811202601179000000200110010010000000071234567818", inv.Header.AccessKey)
	assert.NoError(t, inv.Validate())
}

func TestInvoiceValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Invoice)
		want   []string
	}{
		// Issuer.
		{"issuer RUC with 12 digits", func(i *Invoice) { i.Issuer.RUC = "179000000200" }, []string{"issuer.ruc issuer_ruc_invalid"}},
		{"issuer RUC failing módulo 11 is accepted", func(i *Invoice) { i.Issuer.RUC = "1790000001001" }, nil},
		{"issuer without business name", func(i *Invoice) { i.Issuer.BusinessName = "  " }, []string{"issuer.business_name issuer_name_required"}},
		{"issuer without main address", func(i *Invoice) { i.Issuer.MainAddress = "" }, []string{"issuer.main_address issuer_address_required"}},

		{"withholding agent resolution of 1 digit", func(i *Invoice) { i.Issuer.WithholdingAgentResolution = "1" }, nil},
		{"withholding agent resolution of 8 digits", func(i *Invoice) { i.Issuer.WithholdingAgentResolution = "12345678" }, nil},
		{"withholding agent resolution of 9 digits", func(i *Invoice) { i.Issuer.WithholdingAgentResolution = "123456789" }, []string{"issuer.withholding_agent_resolution issuer_withholding_agent_invalid"}},
		{"withholding agent resolution as printed", func(i *Invoice) {
			i.Issuer.WithholdingAgentResolution = "NAC-DNCRASC20-00000001"
		}, []string{"issuer.withholding_agent_resolution issuer_withholding_agent_invalid"}},
		{"special taxpayer number of 3 characters", func(i *Invoice) { i.Issuer.SpecialTaxpayerNumber = "A12" }, nil},
		{"special taxpayer number of 13 characters", func(i *Invoice) { i.Issuer.SpecialTaxpayerNumber = "ABCDEFG123456" }, nil},
		{"special taxpayer number of 2 characters", func(i *Invoice) { i.Issuer.SpecialTaxpayerNumber = "12" }, []string{"issuer.special_taxpayer_number issuer_special_taxpayer_invalid"}},
		{"special taxpayer number of 14 characters", func(i *Invoice) { i.Issuer.SpecialTaxpayerNumber = "ABCDEFG1234567" }, []string{"issuer.special_taxpayer_number issuer_special_taxpayer_invalid"}},
		{"special taxpayer number with a hyphen", func(i *Invoice) { i.Issuer.SpecialTaxpayerNumber = "123-45" }, []string{"issuer.special_taxpayer_number issuer_special_taxpayer_invalid"}},
		{"RIMPE entrepreneur", func(i *Invoice) { i.Issuer.Rimpe = RimpeEntrepreneur }, nil},
		{"RIMPE popular business", func(i *Invoice) { i.Issuer.Rimpe = RimpePopularBusiness }, nil},
		{"unknown RIMPE regime", func(i *Invoice) { i.Issuer.Rimpe = "rimpe" }, []string{"issuer.rimpe issuer_rimpe_invalid"}},

		// Header.
		{"environment 0", func(i *Invoice) { i.Header.Environment = 0 }, []string{"header.environment header_environment_invalid"}},
		{"environment 3", func(i *Invoice) { i.Header.Environment = 3 }, []string{"header.environment header_environment_invalid"}},
		{"production environment", func(i *Invoice) { i.Header.Environment = EnvironmentProduction }, nil},
		{"establishment with 1 digit", func(i *Invoice) { i.Header.Establishment = "1" }, []string{"header.establishment header_establishment_invalid"}},
		{"establishment with letters", func(i *Invoice) { i.Header.Establishment = "00A" }, []string{"header.establishment header_establishment_invalid"}},
		{"emission point with 4 digits", func(i *Invoice) { i.Header.EmissionPoint = "0011" }, []string{"header.emission_point header_emission_point_invalid"}},
		{"empty sequential", func(i *Invoice) { i.Header.Sequential = "" }, []string{"header.sequential header_sequential_invalid"}},
		{"zero sequential", func(i *Invoice) { i.Header.Sequential = "000000000" }, []string{"header.sequential header_sequential_invalid"}},
		{"sequential with 10 digits", func(i *Invoice) { i.Header.Sequential = "1234567890" }, []string{"header.sequential header_sequential_invalid"}},
		{"sequential with a letter", func(i *Invoice) { i.Header.Sequential = "12a" }, []string{"header.sequential header_sequential_invalid"}},
		{"sequential with 9 digits", func(i *Invoice) { i.Header.Sequential = "000000007" }, nil},
		{"sequential with 1 digit", func(i *Invoice) { i.Header.Sequential = "1" }, nil},
		{"no issue date", func(i *Invoice) { i.Header.IssueDate = time.Time{} }, []string{"header.issue_date header_issue_date_required"}},

		// Access key.
		{"access key with a wrong check digit", func(i *Invoice) {
			i.Header.AccessKey = "1811202601179000000200110010010000000071234567816"
		}, []string{"header.access_key access_key_invalid"}},
		{"access key of 48 digits", func(i *Invoice) {
			i.Header.AccessKey = "181120260117900000020011001001000000007123456781"
		}, []string{"header.access_key access_key_invalid"}},
		{"access key of another date", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
			i.Header.IssueDate = i.Header.IssueDate.AddDate(0, 0, 1)
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key of another RUC", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, "1790000003001")
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key of another environment", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
			i.Header.Environment = EnvironmentProduction
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key of another establishment", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
			i.Header.Establishment = "002"
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key of another emission point", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
			i.Header.EmissionPoint = "002"
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key of another sequential", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
			i.Header.Sequential = "8"
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key with emission type 2", func(i *Invoice) {
			key48 := keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)[:47] + "2"
			i.Header.AccessKey = key48 + strconv.Itoa(checkDigit(key48))
			require.True(t, ValidAccessKey(i.Header.AccessKey))
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key of another document type", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocCreditNote, i.Header, i.Issuer.RUC)
		}, []string{"header.access_key access_key_mismatch"}},
		{"access key with a 9 digit sequential", func(i *Invoice) {
			i.Header.Sequential = "000000007"
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
		}, nil},
		{"access key with the padded sequential", func(i *Invoice) {
			i.Header.AccessKey = keyFor(t, DocInvoice, i.Header, i.Issuer.RUC)
			i.Header.Sequential = "0007"
		}, nil},

		// Buyer.
		{"unknown id type", func(i *Invoice) { i.Buyer.IDType = "09" }, []string{"buyer.id_type party_id_type_invalid"}},
		{"empty id type", func(i *Invoice) { i.Buyer.IDType = "" }, []string{"buyer.id_type party_id_type_invalid"}},
		{"malformed RUC", func(i *Invoice) { i.Buyer.ID = "1790000000002" }, []string{"buyer.id party_ruc_invalid"}},
		{"valid cédula", func(i *Invoice) { i.Buyer.IDType, i.Buyer.ID = IDCedula, "1700000001" }, nil},
		{"cédula with a wrong check digit", func(i *Invoice) { i.Buyer.IDType, i.Buyer.ID = IDCedula, "1700000002" }, []string{"buyer.id party_cedula_invalid"}},
		{"RUC sent as cédula", func(i *Invoice) { i.Buyer.IDType = IDCedula }, []string{"buyer.id party_cedula_invalid"}},
		{"passport", func(i *Invoice) { i.Buyer.IDType, i.Buyer.ID = IDPassport, "AB123456" }, nil},
		{"passport too short", func(i *Invoice) { i.Buyer.IDType, i.Buyer.ID = IDPassport, "A1" }, []string{"buyer.id party_passport_invalid"}},
		{"foreign id", func(i *Invoice) { i.Buyer.IDType, i.Buyer.ID = IDForeign, "X-9988776" }, nil},
		{"foreign id with spaces", func(i *Invoice) { i.Buyer.IDType, i.Buyer.ID = IDForeign, "X 9988776" }, []string{"buyer.id party_passport_invalid"}},
		{"final consumer with another id", func(i *Invoice) {
			i.Buyer = Party{IDType: IDFinalConsumer, ID: "1790000000001", Name: "CONSUMIDOR FINAL"}
			i.Lines = []Line{line("1", "10", "0", 0)}
			payTotal(i)
		}, []string{"buyer.id party_final_consumer_id"}},
		{"final consumer at exactly 50.00 with VAT", func(i *Invoice) {
			i.Buyer = Party{IDType: IDFinalConsumer, ID: "9999999999999", Name: "CONSUMIDOR FINAL"}
			i.Lines = []Line{line("1", "43.48", "0", 15)} // 43.48 + 6.52
			payTotal(i)
		}, nil},
		{"final consumer at 50.01 with VAT", func(i *Invoice) {
			i.Buyer = Party{IDType: IDFinalConsumer, ID: "9999999999999", Name: "CONSUMIDOR FINAL"}
			i.Lines = []Line{line("1", "43.49", "0", 15)} // 43.49 + 6.52
			payTotal(i)
		}, []string{"buyer buyer_final_consumer_limit"}},
		{"final consumer at 50.01 without VAT", func(i *Invoice) {
			i.Buyer = Party{IDType: IDFinalConsumer, ID: "9999999999999", Name: "CONSUMIDOR FINAL"}
			i.Lines = []Line{line("1", "50.01", "0", 0)}
			payTotal(i)
		}, []string{"buyer buyer_final_consumer_limit"}},
		{"buyer without name", func(i *Invoice) { i.Buyer.Name = " " }, []string{"buyer.name party_name_required"}},
		{"buyer without email", func(i *Invoice) { i.Buyer.Email = "" }, nil},
		{"buyer email without at", func(i *Invoice) { i.Buyer.Email = "cliente.ejemplo.com" }, []string{"buyer.email party_email_invalid"}},
		{"buyer email without a dotted domain", func(i *Invoice) { i.Buyer.Email = "cliente@ejemplo" }, []string{"buyer.email party_email_invalid"}},
		{"buyer email with a display name", func(i *Invoice) { i.Buyer.Email = "Cliente <cliente@ejemplo.com>" }, []string{"buyer.email party_email_invalid"}},
		{"buyer email with two addresses", func(i *Invoice) { i.Buyer.Email = "a@b.ec, c@d.ec" }, []string{"buyer.email party_email_invalid"}},

		// Lines.
		{"no lines", func(i *Invoice) { i.Lines = nil; i.Payments[0].Amount = dec("0") }, []string{"lines lines_required", "payments[0].amount payment_amount_invalid"}},
		{"every item worth zero", func(i *Invoice) {
			i.Lines = []Line{line("1", "0", "0", 15), line("3", "0", "0", 0)}
		}, []string{"lines total_zero", "payments payments_total_mismatch"}},
		{"every item fully discounted", func(i *Invoice) {
			i.Lines = []Line{line("1", "10", "10", 15)}
			i.Payments = nil
		}, []string{"lines total_zero", "payments payments_required"}},
		{"one cent of base is enough", func(i *Invoice) { i.Lines = []Line{line("1", "0.01", "0", 0)}; payTotal(i) }, nil},
		{"a single item with a discount above its value is not also total_zero", func(i *Invoice) {
			i.Lines = []Line{line("1", "10", "10.01", 0)}
			i.Payments = nil
		}, []string{"lines[0].discount line_discount_exceeds", "payments payments_required"}},
		{"blank description", func(i *Invoice) { i.Lines[1].Description = " \n " }, []string{"lines[1].description line_description_required"}},
		{"negative quantity", func(i *Invoice) { i.Lines[1].Quantity = dec("-1"); payTotal(i) }, []string{"lines[1].quantity line_negative_amount", "payments[0].amount payment_amount_invalid"}},
		{"negative price", func(i *Invoice) { i.Lines[1].UnitPrice = dec("-100"); payTotal(i) }, []string{"lines[1].unit_price line_negative_amount", "payments[0].amount payment_amount_invalid"}},
		{"negative discount", func(i *Invoice) { i.Lines[1].Discount = dec("-0.01"); payTotal(i) }, []string{"lines[1].discount line_negative_amount"}},
		{"discount above quantity × price", func(i *Invoice) { i.Lines[1].Discount = dec("100.01"); payTotal(i) }, []string{"lines[1].discount line_discount_exceeds"}},
		{"discount equal to quantity × price", func(i *Invoice) { i.Lines[1].Discount = dec("100"); payTotal(i) }, nil},
		{"rate 12", func(i *Invoice) { i.Lines[1].TaxRate = 12; payTotal(i) }, []string{"lines[1].tax_rate line_tax_rate_unsupported"}},
		{"rate 5", func(i *Invoice) { i.Lines[0].TaxRate = 5; payTotal(i) }, []string{"lines[0].tax_rate line_tax_rate_unsupported"}},
		{"quantity with 6 decimals", func(i *Invoice) { i.Lines[1].Quantity = dec("1.000001"); payTotal(i) }, nil},
		{"quantity with 7 decimals", func(i *Invoice) { i.Lines[1].Quantity = dec("1.0000001"); payTotal(i) }, []string{"lines[1].quantity amount_too_many_decimals"}},
		{"unit price with 6 decimals", func(i *Invoice) { i.Lines[0].UnitPrice = dec("10.123456"); payTotal(i) }, nil},
		{"unit price with 7 decimals", func(i *Invoice) { i.Lines[0].UnitPrice = dec("10.1234567"); payTotal(i) }, []string{"lines[0].unit_price amount_too_many_decimals"}},
		{"trailing zeros do not count as decimals", func(i *Invoice) { i.Lines[0].UnitPrice = dec("10.00000000"); payTotal(i) }, nil},
		{"discount with 2 decimals", func(i *Invoice) { i.Lines[1].Discount = dec("0.33"); payTotal(i) }, nil},
		{"discount with 3 decimals", func(i *Invoice) { i.Lines[1].Discount = dec("0.333"); payTotal(i) }, []string{"lines[1].discount amount_too_many_decimals"}},
		{"code of 26 characters", func(i *Invoice) { i.Lines[1].Code = strings.Repeat("C", 26) }, []string{"lines[1].code text_too_long"}},
		{"code of 25 characters", func(i *Invoice) { i.Lines[1].Code = strings.Repeat("Ñ", 25) }, nil},
		{"aux code of 26 characters", func(i *Invoice) { i.Lines[0].AuxCode = strings.Repeat("A", 26) }, []string{"lines[0].aux_code text_too_long"}},
		{"long description is capped, not rejected", func(i *Invoice) { i.Lines[0].Description = strings.Repeat("d", 400) }, nil},
		{"empty code is allowed", func(i *Invoice) { i.Lines[0].Code = "" }, nil},

		// Payments.
		{"no payments", func(i *Invoice) { i.Payments = nil }, []string{"payments payments_required"}},
		{"unknown payment method", func(i *Invoice) { i.Payments[0].Method = "99" }, []string{"payments[0].method payment_method_invalid"}},
		{"payments one cent short", func(i *Invoice) { i.Payments[0].Amount = dec("123.49") }, []string{"payments payments_total_mismatch"}},
		{"payment amounts with 3 decimals", func(i *Invoice) {
			i.Payments = []Payment{{Method: "19", Amount: dec("100.005")}, {Method: "20", Amount: dec("23.495")}}
		}, []string{"payments[0].amount amount_too_many_decimals", "payments[1].amount amount_too_many_decimals"}},
		{"a negative payment that keeps the sum", func(i *Invoice) {
			i.Payments = []Payment{{Method: "01", Amount: dec("133.50")}, {Method: "20", Amount: dec("-10")}}
		}, []string{"payments[1].amount payment_amount_invalid"}},
		{"a zero payment", func(i *Invoice) {
			i.Payments = append(i.Payments, Payment{Method: "20", Amount: dec("0")})
		}, []string{"payments[1].amount payment_amount_invalid"}},
		{"negative term", func(i *Invoice) { i.Payments[0].TermDays = -5 }, []string{"payments[0].term_days payment_amount_invalid"}},
		{"zero term", func(i *Invoice) { i.Payments[0].TermDays = 0 }, nil},
		{"payments split across methods", func(i *Invoice) {
			i.Payments = []Payment{{Method: "19", Amount: dec("100"), TermDays: 30}, {Method: "20", Amount: dec("23.50")}}
		}, nil},

		// Additional fields.
		{"16 additional fields", func(i *Invoice) {
			i.Additional = make([]AdditionalField, 16)
			for n := range i.Additional {
				i.Additional[n] = AdditionalField{Name: "Campo", Value: "valor"}
			}
		}, []string{"additional additional_fields_limit"}},
		{"15 additional fields", func(i *Invoice) {
			i.Additional = make([]AdditionalField, 15)
			for n := range i.Additional {
				i.Additional[n] = AdditionalField{Name: "Campo", Value: "valor"}
			}
		}, nil},
		{"additional field without name", func(i *Invoice) { i.Additional[0].Name = " " }, []string{"additional[0] additional_field_invalid"}},
		{"additional field without value", func(i *Invoice) { i.Additional[0].Value = "" }, []string{"additional[0] additional_field_invalid"}},
		{"additional value of 301 characters", func(i *Invoice) { i.Additional[0].Value = strings.Repeat("v", 301) }, []string{"additional[0] additional_field_invalid"}},
		{"additional value of 300 characters", func(i *Invoice) { i.Additional[0].Value = strings.Repeat("v", 300) }, nil},
		{"additional name of 301 characters", func(i *Invoice) { i.Additional[0].Name = strings.Repeat("n", 301) }, []string{"additional[0] additional_field_invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := sampleInvoice()
			tt.mutate(&inv)
			assert.Equal(t, tt.want, problems(t, inv.Validate()))
		})
	}
}

func TestInvoiceValidateReturnsEveryProblem(t *testing.T) {
	inv := sampleInvoice()
	inv.Issuer.RUC = ""
	inv.Header.Establishment = "1"
	inv.Buyer.Name = ""
	inv.Lines[0].Quantity = dec("-1")
	inv.Lines[1].TaxRate = 12
	inv.Payments[0].Method = "00"
	assert.Equal(t, []string{
		"issuer.ruc issuer_ruc_invalid",
		"header.establishment header_establishment_invalid",
		"buyer.name party_name_required",
		"lines[0].quantity line_negative_amount",
		"lines[1].tax_rate line_tax_rate_unsupported",
		"payments[0].method payment_method_invalid",
		"payments payments_total_mismatch",
	}, problems(t, inv.Validate()))
}
