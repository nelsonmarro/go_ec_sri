package sri

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// asJSON renders v for comparison: decimals print without trailing zeros, so
// 2.00 and 2 compare equal, and times print with their offset.
func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func readFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"testdata"}, parts...)...))
	require.NoError(t, err)
	return b
}

// invoiceOf, creditNoteOf and withholdingOf unpack a Parsed* result for the
// tests that look only at the document and its authorization.
func invoiceOf(p *ParsedInvoice, err error) (Invoice, *AuthorizationInfo, error) {
	if p == nil {
		return Invoice{}, nil, err
	}
	return p.Invoice, p.Authorization, err
}

func creditNoteOf(p *ParsedCreditNote, err error) (CreditNote, *AuthorizationInfo, error) {
	if p == nil {
		return CreditNote{}, nil, err
	}
	return p.CreditNote, p.Authorization, err
}

func withholdingOf(p *ParsedWithholding, err error) (Withholding, *AuthorizationInfo, error) {
	if p == nil {
		return Withholding{}, nil, err
	}
	return p.Withholding, p.Authorization, err
}

func ecDate(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, ecuadorTime) }

func assertInfo(t *testing.T, info *AuthorizationInfo, number string, date time.Time, env Environment) {
	t.Helper()
	require.NotNil(t, info)
	assert.Equal(t, number, info.Number)
	assert.True(t, date.Equal(info.Date), "date %v, want %v", info.Date, date)
	_, offset := info.Date.Zone()
	assert.Equal(t, -5*3600, offset, "the date is in Ecuador's time")
	assert.Equal(t, env, info.Environment)
}

// Round trip: build → AuthorizedXML → parse gives back the input. What the
// XML cannot carry is adjusted in the expected value and listed here:
//   - issue dates (and the support document's date) are calendar dates: they
//     come back at midnight in Ecuador's time;
//   - Party.Email and Party.Phone are not printed (callers add them as
//     additional fields, which come back in Additional);
//   - an empty item code comes back as GEN, a final consumer's name as
//     CONSUMIDOR FINAL and a RIMPE negocio popular as RimpeEntrepreneur (the
//     schema has one legend for both);
//   - decimals keep the scale printed in the XML (2 comes back as 2.000000),
//     equal in value.
func TestParseRoundTrip(t *testing.T) {
	auth := authorizedSamples(t)
	t.Run("invoice", func(t *testing.T) {
		in := goldenInvoice()
		p, err := ParseInvoice(readFile(t, "authorized", "invoice_01.xml"))
		require.NoError(t, err)
		want := in
		want.Header.IssueDate = ecDate(2026, 11, 18)
		assert.JSONEq(t, asJSON(t, want), asJSON(t, p.Invoice))
		assert.JSONEq(t, asJSON(t, in.Totals()), asJSON(t, p.Totals), "the printed totals are the input's")
		assertInfo(t, p.Authorization, in.Header.AccessKey, auth["invoice_01.xml"].Date, EnvironmentTest)
	})
	t.Run("credit note", func(t *testing.T) {
		in := goldenCreditNote()
		p, err := ParseCreditNote(readFile(t, "authorized", "credit_note_04.xml"))
		require.NoError(t, err)
		want := in
		want.Header.IssueDate = ecDate(2026, 11, 19)
		assert.JSONEq(t, asJSON(t, want), asJSON(t, p.CreditNote))
		assert.JSONEq(t, asJSON(t, in.Totals()), asJSON(t, p.Totals), "the printed totals are the input's")
		assertInfo(t, p.Authorization, in.Header.AccessKey, auth["credit_note_04.xml"].Date, EnvironmentTest)
	})
	for _, c := range []struct {
		file string
		in   Withholding
	}{
		{"withholding_07.xml", goldenWithholding()},
		{"withholding_07_informative.xml", goldenInformativeWithholding()},
	} {
		t.Run(c.file, func(t *testing.T) {
			got, info, err := withholdingOf(ParseWithholding(readFile(t, "authorized", c.file)))
			require.NoError(t, err)
			want := c.in
			want.Support.IssueDate = ecDate(2026, 11, 14)
			assert.JSONEq(t, asJSON(t, want), asJSON(t, got))
			assertInfo(t, info, c.in.Header.AccessKey, auth[c.file].Date, EnvironmentTest)
		})
	}
}

// Every field the builders print comes back, with the optional ones set.
func TestParseRoundTripOptionalFields(t *testing.T) {
	inv := goldenInvoice()
	inv.Header.IssueDate = ecDate(2026, 11, 18)
	inv.Issuer.SpecialTaxpayerNumber, inv.Issuer.WithholdingAgentResolution, inv.Issuer.Rimpe = "5368", "1", RimpeEntrepreneur
	inv.Buyer.Address = "Calle Sucre 123, Quito"
	inv.Lines[0].AuxCode = "AUX-1"
	inv.Payments = []Payment{{Method: "01", Amount: dec("33.00")}, {Method: "19", Amount: dec("100.00"), TermDays: 60}}
	inv.Additional = []AdditionalField{{Name: "Email", Value: "cliente@ejemplo.com"}, {Name: "Teléfono", Value: "022000000"}}
	b, err := InvoiceXML(inv)
	require.NoError(t, err)
	got, info, err := invoiceOf(ParseInvoice(b.XML))
	require.NoError(t, err)
	assert.Nil(t, info, "a bare receipt has no authorization")
	assert.JSONEq(t, asJSON(t, inv), asJSON(t, got))

	cn := goldenCreditNote()
	cn.Header.IssueDate = ecDate(2026, 11, 19)
	cn.Issuer.Rimpe = RimpeEntrepreneur
	cn.Lines[1].AuxCode = "AUX-2"
	cn.Additional = []AdditionalField{{Name: "Motivo interno", Value: "Devolución"}}
	b, err = CreditNoteXML(cn)
	require.NoError(t, err)
	gotCN, info, err := creditNoteOf(ParseCreditNote(b.XML))
	require.NoError(t, err)
	assert.Nil(t, info)
	assert.JSONEq(t, asJSON(t, cn), asJSON(t, gotCN))

	w := goldenWithholding()
	w.Support.IssueDate = ecDate(2026, 11, 14)
	w.Supplier.RelatedParty = true
	w.Issuer.SpecialTaxpayerNumber = "5368"
	w.Support.Authorization = "1411202601179000000000110010010000001231234567811"
	w.Additional = []AdditionalField{{Name: "Email", Value: "proveedor@ejemplo.com"}}
	b, err = WithholdingXML(w)
	require.NoError(t, err)
	gotW, info, err := withholdingOf(ParseWithholding(b.XML))
	require.NoError(t, err)
	assert.Nil(t, info)
	assert.JSONEq(t, asJSON(t, w), asJSON(t, gotW))
}

