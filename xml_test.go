package sri

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The samples below are the inputs of Verith's builder tests, filled into the
// library types, with their access keys' check digits recomputed and every
// identification fictitious.

func goldenIssuer() Issuer {
	return Issuer{RUC: "1790000002001", BusinessName: "NEGOCIO DE PRUEBA S.A.", TradeName: "NEGOCIO",
		MainAddress: "Av. Amazonas N1-1", EstablishmentAddress: "Av. Amazonas N1-1", KeepsAccounting: true}
}

func goldenInvoice() Invoice {
	return Invoice{
		Header: Header{Environment: EnvironmentTest, Establishment: "001", EmissionPoint: "001", Sequential: "000000007",
			IssueDate: time.Date(2026, 11, 18, 10, 30, 0, 0, ecuadorTime),
			AccessKey: "1811202601179000000200110010010000000071234567818"},
		Issuer: goldenIssuer(),
		Buyer:  Party{IDType: IDRUC, ID: "1790000000001", Name: "CLIENTE & HIJOS S.A."},
		Lines: []Line{
			{Code: "P-001", Description: "Servicio de consultoría", Quantity: dec("1"), UnitPrice: dec("100"), Discount: dec("0"), TaxRate: 15},
			{Code: "P-002", Description: "Libro", Quantity: dec("2"), UnitPrice: dec("10.00"), Discount: dec("2.00"), TaxRate: 0},
		},
		Payments: []Payment{{Method: "20", Amount: dec("133.00"), TermDays: 30}},
	}
}

func goldenCreditNote() CreditNote {
	inv := goldenInvoice()
	return CreditNote{
		Header: Header{Environment: EnvironmentTest, Establishment: "001", EmissionPoint: "001", Sequential: "000000008",
			IssueDate: time.Date(2026, 11, 19, 9, 0, 0, 0, ecuadorTime),
			AccessKey: "1911202604179000000200110010010000000081234567817"},
		Issuer:         inv.Issuer,
		Buyer:          inv.Buyer,
		ModifiedNumber: "001-001-000000007",
		ModifiedDate:   time.Date(2026, 11, 18, 0, 0, 0, 0, ecuadorTime),
		Reason:         "Anulación de la venta",
		Lines:          inv.Lines,
	}
}

func goldenWithholding() Withholding {
	issuer := goldenIssuer()
	issuer.WithholdingAgentResolution = "1"
	return Withholding{
		Header: Header{Environment: EnvironmentTest, Establishment: "001", EmissionPoint: "002", Sequential: "000000015",
			IssueDate: time.Date(2026, 11, 16, 0, 0, 0, 0, ecuadorTime),
			AccessKey: "1611202607179000000200110010020000000151234567819"},
		Issuer:       issuer,
		Supplier:     Party{IDType: IDRUC, ID: "1790000000001", Name: "PROVEEDOR & HIJOS S.A."},
		FiscalPeriod: time.Date(2026, 11, 1, 0, 0, 0, 0, ecuadorTime),
		Support: SupportDocument{DocType: DocInvoice, Number: "001-001-000000123",
			IssueDate: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), SustentoCode: "01",
			Subtotal0: dec("20.00"), Subtotal15: dec("100.00"), IVA: dec("15.00"), Total: dec("135.00"),
			Payments: []Payment{{Method: "20", Amount: dec("135.00")}}},
		Lines: []WithholdingLine{
			{Tax: WithholdingRenta, Code: "312", Base: dec("120.00"), Percentage: dec("2.00"), Amount: dec("2.40")},
			{Tax: WithholdingIVA, Code: "1", Base: dec("15.00"), Percentage: dec("30.00"), Amount: dec("4.50")},
		},
	}
}

// goldenInformativeWithholding is the informative 07 at 0 %: a RIMPE negocio
// popular's pre-printed sales note (10-digit authorization), no VAT.
func goldenInformativeWithholding() Withholding {
	w := goldenWithholding()
	w.Supplier = Party{IDType: IDRUC, ID: "1700000001001", Name: "TIENDA DOÑA ROSA"}
	w.Support = SupportDocument{DocType: DocSalesNote, Number: "001-001-000000456", Authorization: "1234567890",
		IssueDate: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), SustentoCode: "02",
		Subtotal0: dec("120.50"), Subtotal15: decimal.Zero, IVA: decimal.Zero, Total: dec("120.50"),
		Payments: []Payment{{Method: "20", Amount: dec("120.50")}}}
	w.Lines = []WithholdingLine{{Tax: WithholdingRenta, Code: "332", Base: dec("120.50"), Percentage: dec("0.00"), Amount: dec("0.00")}}
	return w
}

