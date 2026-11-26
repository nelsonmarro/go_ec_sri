package sri

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Official SRI schemas in testdata/xsd.
const (
	invoiceXSD     = "factura_V1.1.0.xsd"
	creditNoteXSD  = "NotaCredito_V1.1.0.xsd"
	withholdingXSD = "ComprobanteRetencion_V2.0.0.xsd"
)

// validateXSD checks doc against an official schema with xmllint. The
// schema is the oracle: what it rejects, the SRI rejects. The test is skipped
// only when xmllint is not installed (CI installs it).
func validateXSD(t *testing.T, schema string, doc []byte) {
	t.Helper()
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint not installed (package libxml2-utils): required in CI")
		}
		t.Skip("xmllint not installed (package libxml2-utils)")
	}
	file := filepath.Join(t.TempDir(), "doc.xml")
	require.NoError(t, os.WriteFile(file, doc, 0o600))
	out, err := exec.Command(xmllint, "--noout", "--nonet", "--schema", filepath.Join("testdata", "xsd", schema), file).CombinedOutput()
	assert.NoError(t, err, string(out))
}

// payAll replaces the invoice payments with one transfer of the whole total.
func payAll(inv *Invoice) {
	inv.Payments = []Payment{{Method: "20", Amount: inv.Totals().Total}}
}

// escapingName needs every XML escape.
const escapingName = `A & B <C> "D"`

func TestInvoiceXMLValidatesAgainstXSD(t *testing.T) {
	cases := map[string]func(*Invoice){
		"mixed (golden)": func(*Invoice) {},
		"only 15 %":      func(i *Invoice) { i.Lines = i.Lines[:1]; payAll(i) },
		"only 0 %":       func(i *Invoice) { i.Lines = i.Lines[1:]; payAll(i) },
		"discounts":      func(i *Invoice) { i.Lines[0].Discount = dec("12.35"); payAll(i) },
		"six decimals": func(i *Invoice) {
			i.Lines[0].Quantity, i.Lines[0].UnitPrice = dec("0.333333"), dec("3.141593")
			payAll(i)
		},
		"cent spread": func(i *Invoice) {
			i.Lines = []Line{line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15)}
			payAll(i)
		},
		"passport buyer": func(i *Invoice) { i.Buyer = Party{IDType: IDPassport, ID: "AB123456", Name: "JOHN SMITH"} },
		"foreign buyer":  func(i *Invoice) { i.Buyer = Party{IDType: IDForeign, ID: "X-99887766", Name: "JOHN SMITH"} },
		"cedula buyer":   func(i *Invoice) { i.Buyer = Party{IDType: IDCedula, ID: "1700000001", Name: "JUAN PÉREZ"} },
		"final consumer": func(i *Invoice) {
			i.Buyer = Party{IDType: IDFinalConsumer, ID: FinalConsumerID, Name: "CONSUMIDOR FINAL"}
			i.Lines = []Line{line("1", "40", "0", 15)}
			i.Payments = []Payment{{Method: "01", Amount: dec("46.00")}}
		},
		"multiple payments": func(i *Invoice) {
			i.Payments = []Payment{{Method: "01", Amount: dec("33.00")}, {Method: "19", Amount: dec("100.00"), TermDays: 60}}
		},
		"no term":           func(i *Invoice) { i.Payments[0].TermDays = 0 },
		"additional fields": func(i *Invoice) { i.Additional = fifteenFields() },
		"escaping":          func(i *Invoice) { i.Buyer.Name = escapingName; i.Lines[0].Description = escapingName },
		"buyer address":     func(i *Invoice) { i.Buyer.Address = "Calle Sucre 123, Quito" },
		"aux code":          func(i *Invoice) { i.Lines[0].AuxCode = "AUX-1" },
		"empty code":        func(i *Invoice) { i.Lines[1].Code = "" },
		"newline in description": func(i *Invoice) {
			i.Lines[0].Description = "Servicio\r\ncon salto"
		},
		"long texts": func(i *Invoice) {
			i.Issuer.BusinessName = strings.Repeat("r", 320)
			i.Issuer.EstablishmentAddress = strings.Repeat("e", 310)
			i.Buyer.Name = strings.Repeat("ñ", 400)
			i.Lines[0].Description = strings.Repeat("á", 350)
		},
		"rimpe, agent and special": func(i *Invoice) {
			i.Issuer.Rimpe = RimpeEntrepreneur
			i.Issuer.WithholdingAgentResolution = "1"
			i.Issuer.SpecialTaxpayerNumber = "123"
		},
		"not obliged, no trade name, no establishment address": func(i *Invoice) {
			i.Issuer.KeepsAccounting = false
			i.Issuer.TradeName, i.Issuer.EstablishmentAddress = "", ""
		},
		"generated key and short sequential": func(i *Invoice) { i.Header.AccessKey, i.Header.Sequential = "", "7" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			inv := goldenInvoice()
			mutate(&inv)
			got, err := InvoiceXML(inv)
			require.NoError(t, err)
			validateXSD(t, invoiceXSD, got.XML)
		})
	}
}