func TestParseAsymmetries(t *testing.T) {
	inv := goldenInvoice()
	inv.Header.IssueDate = ecDate(2026, 11, 18)
	inv.Buyer = Party{IDType: IDFinalConsumer, ID: FinalConsumerID, Name: "Juan", Email: "a@b.ec", Phone: "099"}
	inv.Lines = inv.Lines[1:]
	inv.Lines[0].Code = ""
	inv.Issuer.Rimpe = RimpePopularBusiness
	payAll(&inv)
	b, err := InvoiceXML(inv)
	require.NoError(t, err)
	got, _, err := invoiceOf(ParseInvoice(b.XML))
	require.NoError(t, err)
	assert.Equal(t, Party{IDType: IDFinalConsumer, ID: FinalConsumerID, Name: "CONSUMIDOR FINAL"}, got.Buyer, "email and phone are not printed")
	assert.Equal(t, "GEN", got.Lines[0].Code)
	assert.Equal(t, RimpeEntrepreneur, got.Issuer.Rimpe, "one legend for both regimes")
}

// The bare goldens read like their envelopes, without authorization.
func TestParseBareReceipts(t *testing.T) {
	inv, info, err := invoiceOf(ParseInvoice(readGolden(t, "invoice_01.golden.xml")))
	require.NoError(t, err)
	assert.Nil(t, info)
	assert.Equal(t, "1811202601179000000200110010010000000071234567818", inv.Header.AccessKey)
	cn, info, err := creditNoteOf(ParseCreditNote(readGolden(t, "credit_note_04.golden.xml")))
	require.NoError(t, err)
	assert.Nil(t, info)
	assert.Equal(t, "001-001-000000007", cn.ModifiedNumber)
	w, info, err := withholdingOf(ParseWithholding(readGolden(t, "withholding_07.golden.xml")))
	require.NoError(t, err)
	assert.Nil(t, info)
	assert.Equal(t, "001-001-000000123", w.Support.Number)
}

// A 07 version 1.0.0 in ISO-8859-1, without envelope: each tax names its
// support document and the document carries no totals.
func TestParseWithholdingV100Latin1(t *testing.T) {
	got, info, err := withholdingOf(ParseWithholding(readFile(t, "read", "retencion_100_latin1.xml")))
	require.NoError(t, err)
	assert.Nil(t, info)
	want := Withholding{
		Header: Header{Environment: EnvironmentProduction, Establishment: "002", EmissionPoint: "001", Sequential: "000000077",
			IssueDate: ecDate(2025, 3, 5), AccessKey: "0503202507179000000000120020010000000771234567818"},
		Issuer:       Issuer{RUC: "1790000000001", BusinessName: "COMERCIAL PEÑA CÍA. LTDA.", MainAddress: "Av. Colón 45", KeepsAccounting: true},
		Supplier:     Party{IDType: IDRUC, ID: "1790000002001", Name: "NEGOCIO DE PRUEBA S.A."},
		FiscalPeriod: ecDate(2025, 3, 1),
		Support:      SupportDocument{DocType: DocInvoice, Number: "002-001-000004567", IssueDate: ecDate(2025, 3, 3)},
		Lines: []WithholdingLine{
			{Tax: WithholdingRenta, Code: "303", Base: dec("100.00"), Percentage: dec("10"), Amount: dec("10.00")},
			{Tax: WithholdingIVA, Code: "1", Base: dec("15.00"), Percentage: dec("30"), Amount: dec("4.50")},
		},
	}
	assert.JSONEq(t, asJSON(t, want), asJSON(t, got))
}

// The same Latin-1 receipt inside a Latin-1 envelope: the envelope is decoded
// once and the receipt's own ISO-8859-1 prolog is not decoded again.
func TestParseLatin1Envelope(t *testing.T) {
	receipt := readFile(t, "read", "retencion_100_latin1.xml")
	env := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><autorizacion><estado>AUTORIZADO</estado>" +
		"<numeroAutorizacion>0503202507179000000000120020010000000771234567818</numeroAutorizacion>" +
		"<fechaAutorizacion>2025-03-05T11:00:00-05:00</fechaAutorizacion><ambiente>PRODUCCI\xd3N</ambiente><comprobante><![CDATA[")
	env = append(append(env, receipt...), []byte("]]></comprobante></autorizacion>")...)
	got, info, err := withholdingOf(ParseWithholding(env))
	require.NoError(t, err)
	assert.Equal(t, "COMERCIAL PEÑA CÍA. LTDA.", got.Issuer.BusinessName)
	assert.Equal(t, "Av. Colón 45", got.Issuer.MainAddress)
	assertInfo(t, info, "0503202507179000000000120020010000000771234567818", time.Date(2025, 3, 5, 16, 0, 0, 0, time.UTC), EnvironmentProduction)
}