// readGolden returns a golden file without its trailing newline.
func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden", name))
	require.NoError(t, err)
	return bytes.TrimSuffix(b, []byte("\n"))
}

// The goldens are Verith's (same keys and IDs as the samples above): the
// library must produce them byte for byte from the same data.
func TestXMLGoldens(t *testing.T) {
	cases := []struct {
		golden string
		build  func() (Built, error)
		key    string
	}{
		{"invoice_01.golden.xml", func() (Built, error) { return InvoiceXML(goldenInvoice()) }, "1811202601179000000200110010010000000071234567818"},
		{"credit_note_04.golden.xml", func() (Built, error) { return CreditNoteXML(goldenCreditNote()) }, "1911202604179000000200110010010000000081234567817"},
		{"withholding_07.golden.xml", func() (Built, error) { return WithholdingXML(goldenWithholding()) }, "1611202607179000000200110010020000000151234567819"},
		{"withholding_07_informative.golden.xml", func() (Built, error) { return WithholdingXML(goldenInformativeWithholding()) }, "1611202607179000000200110010020000000151234567819"},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			got, err := c.build()
			require.NoError(t, err)
			assert.Equal(t, string(readGolden(t, c.golden)), string(got.XML))
			assert.Equal(t, c.key, got.AccessKey)
		})
	}
}

func xmlOf(t *testing.T, b Built, err error) string {
	t.Helper()
	require.NoError(t, err)
	return string(b.XML)
}

func TestXMLInvalidInputReturnsValidationErrors(t *testing.T) {
	inv := goldenInvoice()
	inv.Lines = nil
	cn := goldenCreditNote()
	cn.Reason = " "
	w := goldenWithholding()
	w.Supplier.ID = "123"
	cases := map[string]struct {
		build    func() (Built, error)
		validate func() error
	}{
		"invoice":     {func() (Built, error) { return InvoiceXML(inv) }, inv.Validate},
		"credit note": {func() (Built, error) { return CreditNoteXML(cn) }, cn.Validate},
		"withholding": {func() (Built, error) { return WithholdingXML(w) }, w.Validate},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := c.build()
			var errs ValidationErrors
			require.True(t, errors.As(err, &errs), "want ValidationErrors, got %v", err)
			assert.Equal(t, c.validate(), err)
			assert.Nil(t, got.XML)
			assert.Empty(t, got.AccessKey)
		})
	}
}

// A support document with VAT but no base would print an empty
// impuestosDocSustento, which the schema rejects: Validate stops it first.
func TestWithholdingXMLRejectsVATWithoutBase(t *testing.T) {
	w := goldenWithholding()
	w.Support.Subtotal0, w.Support.Subtotal15, w.Support.IVA, w.Support.Total = dec("0"), dec("0"), dec("15"), dec("15")
	w.Support.Payments = []Payment{{Method: "20", Amount: dec("15")}}
	got, err := WithholdingXML(w)
	assert.Equal(t, []string{"support support_totals_mismatch"}, problems(t, err))
	assert.Nil(t, got.XML)
}

