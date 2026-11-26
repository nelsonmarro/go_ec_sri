package sri

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The catalog follows chapter 11 of the SRI ficha técnica (offline scheme,
// version 2.34) plus code 96, seen in the test environment.
func TestExplainFixIn(t *testing.T) {
	want := map[string]FixIn{
		"2": "sri", "10": "sri", "26": "document", "27": "sri", "28": "sri", "35": "support", "36": "support",
		"37": "sri", "39": "signature", "40": "signature", "42": "signature", "43": "wait", "45": "emission_point",
		"46": "issuer", "47": "support", "48": "support", "49": "support", "50": "wait", "52": "support",
		"56": "emission_point", "57": "sri", "58": "retry", "59": "counterparty", "62": "counterparty", "63": "sri",
		"64": "document", "65": "retry", "67": "retry", "68": "document", "69": "counterparty", "70": "wait",
		"96": "issuer",
	}
	assert.Len(t, catalogCodes(), len(want), "every catalog code is in this table")
	for code, fix := range want {
		e := Explain(Message{Identifier: code, Text: "TEXTO"})
		assert.Equal(t, fix, e.FixIn, "code %s", code)
		assert.Equal(t, code, e.Code)
		assert.NotEqual(t, unknownTitle, e.Title, "code %s is in the catalog", code)
	}
}

func TestExplainTextsAreNeutralSpanish(t *testing.T) {
	var all []Explanation
	for _, code := range append(catalogCodes(), "999") {
		all = append(all, Explain(Message{Identifier: code}))
		for _, doc := range []DocType{DocInvoice, DocCreditNote, DocWithholding} {
			all = append(all, ExplainFor(doc, Message{Identifier: code}))
		}
	}
	for _, e := range all {
		code := e.Code
		assert.NotEmpty(t, e.Title, code)
		assert.NotEmpty(t, e.Advice, code)
		for _, banned := range []string{"Verith", "Facturación SRI", "contáctanos", "Contáctanos", "«", "Clientes y proveedores", "Actualiza"} {
			assert.NotContains(t, e.Title+" "+e.Advice, banned, "code %s", code)
		}
		assert.True(t, strings.HasSuffix(e.Advice, "."), "code %s advice ends with a period", code)
	}
}

func TestExplainTexts(t *testing.T) {
	counterparty := "Corrige la identificación del comprador (en una factura o nota de crédito) o del proveedor (en una retención) y emite el comprobante de nuevo."
	for _, code := range []string{"59", "62", "69"} {
		assert.Equal(t, counterparty, Explain(Message{Identifier: code}).Advice, code)
	}
	e := Explain(Message{Identifier: "45"})
	assert.Equal(t, "Ese número de comprobante ya fue usado", e.Title)
	assert.Equal(t, "Emite el comprobante con el siguiente secuencial libre del punto de emisión: saldrá con un número y una clave de acceso nuevos.", e.Advice)
}

func TestExplainOriginal(t *testing.T) {
	e := Explain(Message{Identifier: "45", Text: "SECUENCIAL REGISTRADO", AdditionalInfo: "El secuencial ya fue usado", Type: MessageError})
	assert.Equal(t, "[45] SECUENCIAL REGISTRADO (El secuencial ya fue usado)", e.Original)
	e = Explain(Message{Identifier: "70", Text: "CLAVE DE ACCESO EN PROCESAMIENTO"})
	assert.Equal(t, "[70] CLAVE DE ACCESO EN PROCESAMIENTO", e.Original)
	e = Explain(Message{Identifier: " 43 ", Text: " CLAVE ACCESO REGISTRADA ", AdditionalInfo: "  "})
	assert.Equal(t, "[43] CLAVE ACCESO REGISTRADA", e.Original, "spaces are trimmed")
	assert.Equal(t, "43", e.Code)
	assert.Equal(t, FixWait, e.FixIn)
	assert.Equal(t, "[45]", Explain(Message{Identifier: "45"}).Original, "no trailing space without text")
	assert.Equal(t, "ALGO", Explain(Message{Text: " ALGO "}).Original, "no brackets without identifier")
	assert.Equal(t, "ALGO (MÁS)", Explain(Message{Text: "ALGO", AdditionalInfo: "MÁS"}).Original)
	assert.Equal(t, "[45] (MÁS)", Explain(Message{Identifier: "45", AdditionalInfo: "MÁS"}).Original)
	assert.Equal(t, "", Explain(Message{}).Original)
}

func TestExplainUnknown(t *testing.T) {
	e := Explain(Message{Identifier: "999", Text: "ALGO NUEVO"})
	assert.Equal(t, Explanation{Code: "999", Title: "El SRI rechazó el comprobante",
		Advice: "Lee el mensaje original del SRI para saber qué corregir y emite el comprobante de nuevo.",
		FixIn:  FixSupport, Original: "[999] ALGO NUEVO"}, e)
	assert.Equal(t, FixSupport, Explain(Message{}).FixIn)
}

// ExplainFor names the counterparty of the document: the buyer of an invoice
// or credit note, the supplier of a withholding.
func TestExplainFor(t *testing.T) {
	cases := []struct {
		doc           DocType
		code          string
		fix           FixIn
		title, advice string
	}{
		{DocInvoice, "59", "buyer", "La identificación del comprador no es válida para el SRI", "Corrige la identificación del comprador y emite la factura de nuevo."},
		{DocInvoice, "62", "buyer", "La identificación del comprador no es válida para el SRI", "Corrige la identificación del comprador y emite la factura de nuevo."},
		{DocInvoice, "69", "buyer", "La identificación del comprador no es válida para el SRI", "Corrige la identificación del comprador y emite la factura de nuevo."},
		{DocInvoice, "64", "support", "El SRI no reconoce el documento relacionado", "Es un error del sistema que genera el comprobante: repórtalo a quien le da soporte, con el mensaje original del SRI."},
		{DocInvoice, "68", "support", "El SRI no reconoce el documento relacionado", "Es un error del sistema que genera el comprobante: repórtalo a quien le da soporte, con el mensaje original del SRI."},
		{DocCreditNote, "59", "buyer", "La identificación del comprador no es válida para el SRI", "Corrige la identificación del comprador y emite la nota de crédito de nuevo. Si la identificación está mal en la factura que modifica, anula esa factura en el portal del SRI."},
		{DocCreditNote, "62", "buyer", "La identificación del comprador no es válida para el SRI", "Corrige la identificación del comprador y emite la nota de crédito de nuevo. Si la identificación está mal en la factura que modifica, anula esa factura en el portal del SRI."},
		{DocCreditNote, "69", "buyer", "La identificación del comprador no es válida para el SRI", "Corrige la identificación del comprador y emite la nota de crédito de nuevo. Si la identificación está mal en la factura que modifica, anula esa factura en el portal del SRI."},
		{DocCreditNote, "64", "document", "El SRI no reconoce la factura que modifica la nota de crédito", "Revisa el número y la fecha de la factura que modifica y emite la nota de crédito de nuevo."},
		{DocCreditNote, "68", "document", "El SRI no reconoce la factura que modifica la nota de crédito", "Revisa el número y la fecha de la factura que modifica y emite la nota de crédito de nuevo."},
		{DocWithholding, "59", "supplier", "El SRI no encontró la identificación del proveedor", "El RUC del proveedor no es válido para el SRI. Corrige el proveedor y emite el comprobante de nuevo."},
		{DocWithholding, "62", "supplier", "La identificación del proveedor es incorrecta", "El RUC del proveedor no es válido para el SRI. Corrige el proveedor y emite el comprobante de nuevo."},
		{DocWithholding, "69", "supplier", "El RUC del proveedor no existe en el SRI", "El RUC del proveedor no es válido para el SRI. Corrige el proveedor y emite el comprobante de nuevo."},
		{DocWithholding, "64", "document", "El SRI no reconoce el tipo del documento del proveedor", "Revisa el tipo, el número y la fecha del documento del proveedor (documento de sustento) y emite la retención de nuevo."},
		{DocWithholding, "68", "document", "El SRI no encontró el documento del proveedor", "Revisa el número y la fecha del documento del proveedor (documento de sustento) y emite la retención de nuevo."},
	}
	for _, c := range cases {
		t.Run(string(c.doc)+"/"+c.code, func(t *testing.T) {
			m := Message{Identifier: c.code, Text: "TEXTO", AdditionalInfo: "INFO"}
			e := ExplainFor(c.doc, m)
			assert.Equal(t, Explanation{Code: c.code, Title: c.title, Advice: c.advice, FixIn: c.fix, Original: "[" + c.code + "] TEXTO (INFO)"}, e)
		})
	}
	t.Run("the other codes and document types are Explain's", func(t *testing.T) {
		for _, code := range append(catalogCodes(), "999") {
			m := Message{Identifier: code, Text: "TEXTO"}
			assert.Equal(t, Explain(m), ExplainFor(DocSalesNote, m), code)
			assert.Equal(t, Explain(m), ExplainFor("", m), code)
		}
		for _, doc := range []DocType{DocInvoice, DocCreditNote, DocWithholding} {
			m := Message{Identifier: "45", Text: "SECUENCIAL REGISTRADO"}
			assert.Equal(t, Explain(m), ExplainFor(doc, m))
		}
	})
	t.Run("Explain names both counterparties", func(t *testing.T) {
		assert.Equal(t, "El SRI no encontró la identificación del comprador o del proveedor", Explain(Message{Identifier: "59"}).Title)
		assert.Equal(t, "La identificación del comprador o del proveedor es incorrecta", Explain(Message{Identifier: "62"}).Title)
		assert.Equal(t, "El RUC del comprador o del proveedor no existe en el SRI", Explain(Message{Identifier: "69"}).Title)
	})
}

// catalogCodes lists the identifiers the catalog explains, sorted.
func catalogCodes() []string {
	out := make([]string, 0, len(rejectionCatalog))
	for code := range rejectionCatalog {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}