// The SRI returns UTF-8 text that keeps the prolog its system wrote: valid
// UTF-8 is never decoded again by a Latin-1 label.
func TestParseUTF8DeclaredLatin1(t *testing.T) {
	doc := strings.Replace(string(readGolden(t, "withholding_07_informative.golden.xml")), `encoding="UTF-8"`, `encoding="ISO-8859-1"`, 1)
	got, _, err := withholdingOf(ParseWithholding([]byte(doc)))
	require.NoError(t, err)
	assert.Equal(t, "TIENDA DOÑA ROSA", got.Supplier.Name)
}

// A 07 version 2.0.0 another system wrote, indented, inside the envelope.
func TestParseWithholdingV200Envelope(t *testing.T) {
	got, info, err := withholdingOf(ParseWithholding(readFile(t, "read", "retencion_200_autorizacion.xml")))
	require.NoError(t, err)
	want := Withholding{
		Header: Header{Environment: EnvironmentProduction, Establishment: "001", EmissionPoint: "003", Sequential: "000000009",
			IssueDate: ecDate(2026, 11, 12), AccessKey: "1211202607179000000000120010030000000091234567815"},
		Issuer:       Issuer{RUC: "1790000000001", BusinessName: "CLIENTE & HIJOS S.A.", MainAddress: "Calle Sucre 123, Quito", KeepsAccounting: true},
		Supplier:     Party{IDType: IDRUC, ID: "1790000002001", Name: "NEGOCIO DE PRUEBA S.A."},
		FiscalPeriod: ecDate(2026, 11, 1),
		Support: SupportDocument{DocType: DocInvoice, Number: "001-002-000000123", IssueDate: ecDate(2026, 11, 10), SustentoCode: "01",
			Subtotal0: decimal.Zero, Subtotal15: dec("33.33"), IVA: dec("4.99"), Total: dec("38.32"),
			Payments: []Payment{{Method: "20", Amount: dec("38.32")}}},
		Lines: []WithholdingLine{
			{Tax: WithholdingRenta, Code: "312", Base: dec("33.33"), Percentage: dec("1.75"), Amount: dec("0.58")},
			{Tax: WithholdingIVA, Code: "2", Base: dec("4.99"), Percentage: dec("70"), Amount: dec("3.49")},
		},
	}
	assert.JSONEq(t, asJSON(t, want), asJSON(t, got))
	assertInfo(t, info, "1211202607179000000000120010030000000091234567815", time.Date(2026, 11, 12, 14, 15, 0, 0, time.UTC), EnvironmentProduction)
}

// The SRI may return the receipt as escaped text instead of CDATA.
func TestParseCreditNoteEscapedEnvelope(t *testing.T) {
	got, info, err := creditNoteOf(ParseCreditNote(readFile(t, "read", "nota_credito_escapada.xml")))
	require.NoError(t, err)
	assert.Equal(t, "CLIENTE & HIJOS S.A.", got.Buyer.Name)
	assert.Equal(t, "001-002-000000123", got.ModifiedNumber)
	assert.True(t, ecDate(2025, 3, 15).Equal(got.ModifiedDate))
	assert.Equal(t, "Devolución", got.Reason)
	assert.Empty(t, got.Lines)
	assert.Equal(t, []AdditionalField{{Name: "e-mail", Value: "cliente@ejemplo.com"}}, got.Additional)
	assertInfo(t, info, "2004202504179000000200120010020000000151234567813", time.Date(2025, 4, 20, 14, 0, 0, 0, time.UTC), EnvironmentProduction)
}

// The envelope may come nested in the authorization answer.
func TestParseNestedEnvelope(t *testing.T) {
	doc := "<RespuestaAutorizacionComprobante><claveAccesoConsultada>x</claveAccesoConsultada><autorizaciones><autorizacion>" +
		"<estado>AUTORIZADO</estado><numeroAutorizacion>1811202601179000000200110010010000000071234567818</numeroAutorizacion>" +
		"<fechaAutorizacion>2026-11-18T10:30:00.512-05:00</fechaAutorizacion><ambiente>PRUEBAS</ambiente><comprobante><![CDATA[" +
		string(readGolden(t, "invoice_01.golden.xml")) + "]]></comprobante></autorizacion></autorizaciones></RespuestaAutorizacionComprobante>"
	got, info, err := invoiceOf(ParseInvoice([]byte(doc)))
	require.NoError(t, err)
	assert.Equal(t, "CLIENTE & HIJOS S.A.", got.Buyer.Name)
	assertInfo(t, info, "1811202601179000000200110010010000000071234567818", time.Date(2026, 11, 18, 15, 30, 0, 512_000_000, time.UTC), EnvironmentTest)
}

// invoiceEnvelope wraps the invoice golden in an envelope with the given number and date.
func invoiceEnvelope(t *testing.T, number, date string) []byte {
	t.Helper()
	return []byte("<autorizacion><estado>AUTORIZADO</estado><numeroAutorizacion>" + number + "</numeroAutorizacion>" +
		"<fechaAutorizacion>" + date + "</fechaAutorizacion><comprobante><![CDATA[" +
		string(readGolden(t, "invoice_01.golden.xml")) + "]]></comprobante></autorizacion>")
}