func TestXMLGeneratesAccessKey(t *testing.T) {
	inv := goldenInvoice()
	inv.Header.AccessKey, inv.Header.Sequential = "", "7"
	cn := goldenCreditNote()
	cn.Header.AccessKey, cn.Header.Sequential = "", "8"
	w := goldenWithholding()
	w.Header.AccessKey, w.Header.Sequential = "", "15"
	cases := []struct {
		name   string
		doc    DocType
		header Header
		build  func() (Built, error)
	}{
		{"invoice", DocInvoice, inv.Header, func() (Built, error) { return InvoiceXML(inv) }},
		{"credit note", DocCreditNote, cn.Header, func() (Built, error) { return CreditNoteXML(cn) }},
		{"withholding", DocWithholding, w.Header, func() (Built, error) { return WithholdingXML(w) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.build()
			require.NoError(t, err)
			require.True(t, ValidAccessKey(got.AccessKey), got.AccessKey)
			parts, err := ParseAccessKey(got.AccessKey)
			require.NoError(t, err)
			assert.Equal(t, c.header.IssueDate.Format("02012006"), parts.IssueDate.Format("02012006"))
			assert.Equal(t, c.doc, parts.Doc)
			assert.Equal(t, "1790000002001", parts.RUC)
			assert.Equal(t, EnvironmentTest, parts.Environment)
			assert.Equal(t, c.header.Establishment, parts.Establishment)
			assert.Equal(t, c.header.EmissionPoint, parts.EmissionPoint)
			assert.Equal(t, "00000000"[:9-len(c.header.Sequential)]+c.header.Sequential, parts.Sequential)
			assert.Equal(t, "1", parts.EmissionType)
			assert.Contains(t, string(got.XML), "<claveAcceso>"+got.AccessKey+"</claveAcceso>")
			assert.Contains(t, string(got.XML), "<secuencial>"+parts.Sequential+"</secuencial>")
		})
	}
}