func TestCreditNoteXMLValidatesAgainstXSD(t *testing.T) {
	cases := map[string]func(*CreditNote){
		"mixed (golden)": func(*CreditNote) {},
		"only 15 %":      func(c *CreditNote) { c.Lines = c.Lines[:1] },
		"only 0 %":       func(c *CreditNote) { c.Lines = c.Lines[1:] },
		"discounts":      func(c *CreditNote) { c.Lines = []Line{line("2", "10", "5.00", 0), line("1", "100", "0.50", 15)} },
		"cedula buyer":   func(c *CreditNote) { c.Buyer = Party{IDType: IDCedula, ID: "1700000001", Name: "JUAN PÉREZ"} },
		"passport buyer": func(c *CreditNote) { c.Buyer = Party{IDType: IDPassport, ID: "AB123456", Name: "JOHN SMITH"} },
		"six decimals": func(c *CreditNote) {
			c.Lines = []Line{line("0.333333", "3.141593", "0", 15)}
		},
		"300-char reason with accents": func(c *CreditNote) { c.Reason = strings.Repeat("ñá", 150) },
		"escaping":                     func(c *CreditNote) { c.Buyer.Name, c.Reason = escapingName, escapingName },
		"additional fields":            func(c *CreditNote) { c.Additional = fifteenFields() },
		"aux code":                     func(c *CreditNote) { c.Lines[0].AuxCode = "AUX-1" },
		"not obliged, no establishment address": func(c *CreditNote) {
			c.Issuer.KeepsAccounting, c.Issuer.EstablishmentAddress = false, ""
		},
		"rimpe, agent and special": func(c *CreditNote) {
			c.Issuer.Rimpe = RimpePopularBusiness
			c.Issuer.WithholdingAgentResolution = "1"
			c.Issuer.SpecialTaxpayerNumber = "123"
		},
		"generated key": func(c *CreditNote) { c.Header.AccessKey = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cn := goldenCreditNote()
			mutate(&cn)
			got, err := CreditNoteXML(cn)
			require.NoError(t, err)
			validateXSD(t, creditNoteXSD, got.XML)
		})
	}
}

func TestWithholdingXMLValidatesAgainstXSD(t *testing.T) {
	cases := map[string]func(*Withholding){
		"sample (golden)":               func(*Withholding) {},
		"informative 332 at 0 % (02)":   func(w *Withholding) { *w = goldenInformativeWithholding() },
		"sales note support":            func(w *Withholding) { *w = goldenInformativeWithholding(); w.Support.SustentoCode = "04" },
		"rimpe and special":             func(w *Withholding) { w.Issuer.Rimpe = RimpeEntrepreneur; w.Issuer.SpecialTaxpayerNumber = "123" },
		"supplier authorization 49":     func(w *Withholding) { w.Support.Authorization = "1411202601179000000000110010010000001231234567811" },
		"fractional percentage":         func(w *Withholding) { w.Lines[0].Percentage, w.Lines[0].Amount = dec("1.75"), dec("2.10") },
		"only income tax on a 0 % base": func(w *Withholding) { onlyZeroBase(w) },
		"multiple payments": func(w *Withholding) {
			w.Support.Payments = []Payment{{Method: "01", Amount: dec("35.00")}, {Method: "20", Amount: dec("100.00")}}
		},
		"additional fields": func(w *Withholding) { w.Additional = fifteenFields() },
		"escaping":          func(w *Withholding) { w.Supplier.Name = escapingName },
		"related party":     func(w *Withholding) { w.Supplier.RelatedParty = true },
		"cedula supplier":   func(w *Withholding) { w.Supplier = Party{IDType: IDCedula, ID: "1700000001", Name: "JUAN PÉREZ"} },
		"passport supplier": func(w *Withholding) { w.Supplier = Party{IDType: IDPassport, ID: "AB123456", Name: "JOHN SMITH"} },
		"not obliged, no trade name": func(w *Withholding) {
			w.Issuer.KeepsAccounting, w.Issuer.TradeName, w.Issuer.WithholdingAgentResolution = false, "", ""
		},
		"generated key": func(w *Withholding) { w.Header.AccessKey = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			w := goldenWithholding()
			mutate(&w)
			got, err := WithholdingXML(w)
			require.NoError(t, err)
			validateXSD(t, withholdingXSD, got.XML)
		})
	}
}

// onlyZeroBase turns the sample into a purchase of 120 at 0 % with only an
// income-tax withholding.
func onlyZeroBase(w *Withholding) {
	w.Support.Subtotal0, w.Support.Subtotal15, w.Support.IVA, w.Support.Total = dec("120.00"), dec("0"), dec("0"), dec("120.00")
	w.Support.SustentoCode = "02"
	w.Support.Payments = []Payment{{Method: "20", Amount: dec("120.00")}}
	w.Lines = w.Lines[:1]
}

// fifteenFields is the most additional fields the schemas accept.
func fifteenFields() []AdditionalField {
	fields := make([]AdditionalField, 15)
	for i := range fields {
		fields[i] = AdditionalField{Name: "Campo " + string(rune('A'+i)), Value: "Valor & <" + string(rune('a'+i)) + ">"}
	}
	fields[0] = AdditionalField{Name: "Correo", Value: "cliente@ejemplo.com"}
	fields[1] = AdditionalField{Name: "Observación", Value: strings.Repeat("é", 300)}
	return fields
}