func TestParseEnvelopeAuthorization(t *testing.T) {
	key := "1811202601179000000200110010010000000071234567818"
	t.Run("the older SRI date format, in Ecuador's time", func(t *testing.T) {
		_, info, err := invoiceOf(ParseInvoice(invoiceEnvelope(t, key, "18/11/2026 10:30:00")))
		require.NoError(t, err)
		assertInfo(t, info, key, time.Date(2026, 11, 18, 15, 30, 0, 0, time.UTC), EnvironmentTest)
	})
	t.Run("a date in another zone is converted to Ecuador's time", func(t *testing.T) {
		_, info, err := invoiceOf(ParseInvoice(invoiceEnvelope(t, key, "2026-11-18T15:30:00Z")))
		require.NoError(t, err)
		assertInfo(t, info, key, time.Date(2026, 11, 18, 15, 30, 0, 0, time.UTC), EnvironmentTest)
	})
	t.Run("an empty number falls back to the access key", func(t *testing.T) {
		_, info, err := invoiceOf(ParseInvoice(invoiceEnvelope(t, " ", "2026-11-18T10:30:00-05:00")))
		require.NoError(t, err)
		assert.Equal(t, key, info.Number)
	})
	t.Run("no date leaves it zero", func(t *testing.T) {
		_, info, err := invoiceOf(ParseInvoice(invoiceEnvelope(t, key, "")))
		require.NoError(t, err)
		assert.True(t, info.Date.IsZero())
	})
	t.Run("an unreadable date is an error", func(t *testing.T) {
		_, info, err := invoiceOf(ParseInvoice(invoiceEnvelope(t, key, "ayer")))
		require.EqualError(t, err, `no se pudo leer la factura: la fecha de autorización no es válida: "ayer"`)
		assert.Nil(t, info)
	})
}

// replaceIn returns the golden with old replaced by new, failing if old is missing.
func replaceIn(t *testing.T, golden, old, new string) []byte {
	t.Helper()
	doc := string(readGolden(t, golden))
	require.Contains(t, doc, old)
	return []byte(strings.Replace(doc, old, new, 1))
}