func TestInvoiceXMLRules(t *testing.T) {
	build := func(t *testing.T, mutate func(*Invoice)) string {
		t.Helper()
		inv := goldenInvoice()
		mutate(&inv)
		got, err := InvoiceXML(inv)
		return xmlOf(t, got, err)
	}
	t.Run("each line prints its VAT from Totals, cent spread included", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Lines = []Line{line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15)}
			payAll(i)
		})
		assert.Equal(t, 2, strings.Count(xml, "<baseImponible>0.10</baseImponible><valor>0.02</valor>"))
		assert.Equal(t, 1, strings.Count(xml, "<baseImponible>0.10</baseImponible><valor>0.01</valor>"))
		assert.Contains(t, xml, "<baseImponible>0.30</baseImponible><tarifa>15</tarifa><valor>0.05</valor></totalImpuesto>")
		assert.Contains(t, xml, "<importeTotal>0.35</importeTotal>")
	})
	t.Run("one tax total per rate with a base, 0 % first, and the discount", func(t *testing.T) {
		xml := build(t, func(*Invoice) {})
		zero := strings.Index(xml, "<totalImpuesto><codigo>2</codigo><codigoPorcentaje>0</codigoPorcentaje><baseImponible>18.00</baseImponible><tarifa>0</tarifa><valor>0.00</valor></totalImpuesto>")
		fifteen := strings.Index(xml, "<totalImpuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><baseImponible>100.00</baseImponible><tarifa>15</tarifa><valor>15.00</valor></totalImpuesto>")
		assert.True(t, zero >= 0 && fifteen > zero, xml)
		assert.Contains(t, xml, "<totalSinImpuestos>118.00</totalSinImpuestos><totalDescuento>2.00</totalDescuento>")
		only15 := build(t, func(i *Invoice) { i.Lines = i.Lines[:1]; payAll(i) })
		assert.NotContains(t, only15, "<codigoPorcentaje>0</codigoPorcentaje>")
		only0 := build(t, func(i *Invoice) { i.Lines = i.Lines[1:]; payAll(i) })
		assert.NotContains(t, only0, "<codigoPorcentaje>4</codigoPorcentaje>")
	})
	t.Run("quantities and prices with 6 decimals, amounts with 2", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Lines[0].Quantity, i.Lines[0].UnitPrice = dec("0.333333"), dec("3.141593")
			payAll(i)
		})
		assert.Contains(t, xml, "<cantidad>0.333333</cantidad><precioUnitario>3.141593</precioUnitario><descuento>0.00</descuento><precioTotalSinImpuesto>1.05</precioTotalSinImpuesto>")
		assert.Contains(t, xml, "<baseImponible>1.05</baseImponible><valor>0.16</valor>")
		assert.Contains(t, xml, "<importeTotal>19.21</importeTotal>")
	})
	t.Run("an empty item code becomes GEN", func(t *testing.T) {
		xml := build(t, func(i *Invoice) { i.Lines[0].Code = ""; i.Lines[1].Code = "  " })
		assert.Equal(t, 2, strings.Count(xml, "<codigoPrincipal>GEN</codigoPrincipal>"))
	})
	t.Run("the aux code is printed only when given", func(t *testing.T) {
		assert.Contains(t, build(t, func(i *Invoice) { i.Lines[0].AuxCode = " AUX-1 " }), "<codigoPrincipal>P-001</codigoPrincipal><codigoAuxiliar>AUX-1</codigoAuxiliar><descripcion>")
		assert.NotContains(t, build(t, func(*Invoice) {}), "codigoAuxiliar")
	})
	t.Run("a final consumer is printed as CONSUMIDOR FINAL", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Buyer = Party{IDType: IDFinalConsumer, ID: FinalConsumerID, Name: "cliente de mostrador"}
			i.Lines = []Line{line("1", "10", "0", 15)}
			i.Payments = []Payment{{Method: "01", Amount: dec("11.50")}}
		})
		assert.Contains(t, xml, "<tipoIdentificacionComprador>07</tipoIdentificacionComprador><razonSocialComprador>CONSUMIDOR FINAL</razonSocialComprador><identificacionComprador>9999999999999</identificacionComprador>")
	})
	t.Run("special characters are escaped", func(t *testing.T) {
		xml := build(t, func(i *Invoice) { i.Buyer.Name = escapingName; i.Lines[0].Description = "Tornillo <5mm> & tuerca" })
		assert.Contains(t, xml, "<razonSocialComprador>A &amp; B &lt;C&gt; &#34;D&#34;</razonSocialComprador>")
		assert.Contains(t, xml, "<descripcion>Tornillo &lt;5mm&gt; &amp; tuerca</descripcion>")
	})
	t.Run("texts are trimmed, kept on one line and capped at 300", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Lines[0].Description = "  " + strings.Repeat("á", 350) + "  "
			i.Lines[1].Description = "Servicio\r\ncon salto"
			i.Buyer.Name = strings.Repeat("n", 400)
			i.Issuer.BusinessName = "  " + strings.Repeat("r", 320)
			i.Issuer.MainAddress = "Av.\nAmazonas"
			i.Issuer.TradeName = " NEGOCIO "
			i.Issuer.EstablishmentAddress = strings.Repeat("e", 310)
		})
		assert.Contains(t, xml, "<descripcion>"+strings.Repeat("á", 300)+"</descripcion>")
		assert.Contains(t, xml, "<descripcion>Servicio con salto</descripcion>")
		assert.Contains(t, xml, "<razonSocialComprador>"+strings.Repeat("n", 300)+"</razonSocialComprador>")
		assert.Contains(t, xml, "<razonSocial>"+strings.Repeat("r", 300)+"</razonSocial>")
		assert.Contains(t, xml, "<dirMatriz>Av. Amazonas</dirMatriz>")
		assert.Contains(t, xml, "<nombreComercial>NEGOCIO</nombreComercial>")
		assert.Contains(t, xml, "<dirEstablecimiento>"+strings.Repeat("e", 300)+"</dirEstablecimiento>")
	})
	t.Run("payments in order, the term only when positive", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Payments = []Payment{{Method: "01", Amount: dec("33.00")}, {Method: "19", Amount: dec("100"), TermDays: 60}}
		})
		assert.Contains(t, xml, "<pagos><pago><formaPago>01</formaPago><total>33.00</total></pago><pago><formaPago>19</formaPago><total>100.00</total><plazo>60</plazo><unidadTiempo>dias</unidadTiempo></pago></pagos>")
	})
	t.Run("RIMPE legend, withholding agent and special taxpayer", func(t *testing.T) {
		for _, regime := range []RimpeRegime{RimpeEntrepreneur, RimpePopularBusiness} {
			xml := build(t, func(i *Invoice) {
				i.Issuer.Rimpe, i.Issuer.WithholdingAgentResolution, i.Issuer.SpecialTaxpayerNumber = regime, "1", "123"
			})
			assert.Contains(t, xml, "<dirMatriz>Av. Amazonas N1-1</dirMatriz><agenteRetencion>1</agenteRetencion><contribuyenteRimpe>CONTRIBUYENTE RÉGIMEN RIMPE</contribuyenteRimpe></infoTributaria>")
			assert.Contains(t, xml, "<contribuyenteEspecial>123</contribuyenteEspecial><obligadoContabilidad>SI</obligadoContabilidad>")
		}
		plain := build(t, func(*Invoice) {})
		assert.NotContains(t, plain, "contribuyenteRimpe")
		assert.NotContains(t, plain, "agenteRetencion")
		assert.NotContains(t, plain, "contribuyenteEspecial")
	})
	t.Run("a business not obliged to keep accounting says NO", func(t *testing.T) {
		assert.Contains(t, build(t, func(i *Invoice) { i.Issuer.KeepsAccounting = false }), "<obligadoContabilidad>NO</obligadoContabilidad>")
	})
	t.Run("the buyer address is printed only when given; email and phone never", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Buyer.Address, i.Buyer.Email, i.Buyer.Phone = " Calle Sucre\n123 ", "cliente@ejemplo.com", "022000000"
		})
		assert.Contains(t, xml, "<identificacionComprador>1790000000001</identificacionComprador><direccionComprador>Calle Sucre 123</direccionComprador><totalSinImpuestos>")
		assert.NotContains(t, xml, "cliente@ejemplo.com")
		assert.NotContains(t, xml, "022000000")
		assert.NotContains(t, build(t, func(*Invoice) {}), "direccionComprador")
	})
	t.Run("additional fields only when present", func(t *testing.T) {
		assert.NotContains(t, build(t, func(*Invoice) {}), "infoAdicional")
		xml := build(t, func(i *Invoice) {
			i.Additional = []AdditionalField{{Name: "Correo", Value: "cliente@ejemplo.com"}, {Name: " Nota\n", Value: "A & B\n<C>"}}
		})
		assert.True(t, strings.HasSuffix(xml, `</detalles><infoAdicional><campoAdicional nombre="Correo">cliente@ejemplo.com</campoAdicional><campoAdicional nombre="Nota">A &amp; B &lt;C&gt;</campoAdicional></infoAdicional></factura>`), xml)
	})
	t.Run("the issue date is printed in its own location", func(t *testing.T) {
		xml := build(t, func(i *Invoice) {
			i.Header.IssueDate = time.Date(2026, 11, 18, 23, 30, 0, 0, ecuadorTime) // 19/11 in UTC
		})
		assert.Contains(t, xml, "<fechaEmision>18/11/2026</fechaEmision>")
	})
}