func TestParseErrors(t *testing.T) {
	const inv, cn, wh = "invoice_01.golden.xml", "credit_note_04.golden.xml", "withholding_07.golden.xml"
	parse := map[string]func([]byte) error{
		inv: func(b []byte) error { _, _, err := invoiceOf(ParseInvoice(b)); return err },
		cn:  func(b []byte) error { _, _, err := creditNoteOf(ParseCreditNote(b)); return err },
		wh:  func(b []byte) error { _, _, err := withholdingOf(ParseWithholding(b)); return err },
	}
	cases := []struct {
		name, parser string
		doc          func(t *testing.T) []byte
		want         string
	}{
		{"broken XML", inv, func(*testing.T) []byte { return []byte("<factura><infoTributaria>") }, "no se pudo leer la factura: el XML está mal formado"},
		{"empty", wh, func(*testing.T) []byte { return nil }, "no se pudo leer la retención: el XML está mal formado"},
		{"broken envelope", cn, func(*testing.T) []byte { return []byte("<autorizacion><comprobante>") }, "no se pudo leer la nota de crédito: el XML está mal formado"},
		{"a credit note is not an invoice", inv, func(t *testing.T) []byte { return readGolden(t, cn) }, "no se pudo leer la factura: el XML no es una factura (01)"},
		{"an invoice is not a credit note", cn, func(t *testing.T) []byte { return readGolden(t, inv) }, "no se pudo leer la nota de crédito: el XML no es una nota de crédito (04)"},
		{"an invoice is not a withholding", wh, func(t *testing.T) []byte { return readGolden(t, inv) }, "no se pudo leer la retención: el XML no es un comprobante de retención (07)"},
		{"wrong codDoc", inv, func(t *testing.T) []byte { return replaceIn(t, inv, "<codDoc>01</codDoc>", "<codDoc>04</codDoc>") }, "no se pudo leer la factura: el XML no es una factura (01)"},
		{"another root with codDoc 01", inv, func(t *testing.T) []byte {
			doc := strings.Replace(string(readGolden(t, inv)), "<factura ", "<liquidacionCompra ", 1)
			return []byte(strings.Replace(doc, "</factura>", "</liquidacionCompra>", 1))
		}, "no se pudo leer la factura: el XML no es una factura (01)"},
		{"another root with codDoc 04", cn, func(t *testing.T) []byte {
			doc := strings.Replace(string(readGolden(t, cn)), "<notaCredito ", "<notaDebito ", 1)
			return []byte(strings.Replace(doc, "</notaCredito>", "</notaDebito>", 1))
		}, "no se pudo leer la nota de crédito: el XML no es una nota de crédito (04)"},
		{"another root with codDoc 07", wh, func(t *testing.T) []byte {
			doc := strings.Replace(string(readGolden(t, wh)), "<comprobanteRetencion ", "<guiaRemision ", 1)
			return []byte(strings.Replace(doc, "</comprobanteRetencion>", "</guiaRemision>", 1))
		}, "no se pudo leer la retención: el XML no es un comprobante de retención (07)"},
		{"no infoTributaria", inv, func(*testing.T) []byte { return []byte("<factura><infoFactura/></factura>") }, "no se pudo leer la factura: el XML no es una factura (01)"},
		{"no infoFactura", inv, func(t *testing.T) []byte {
			return []byte(strings.Split(string(readGolden(t, inv)), "<infoFactura>")[0] + "</factura>")
		}, "no se pudo leer la factura: falta infoFactura"},
		{"invalid access key", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<claveAcceso>1811202601179000000200110010010000000071234567818", "<claveAcceso>1811202601179000000200110010010000000071234567810")
		}, "no se pudo leer la factura: la clave de acceso no es válida (49 dígitos con su dígito verificador)"},
		{"access key of another RUC", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<ruc>1790000002001</ruc>", "<ruc>1790000000001</ruc>")
		}, "no se pudo leer la factura: la clave de acceso no corresponde al comprobante"},
		{"bad environment", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<ambiente>1</ambiente>", "<ambiente>x</ambiente>")
		}, `no se pudo leer la factura: el ambiente no es válido: "x"`},
		{"bad issue date", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<fechaEmision>18/11/2026</fechaEmision>", "<fechaEmision>2026-11-18</fechaEmision>")
		}, `no se pudo leer la factura: la fecha no es válida: "2026-11-18"`},
		{"bad amount", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<cantidad>1.000000</cantidad>", "<cantidad>uno</cantidad>")
		}, `no se pudo leer la factura: el valor no es un número: "uno"`},
		{"bad tax rate", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<tarifa>15</tarifa><baseImponible>100.00", "<tarifa>12.5</tarifa><baseImponible>100.00")
		}, `no se pudo leer la factura: la tarifa de IVA no es válida: "12.5"`},
		{"a line without VAT", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<impuestos><impuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><tarifa>15</tarifa><baseImponible>100.00</baseImponible><valor>15.00</valor></impuesto></impuestos>", "")
		}, "no se pudo leer la factura: un ítem no tiene IVA"},
		{"bad term", inv, func(t *testing.T) []byte { return replaceIn(t, inv, "<plazo>30</plazo>", "<plazo>treinta</plazo>") }, `no se pudo leer la factura: el plazo no es válido: "treinta"`},
		{"unknown term unit", inv, func(t *testing.T) []byte {
			return replaceIn(t, inv, "<unidadTiempo>dias</unidadTiempo>", "<unidadTiempo>semanas</unidadTiempo>")
		}, `no se pudo leer la factura: la unidad del plazo no es válida: "semanas"`},
		{"credit note on another document", cn, func(t *testing.T) []byte {
			return replaceIn(t, cn, "<codDocModificado>01</codDocModificado>", "<codDocModificado>05</codDocModificado>")
		}, "no se pudo leer la nota de crédito: la nota de crédito no modifica una factura (01)"},
		{"bad modified date", cn, func(t *testing.T) []byte {
			return replaceIn(t, cn, "<fechaEmisionDocSustento>18/11/2026", "<fechaEmisionDocSustento>18-11-2026")
		}, `no se pudo leer la nota de crédito: la fecha no es válida: "18-11-2026"`},
		{"no infoNotaCredito", cn, func(t *testing.T) []byte {
			return []byte(strings.Split(string(readGolden(t, cn)), "<infoNotaCredito>")[0] + "</notaCredito>")
		}, "no se pudo leer la nota de crédito: falta infoNotaCredito"},
		{"no infoCompRetencion", wh, func(t *testing.T) []byte {
			return []byte(strings.Split(string(readGolden(t, wh)), "<infoCompRetencion>")[0] + "</comprobanteRetencion>")
		}, "no se pudo leer la retención: falta infoCompRetencion"},
		{"bad fiscal period", wh, func(t *testing.T) []byte { return replaceIn(t, wh, "<periodoFiscal>11/2026", "<periodoFiscal>2026-11") }, `no se pudo leer la retención: el período fiscal no es válido: "2026-11"`},
		{"several support documents", wh, func(t *testing.T) []byte {
			doc := string(readGolden(t, wh))
			sustento := doc[strings.Index(doc, "<docSustento>"):strings.Index(doc, "</docsSustento>")]
			return []byte(strings.Replace(doc, "</docsSustento>", sustento+"</docsSustento>", 1))
		}, "no se pudo leer la retención: la retención tiene varios documentos de sustento y solo se lee uno"},
		{"no support document", wh, func(t *testing.T) []byte {
			doc := string(readGolden(t, wh))
			return []byte(doc[:strings.Index(doc, "<docsSustento>")] + doc[strings.Index(doc, "</docsSustento>")+len("</docsSustento>"):])
		}, "no se pudo leer la retención: la retención no tiene documento de sustento"},
		{"another withheld tax", wh, func(t *testing.T) []byte {
			return replaceIn(t, wh, "<retencion><codigo>2</codigo>", "<retencion><codigo>6</codigo>")
		}, `no se pudo leer la retención: el impuesto retenido no es renta ni IVA: "6"`},
		{"support VAT of another rate", wh, func(t *testing.T) []byte {
			return replaceIn(t, wh, "<codigoPorcentaje>4</codigoPorcentaje><baseImponible>100.00</baseImponible><tarifa>15", "<codigoPorcentaje>2</codigoPorcentaje><baseImponible>100.00</baseImponible><tarifa>12")
		}, `no se pudo leer la retención: el documento de sustento tiene un impuesto que no es IVA 0 % ni 15 %: "2/2"`},
		{"support tax that is not VAT", wh, func(t *testing.T) []byte {
			return replaceIn(t, wh, "<codImpuestoDocSustento>2</codImpuestoDocSustento><codigoPorcentaje>0</codigoPorcentaje>", "<codImpuestoDocSustento>3</codImpuestoDocSustento><codigoPorcentaje>0</codigoPorcentaje>")
		}, `no se pudo leer la retención: el documento de sustento tiene un impuesto que no es IVA 0 % ni 15 %: "3/0"`},
		{"bad support amount", wh, func(t *testing.T) []byte { return replaceIn(t, wh, "<importeTotal>135.00", "<importeTotal>mucho") }, `no se pudo leer la retención: el valor no es un número: "mucho"`},
		{"bad support date", wh, func(t *testing.T) []byte {
			return replaceIn(t, wh, "<fechaEmisionDocSustento>14/11/2026", "<fechaEmisionDocSustento>14.11.2026")
		}, `no se pudo leer la retención: la fecha no es válida: "14.11.2026"`},
		{"unknown encoding", inv, func(t *testing.T) []byte {
			return append([]byte(`<?xml version="1.0" encoding="KOI8-R"?><factura>`), 0xff)
		}, "no se pudo leer la factura: el XML está mal formado"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := parse[c.parser](c.doc(t))
			require.Error(t, err)
			assert.True(t, strings.HasPrefix(err.Error(), c.want), "got %q, want prefix %q", err, c.want)
		})
	}
}

func TestParseWithholdingV100SeveralSupportDocuments(t *testing.T) {
	doc := string(readFile(t, "read", "retencion_100_latin1.xml"))
	doc = strings.Replace(doc, "<numDocSustento>002001000004567</numDocSustento>", "<numDocSustento>002001000004568</numDocSustento>", 1)
	_, _, err := withholdingOf(ParseWithholding([]byte(doc)))
	require.EqualError(t, err, "no se pudo leer la retención: la retención tiene varios documentos de sustento y solo se lee uno")
}