func TestCreditNoteXMLRules(t *testing.T) {
	build := func(t *testing.T, mutate func(*CreditNote)) string {
		t.Helper()
		cn := goldenCreditNote()
		mutate(&cn)
		got, err := CreditNoteXML(cn)
		return xmlOf(t, got, err)
	}
	t.Run("document type, version and modified invoice", func(t *testing.T) {
		xml := build(t, func(*CreditNote) {})
		assert.Contains(t, xml, `<notaCredito id="comprobante" version="1.1.0">`)
		assert.Contains(t, xml, "<codDoc>04</codDoc>")
		assert.Contains(t, xml, "<codDocModificado>01</codDocModificado><numDocModificado>001-001-000000007</numDocModificado><fechaEmisionDocSustento>18/11/2026</fechaEmisionDocSustento>")
	})
	t.Run("the modified date is not shifted by its time zone", func(t *testing.T) {
		xml := build(t, func(c *CreditNote) {
			c.ModifiedDate = time.Date(2026, 11, 18, 0, 0, 0, 0, time.FixedZone("far east", 13*3600))
		})
		assert.Contains(t, xml, "<fechaEmisionDocSustento>18/11/2026</fechaEmisionDocSustento>")
	})
	t.Run("the value is the total with VAT, no payments and no tarifa in the totals", func(t *testing.T) {
		xml := build(t, func(*CreditNote) {})
		assert.Contains(t, xml, "<totalSinImpuestos>118.00</totalSinImpuestos><valorModificacion>133.00</valorModificacion><moneda>DOLAR</moneda>")
		assert.Contains(t, xml, "<totalImpuesto><codigo>2</codigo><codigoPorcentaje>4</codigoPorcentaje><baseImponible>100.00</baseImponible><valor>15.00</valor></totalImpuesto>")
		assert.NotContains(t, xml, "pagos")
		assert.NotContains(t, xml, "formaPago")
	})
	t.Run("lines use the internal and additional codes, GEN when empty", func(t *testing.T) {
		xml := build(t, func(c *CreditNote) {
			c.Lines = []Line{c.Lines[0], c.Lines[1]}
			c.Lines[0].AuxCode = "AUX-1"
			c.Lines[1].Code = ""
		})
		assert.Contains(t, xml, "<codigoInterno>P-001</codigoInterno><codigoAdicional>AUX-1</codigoAdicional><descripcion>Servicio de consultoría</descripcion>")
		assert.Contains(t, xml, "<codigoInterno>GEN</codigoInterno><descripcion>Libro</descripcion>")
		assert.NotContains(t, xml, "codigoPrincipal")
	})
	t.Run("the reason is normalized; no buyer address in this schema", func(t *testing.T) {
		xml := build(t, func(c *CreditNote) {
			c.Reason = "  Línea 1\nlínea 2 "
			c.Buyer.Address = "Calle Sucre 123"
		})
		assert.Contains(t, xml, "<motivo>Línea 1 línea 2</motivo>")
		assert.NotContains(t, xml, "direccionComprador")
	})
	t.Run("additional fields only when present", func(t *testing.T) {
		assert.NotContains(t, build(t, func(*CreditNote) {}), "infoAdicional")
		xml := build(t, func(c *CreditNote) { c.Additional = []AdditionalField{{Name: "Correo", Value: "cliente@ejemplo.com"}} })
		assert.True(t, strings.HasSuffix(xml, `</detalles><infoAdicional><campoAdicional nombre="Correo">cliente@ejemplo.com</campoAdicional></infoAdicional></notaCredito>`), xml)
	})
	t.Run("a credit note whose items are all worth zero gets ValidationErrors and no XML", func(t *testing.T) {
		cn := goldenCreditNote()
		cn.Lines = []Line{line("1", "0", "0", 15)}
		got, err := CreditNoteXML(cn)
		var errs ValidationErrors
		require.True(t, errors.As(err, &errs), "want ValidationErrors, got %v", err)
		assert.Equal(t, []string{"lines total_zero"}, problems(t, err))
		assert.Nil(t, got.XML)
	})
}