func TestParseTermUnitsAndRates(t *testing.T) {
	const inv = "invoice_01.golden.xml"
	months, _, err := invoiceOf(ParseInvoice(replaceIn(t, inv, "<unidadTiempo>dias</unidadTiempo>", "<unidadTiempo> Meses </unidadTiempo>")))
	require.NoError(t, err)
	assert.Equal(t, 900, months.Payments[0].TermDays, "a month counts as 30 days")
	days, _, err := invoiceOf(ParseInvoice(replaceIn(t, inv, "<unidadTiempo>dias</unidadTiempo>", "<unidadTiempo>Días</unidadTiempo>")))
	require.NoError(t, err)
	assert.Equal(t, 30, days.Payments[0].TermDays)
	rate, _, err := invoiceOf(ParseInvoice(replaceIn(t, inv, "<tarifa>15</tarifa><baseImponible>100.00", "<tarifa>15.00</tarifa><baseImponible>100.00")))
	require.NoError(t, err)
	assert.Equal(t, 15, rate.Lines[0].TaxRate, "a tarifa written with decimals")
}

func TestParseRimpeLegends(t *testing.T) {
	legend := func(t *testing.T, text string) RimpeRegime {
		t.Helper()
		doc := replaceIn(t, "invoice_01.golden.xml", "</dirMatriz>", "</dirMatriz><contribuyenteRimpe>"+text+"</contribuyenteRimpe>")
		got, _, err := invoiceOf(ParseInvoice(doc))
		require.NoError(t, err)
		return got.Issuer.Rimpe
	}
	assert.Equal(t, RimpeEntrepreneur, legend(t, "CONTRIBUYENTE RÉGIMEN RIMPE"))
	assert.Equal(t, RimpePopularBusiness, legend(t, "CONTRIBUYENTE NEGOCIO POPULAR - RÉGIMEN RIMPE"))
	assert.Equal(t, RimpePopularBusiness, legend(t, "Contribuyente Negocio Popular - Régimen RIMPE"))
	got, _, err := invoiceOf(ParseInvoice(readGolden(t, "invoice_01.golden.xml")))
	require.NoError(t, err)
	assert.Equal(t, RimpeNone, got.Issuer.Rimpe)
}

func TestParseDecimalTerm(t *testing.T) {
	const inv = "invoice_01.golden.xml"
	got, _, err := invoiceOf(ParseInvoice(replaceIn(t, inv, "<plazo>30</plazo>", "<plazo>30.00</plazo>")))
	require.NoError(t, err)
	assert.Equal(t, 30, got.Payments[0].TermDays)
	_, _, err = invoiceOf(ParseInvoice(replaceIn(t, inv, "<plazo>30</plazo>", "<plazo>30.5</plazo>")))
	require.EqualError(t, err, `no se pudo leer la factura: el plazo no es válido: "30.5"`)
}

// windows-1252 differs from ISO-8859-1 only in 0x80–0x9F (curly quotes, €).
func TestParseWindows1252(t *testing.T) {
	doc := string(readFile(t, "read", "retencion_100_latin1.xml"))
	doc = strings.Replace(doc, `encoding="ISO-8859-1"`, `encoding="windows-1252"`, 1)
	doc = strings.Replace(doc, "<dirMatriz>", "<dirMatriz>\x93Local\x94 \x80 ", 1)
	got, _, err := withholdingOf(ParseWithholding([]byte(doc)))
	require.NoError(t, err)
	assert.Equal(t, "“Local” € Av. Colón 45", got.Issuer.MainAddress)
	assert.Equal(t, "COMERCIAL PEÑA CÍA. LTDA.", got.Issuer.BusinessName)
}

// A support number another system already printed with dashes is kept.
func TestParseSupportNumberWithDashes(t *testing.T) {
	doc := strings.ReplaceAll(string(readFile(t, "read", "retencion_100_latin1.xml")), "<numDocSustento>002001000004567</numDocSustento>", "<numDocSustento>002-001-000004567</numDocSustento>")
	got, _, err := withholdingOf(ParseWithholding([]byte(doc)))
	require.NoError(t, err)
	assert.Equal(t, "002-001-000004567", got.Support.Number)
	assert.Len(t, got.Lines, 2)
}

// The totals are the ones printed in the XML, even when the lines would
// compute others (a receipt another system rounded differently).
func TestParsePrintedTotals(t *testing.T) {
	const inv, cn = "invoice_01.golden.xml", "credit_note_04.golden.xml"
	doc := replaceIn(t, inv, "<importeTotal>133.00</importeTotal>", "<importeTotal>133.01</importeTotal>")
	doc = []byte(strings.Replace(string(doc), "<precioTotalSinImpuesto>100.00</precioTotalSinImpuesto><impuestos><impuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><tarifa>15</tarifa><baseImponible>100.00</baseImponible><valor>15.00</valor>",
		"<precioTotalSinImpuesto>100.01</precioTotalSinImpuesto><impuestos><impuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><tarifa>15</tarifa><baseImponible>100.01</baseImponible><valor>14.99</valor>", 1))
	doc = []byte(strings.Replace(string(doc), "<totalDescuento>2.00</totalDescuento>", "<totalDescuento>2.10</totalDescuento>", 1))
	doc = []byte(strings.Replace(string(doc), "<baseImponible>18.00</baseImponible><tarifa>0</tarifa><valor>0.00</valor></totalImpuesto>", "<baseImponible>18.02</baseImponible><tarifa>0</tarifa><valor>0.00</valor></totalImpuesto>", 1))
	doc = []byte(strings.Replace(string(doc), "<baseImponible>100.00</baseImponible><tarifa>15</tarifa><valor>15.00</valor></totalImpuesto>", "<baseImponible>100.03</baseImponible><tarifa>15</tarifa><valor>15.04</valor></totalImpuesto>", 1))
	p, err := ParseInvoice(doc)
	require.NoError(t, err)
	want := Totals{Subtotal0: dec("18.02"), Subtotal15: dec("100.03"), Discount: dec("2.10"), IVA: dec("15.04"), Total: dec("133.01"),
		Lines: []LineTotals{{Subtotal: dec("100.01"), IVA: dec("14.99")}, {Subtotal: dec("18.00"), IVA: dec("0.00")}}}
	assert.JSONEq(t, asJSON(t, want), asJSON(t, p.Totals))
	assert.Equal(t, "133", p.Invoice.Totals().Total.String(), "the lines still compute their own")

	// The credit note prints no total discount: it is the sum of the printed line discounts.
	ncDoc := replaceIn(t, cn, "<valorModificacion>133.00</valorModificacion>", "<valorModificacion>133.02</valorModificacion>")
	ncDoc = []byte(strings.Replace(string(ncDoc), "<baseImponible>100.00</baseImponible><valor>15.00</valor></totalImpuesto>", "<baseImponible>100.00</baseImponible><valor>15.02</valor></totalImpuesto>", 1))
	ncDoc = []byte(strings.Replace(string(ncDoc), "<descuento>0.00</descuento>", "<descuento>0.50</descuento>", 1))
	pn, err := ParseCreditNote(ncDoc)
	require.NoError(t, err)
	wantNC := Totals{Subtotal0: dec("18.00"), Subtotal15: dec("100.00"), Discount: dec("2.50"), IVA: dec("15.02"), Total: dec("133.02"),
		Lines: []LineTotals{{Subtotal: dec("100.00"), IVA: dec("15.00")}, {Subtotal: dec("18.00"), IVA: dec("0.00")}}}
	assert.JSONEq(t, asJSON(t, wantNC), asJSON(t, pn.Totals))
}

func TestParseErrorReturnsNil(t *testing.T) {
	inv, err := ParseInvoice([]byte("<x/>"))
	assert.Error(t, err)
	assert.Nil(t, inv)
	cn, err := ParseCreditNote([]byte("<x/>"))
	assert.Error(t, err)
	assert.Nil(t, cn)
	w, err := ParseWithholding([]byte("<x/>"))
	assert.Error(t, err)
	assert.Nil(t, w)
}

// entry is one <autorizacion> of an SRI answer around the invoice golden.
func entry(t *testing.T, estado, date string) string {
	t.Helper()
	e := "<autorizacion>"
	if estado != "" {
		e += "<estado>" + estado + "</estado>"
	}
	return e + "<numeroAutorizacion>1811202601179000000200110010010000000071234567818</numeroAutorizacion>" +
		"<fechaAutorizacion>" + date + "</fechaAutorizacion><comprobante><![CDATA[" +
		string(readGolden(t, "invoice_01.golden.xml")) + "]]></comprobante><mensajes/></autorizacion>"
}

func TestParseEnvelopeState(t *testing.T) {
	answer := func(entries ...string) []byte {
		return []byte("<RespuestaAutorizacionComprobante><autorizaciones>" + strings.Join(entries, "") + "</autorizaciones></RespuestaAutorizacionComprobante>")
	}
	t.Run("a rejected envelope is not read as authorized", func(t *testing.T) {
		p, err := ParseInvoice([]byte(entry(t, "NO AUTORIZADO", "2026-11-18T10:00:00-05:00")))
		require.EqualError(t, err, "no se pudo leer la factura: el comprobante no está autorizado (NO AUTORIZADO)")
		assert.Nil(t, p)
	})
	t.Run("in process is not authorized either", func(t *testing.T) {
		_, err := ParseCreditNote(answer(entry(t, "EN PROCESO", "2026-11-18T10:00:00-05:00")))
		require.EqualError(t, err, "no se pudo leer la nota de crédito: el comprobante no está autorizado (EN PROCESO)")
	})
	t.Run("the authorized entry wins over a rejected one before it", func(t *testing.T) {
		p, err := ParseInvoice(answer(entry(t, "NO AUTORIZADO", "2026-11-18T10:00:00-05:00"), entry(t, " AUTORIZADO ", "2026-11-18T10:30:00-05:00")))
		require.NoError(t, err)
		assertInfo(t, p.Authorization, "1811202601179000000200110010010000000071234567818", time.Date(2026, 11, 18, 15, 30, 0, 0, time.UTC), EnvironmentTest)
	})
	t.Run("the first authorized entry is read", func(t *testing.T) {
		p, err := ParseInvoice(answer(entry(t, "AUTORIZADO", "2026-11-18T10:30:00-05:00"), entry(t, "NO AUTORIZADO", "2026-11-18T11:00:00-05:00")))
		require.NoError(t, err)
		assert.True(t, time.Date(2026, 11, 18, 15, 30, 0, 0, time.UTC).Equal(p.Authorization.Date))
	})
	t.Run("an envelope without estado is read as before", func(t *testing.T) {
		p, err := ParseInvoice([]byte(entry(t, "", "2026-11-18T10:30:00-05:00")))
		require.NoError(t, err)
		require.NotNil(t, p.Authorization)
	})
	t.Run("a rejected entry without receipt", func(t *testing.T) {
		_, err := ParseInvoice(answer("<autorizacion><estado>NO AUTORIZADO</estado><mensajes/></autorizacion>"))
		require.EqualError(t, err, "no se pudo leer la factura: el comprobante no está autorizado (NO AUTORIZADO)")
	})
	t.Run("the rejected entry's date does not leak into the authorized one", func(t *testing.T) {
		rejected := "<autorizacion><estado>NO AUTORIZADO</estado><numeroAutorizacion>X</numeroAutorizacion><fechaAutorizacion>2026-11-18T10:00:00-05:00</fechaAutorizacion><mensajes/></autorizacion>"
		authorized := "<autorizacion><estado>AUTORIZADO</estado><comprobante><![CDATA[" + string(readGolden(t, "invoice_01.golden.xml")) + "]]></comprobante></autorizacion>"
		p, err := ParseInvoice(answer(rejected, authorized))
		require.NoError(t, err)
		assert.Equal(t, "1811202601179000000200110010010000000071234567818", p.Authorization.Number)
		assert.True(t, p.Authorization.Date.IsZero())
	})
}