func TestWithholdingXMLRules(t *testing.T) {
	build := func(t *testing.T, mutate func(*Withholding)) string {
		t.Helper()
		w := goldenWithholding()
		mutate(&w)
		got, err := WithholdingXML(w)
		return xmlOf(t, got, err)
	}
	t.Run("support document: number without hyphens, local payment, authorization only when given", func(t *testing.T) {
		xml := build(t, func(*Withholding) {})
		assert.Contains(t, xml, "<codSustento>01</codSustento><codDocSustento>01</codDocSustento><numDocSustento>001001000000123</numDocSustento><fechaEmisionDocSustento>14/11/2026</fechaEmisionDocSustento><pagoLocExt>01</pagoLocExt>")
		assert.NotContains(t, xml, "numAutDocSustento")
		assert.Contains(t, build(t, func(w *Withholding) { w.Support.Authorization = "1234567890" }),
			"</fechaEmisionDocSustento><numAutDocSustento>1234567890</numAutDocSustento><pagoLocExt>")
	})
	t.Run("the fiscal period is the month of FiscalPeriod, not of the issue date", func(t *testing.T) {
		xml := build(t, func(w *Withholding) { w.FiscalPeriod = time.Date(2026, 10, 31, 0, 0, 0, 0, ecuadorTime) })
		assert.Contains(t, xml, "<periodoFiscal>10/2026</periodoFiscal>")
		assert.Contains(t, xml, "<fechaEmision>16/11/2026</fechaEmision>")
	})
	t.Run("withholding lines: codigo by tax, percentage without trailing zeros", func(t *testing.T) {
		xml := build(t, func(w *Withholding) {
			w.Lines[0].Percentage, w.Lines[0].Amount = dec("1.75"), dec("2.10")
			w.Lines[1].Percentage, w.Lines[1].Code, w.Lines[1].Amount = dec("100.00"), "3", dec("15.00")
		})
		assert.Contains(t, xml, "<retencion><codigo>1</codigo><codigoRetencion>312</codigoRetencion><baseImponible>120.00</baseImponible><porcentajeRetener>1.75</porcentajeRetener><valorRetenido>2.10</valorRetenido></retencion>")
		assert.Contains(t, xml, "<retencion><codigo>2</codigo><codigoRetencion>3</codigoRetencion><baseImponible>15.00</baseImponible><porcentajeRetener>100</porcentajeRetener><valorRetenido>15.00</valorRetenido></retencion>")
	})
	t.Run("support taxes only for positive subtotals", func(t *testing.T) {
		xml := build(t, onlyZeroBase)
		assert.Contains(t, xml, "<impuestosDocSustento><impuestoDocSustento><codImpuestoDocSustento>2</codImpuestoDocSustento><codigoPorcentaje>0</codigoPorcentaje><baseImponible>120.00</baseImponible><tarifa>0</tarifa><valorImpuesto>0.00</valorImpuesto></impuestoDocSustento></impuestosDocSustento>")
		only15 := build(t, func(w *Withholding) {
			w.Support.Subtotal0, w.Support.Total = dec("0"), dec("115.00")
			w.Support.Payments = []Payment{{Method: "20", Amount: dec("115.00")}}
			w.Lines[0].Base, w.Lines[0].Amount = dec("100.00"), dec("2.00")
		})
		assert.NotContains(t, only15, "<codigoPorcentaje>0</codigoPorcentaje>")
	})
	t.Run("payments in order", func(t *testing.T) {
		xml := build(t, func(w *Withholding) {
			w.Support.Payments = []Payment{{Method: "01", Amount: dec("35")}, {Method: "20", Amount: dec("100.00")}}
		})
		assert.Contains(t, xml, "<pagos><pago><formaPago>01</formaPago><total>35.00</total></pago><pago><formaPago>20</formaPago><total>100.00</total></pago></pagos></docSustento>")
	})
	t.Run("supplier texts are escaped and normalized; parteRel NO by default", func(t *testing.T) {
		xml := build(t, func(w *Withholding) { w.Supplier.Name = " A & B\n<C> " })
		assert.Contains(t, xml, "<parteRel>NO</parteRel><razonSocialSujetoRetenido>A &amp; B &lt;C&gt;</razonSocialSujetoRetenido>")
	})
	t.Run("a related-party supplier prints parteRel SI", func(t *testing.T) {
		xml := build(t, func(w *Withholding) { w.Supplier.RelatedParty = true })
		assert.Contains(t, xml, "<tipoIdentificacionSujetoRetenido>04</tipoIdentificacionSujetoRetenido><parteRel>SI</parteRel><razonSocialSujetoRetenido>")
	})
	t.Run("RIMPE legend and special taxpayer", func(t *testing.T) {
		xml := build(t, func(w *Withholding) { w.Issuer.Rimpe, w.Issuer.SpecialTaxpayerNumber = RimpeEntrepreneur, "123" })
		assert.Contains(t, xml, "<agenteRetencion>1</agenteRetencion><contribuyenteRimpe>CONTRIBUYENTE RÉGIMEN RIMPE</contribuyenteRimpe>")
		assert.Contains(t, xml, "<contribuyenteEspecial>123</contribuyenteEspecial><obligadoContabilidad>SI</obligadoContabilidad>")
		assert.Contains(t, build(t, func(w *Withholding) { w.Issuer.KeepsAccounting = false }), "<obligadoContabilidad>NO</obligadoContabilidad>")
	})
	t.Run("additional fields only when present", func(t *testing.T) {
		assert.NotContains(t, build(t, func(*Withholding) {}), "infoAdicional")
		xml := build(t, func(w *Withholding) {
			w.Additional = []AdditionalField{{Name: "Correo", Value: "proveedor@ejemplo.com"}}
		})
		assert.True(t, strings.HasSuffix(xml, `</docsSustento><infoAdicional><campoAdicional nombre="Correo">proveedor@ejemplo.com</campoAdicional></infoAdicional></comprobanteRetencion>`), xml)
	})
}