// Only VAT 0 % and 15 % are read (v0.1.0): any other tax, on a line or in the
// totals, is an error instead of a silent loss.
func TestParseRejectsOtherTaxes(t *testing.T) {
	const inv, cn = "invoice_01.golden.xml", "credit_note_04.golden.xml"
	line15 := "<impuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><tarifa>15</tarifa><baseImponible>100.00</baseImponible><valor>15.00</valor></impuesto>"
	total15 := "<totalImpuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><baseImponible>100.00</baseImponible><tarifa>15</tarifa><valor>15.00</valor></totalImpuesto>"
	ncTotal15 := "<totalImpuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><baseImponible>100.00</baseImponible><valor>15.00</valor></totalImpuesto>"
	cases := []struct {
		name, doc, old, new, want string
	}{
		{"ICE on a line", inv, line15, line15 + "<impuesto><codigo>3</codigo><codigoPorcentaje>3011</codigoPorcentaje><tarifa>7</tarifa><baseImponible>100.00</baseImponible><valor>7.00</valor></impuesto>",
			`no se pudo leer la factura: un ítem tiene un impuesto que no es IVA 0 % ni 15 %: "3/3011"`},
		{"IRBPNR on a line", inv, line15, line15 + "<impuesto><codigo>5</codigo><codigoPorcentaje>5001</codigoPorcentaje><tarifa>0</tarifa><baseImponible>1.00</baseImponible><valor>0.02</valor></impuesto>",
			`no se pudo leer la factura: un ítem tiene un impuesto que no es IVA 0 % ni 15 %: "5/5001"`},
		{"VAT 12 % on a line", inv, line15, strings.Replace(strings.Replace(line15, "<codigoPorcentaje>4<", "<codigoPorcentaje>2<", 1), "<tarifa>15<", "<tarifa>12<", 1),
			`no se pudo leer la factura: un ítem tiene un impuesto que no es IVA 0 % ni 15 %: "2/2"`},
		{"VAT exento on a line", inv, line15, strings.Replace(strings.Replace(line15, "<codigoPorcentaje>4<", "<codigoPorcentaje>7<", 1), "<tarifa>15<", "<tarifa>0<", 1),
			`no se pudo leer la factura: un ítem tiene un impuesto que no es IVA 0 % ni 15 %: "2/7"`},
		{"two VATs on a line", inv, line15, line15 + line15, "no se pudo leer la factura: un ítem tiene más de un IVA"},
		{"a tarifa that does not match its code", inv, line15, strings.Replace(line15, "<tarifa>15<", "<tarifa>12<", 1), `no se pudo leer la factura: la tarifa de IVA no es válida: "12"`},
		{"ICE in the totals", inv, total15, total15 + "<totalImpuesto><codigo>3</codigo><codigoPorcentaje>3011</codigoPorcentaje><baseImponible>100.00</baseImponible><valor>7.00</valor></totalImpuesto>",
			`no se pudo leer la factura: el comprobante tiene un impuesto que no es IVA 0 % ni 15 %: "3/3011"`},
		{"VAT 5 % in the totals", inv, total15, strings.Replace(total15, "<codigoPorcentaje>4<", "<codigoPorcentaje>5<", 1),
			`no se pudo leer la factura: el comprobante tiene un impuesto que no es IVA 0 % ni 15 %: "2/5"`},
		{"ICE on a credit note line", cn, line15, line15 + "<impuesto><codigo>3</codigo><codigoPorcentaje>3011</codigoPorcentaje><tarifa>7</tarifa><baseImponible>100.00</baseImponible><valor>7.00</valor></impuesto>",
			`no se pudo leer la nota de crédito: un ítem tiene un impuesto que no es IVA 0 % ni 15 %: "3/3011"`},
		{"VAT 8 % in the credit note totals", cn, ncTotal15, strings.Replace(ncTotal15, "<codigoPorcentaje>4<", "<codigoPorcentaje>8<", 1),
			`no se pudo leer la nota de crédito: el comprobante tiene un impuesto que no es IVA 0 % ni 15 %: "2/8"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := replaceIn(t, c.doc, c.old, c.new)
			var err error
			if c.doc == inv {
				_, err = ParseInvoice(doc)
			} else {
				_, err = ParseCreditNote(doc)
			}
			require.EqualError(t, err, c.want)
		})
	}
}

// In a 1.0.0 07 the support document's date is optional.
func TestParseWithholdingV100WithoutSupportDate(t *testing.T) {
	doc := strings.ReplaceAll(string(readFile(t, "read", "retencion_100_latin1.xml")), "<fechaEmisionDocSustento>03/03/2025</fechaEmisionDocSustento>", "")
	got, _, err := withholdingOf(ParseWithholding([]byte(doc)))
	require.NoError(t, err)
	assert.True(t, got.Support.IssueDate.IsZero())
	assert.Equal(t, "002-001-000004567", got.Support.Number)
	bad := strings.Replace(string(readFile(t, "read", "retencion_100_latin1.xml")), "<fechaEmisionDocSustento>03/03/2025", "<fechaEmisionDocSustento>2025-03-03", 1)
	_, err = ParseWithholding([]byte(bad))
	require.EqualError(t, err, `no se pudo leer la retención: la fecha no es válida: "2025-03-03"`)
}
