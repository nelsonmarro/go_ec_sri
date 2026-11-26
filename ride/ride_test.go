package ride

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sri "github.com/nelsonmarro/go_ec_sri"
)

type renderFunc func(w *bytes.Buffer, xml []byte, o Options) error

var renderers = map[string]struct {
	file   string
	bare   string
	render renderFunc
}{
	"invoice":                 {"invoice_01.xml", "invoice_01.golden.xml", func(w *bytes.Buffer, x []byte, o Options) error { return Invoice(w, x, o) }},
	"credit note":             {"credit_note_04.xml", "credit_note_04.golden.xml", func(w *bytes.Buffer, x []byte, o Options) error { return CreditNote(w, x, o) }},
	"withholding":             {"withholding_07.xml", "withholding_07.golden.xml", func(w *bytes.Buffer, x []byte, o Options) error { return Withholding(w, x, o) }},
	"informative withholding": {"withholding_07_informative.xml", "withholding_07_informative.golden.xml", func(w *bytes.Buffer, x []byte, o Options) error { return Withholding(w, x, o) }},
}

func testdata(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", dir, name))
	require.NoError(t, err)
	return b
}

func logoPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

func logoJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 20)), nil))
	return b.Bytes()
}

func TestRendersPDF(t *testing.T) {
	for name, r := range renderers {
		for logoName, logo := range map[string][]byte{"no logo": nil, "png": logoPNG(t), "jpeg": logoJPEG(t)} {
			t.Run(name+"/"+logoName, func(t *testing.T) {
				var out bytes.Buffer
				require.NoError(t, r.render(&out, testdata(t, "authorized", r.file), Options{Logo: logo}))
				assert.True(t, bytes.HasPrefix(out.Bytes(), []byte("%PDF")))
				assert.Greater(t, out.Len(), 2000)
			})
		}
	}
}

func TestLogoChangesDocument(t *testing.T) {
	x := testdata(t, "authorized", "invoice_01.xml")
	var without, with bytes.Buffer
	require.NoError(t, Invoice(&without, x, Options{}))
	require.NoError(t, Invoice(&with, x, Options{Logo: logoPNG(t)}))
	assert.Greater(t, with.Len(), without.Len())
}

func TestBareReceiptIsRefused(t *testing.T) {
	for name, r := range renderers {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := r.render(&out, testdata(t, "golden", r.bare), Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "autorizado")
			assert.Zero(t, out.Len(), "nothing is written")
		})
	}
}

func TestInvalidLogoIsAnError(t *testing.T) {
	for name, r := range renderers {
		for logoName, logo := range map[string][]byte{
			"garbage":   []byte("esto no es una imagen"),
			"truncated": logoPNG(t)[:30],
		} {
			t.Run(name+"/"+logoName, func(t *testing.T) {
				var out bytes.Buffer
				err := r.render(&out, testdata(t, "authorized", r.file), Options{Logo: logo})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "logo")
				assert.Zero(t, out.Len())
			})
		}
	}
}

func TestWrongDocumentType(t *testing.T) {
	var out bytes.Buffer
	assert.Error(t, Invoice(&out, testdata(t, "authorized", "credit_note_04.xml"), Options{}))
	assert.Error(t, CreditNote(&out, testdata(t, "authorized", "invoice_01.xml"), Options{}))
	assert.Error(t, Withholding(&out, testdata(t, "authorized", "invoice_01.xml"), Options{}))
	assert.Error(t, Invoice(&out, []byte("no es xml"), Options{}))
	assert.Zero(t, out.Len())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disco lleno") }

func TestWriterErrorIsReturned(t *testing.T) {
	err := Invoice(failingWriter{}, testdata(t, "authorized", "invoice_01.xml"), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disco lleno")
}

var textRun = regexp.MustCompile(`\((.*)\) Tj`)

// pdfTexts returns the text runs of a PDF in drawing order. maroto writes the
// content stream uncompressed, in Windows-1252.
func pdfTexts(t *testing.T, pdf []byte) []string {
	t.Helper()
	var out []string
	for _, m := range textRun.FindAllSubmatch(pdf, -1) {
		var r []rune
		for _, b := range m[1] {
			r = append(r, rune(b)) // Windows-1252 letters used here equal their Latin-1 code points
		}
		out = append(out, strings.ReplaceAll(string(r), `\`, ""))
	}
	return out
}

func render1(t *testing.T, f renderFunc, xml []byte) []string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, f(&out, xml, Options{}))
	return pdfTexts(t, out.Bytes())
}

func TestInvoicePrintsTheAuthorizedData(t *testing.T) {
	texts := render1(t, renderers["invoice"].render, testdata(t, "authorized", "invoice_01.xml"))
	for _, want := range []string{
		"No. 001-001-000000007", "1811202601179000000200110010010000000071234567818", "18/11/2026 10:30:00",
		"AMBIENTE: PRUEBAS", "R.U.C.: 1790000002001", "Identificación: 1790000000001", "Fecha Emisión: 18/11/2026",
		"Subtotal 15 %", "IVA 15 %", "133.00", "15.00", "P-002", "2.00", "10.00", "18.00",
		"Transferencia u otros con el sistema financiero  133.00  (plazo: 30 días)",
	} {
		assert.Contains(t, texts, want)
	}
}

// The RIDE prints what the authorized XML says, not a recomputation.
func TestInvoicePrintsThePrintedTotals(t *testing.T) {
	x := bytes.Replace(testdata(t, "authorized", "invoice_01.xml"), []byte("<importeTotal>133.00</importeTotal>"), []byte("<importeTotal>777.77</importeTotal>"), 1)
	x = bytes.Replace(x, []byte("<valor>15.00</valor>"), []byte("<valor>14.99</valor>"), 1)
	texts := render1(t, renderers["invoice"].render, x)
	assert.Contains(t, texts, "777.77")
	assert.Contains(t, texts, "14.99")
}

func TestCreditNotePrintsTheModifiedInvoice(t *testing.T) {
	texts := render1(t, renderers["credit note"].render, testdata(t, "authorized", "credit_note_04.xml"))
	for _, want := range []string{
		"NOTA DE CRÉDITO", "FECHA Y HORA DE AUTORIZACIÓN", "19/11/2026 09:05:00", "Comprobante que se modifica",
		"Razón de modificación: Anulación de la venta", "VALOR TOTAL", "133.00",
	} {
		assert.Contains(t, texts, want)
	}
	var modified string
	for _, s := range texts {
		if strings.HasPrefix(s, "FACTURA ") {
			modified = s
		}
	}
	assert.Regexp(t, `^FACTURA \d{3}-\d{3}-\d{9}$`, modified)
}

func TestWithholdingPrintsLinesAndTotal(t *testing.T) {
	texts := render1(t, renderers["withholding"].render, testdata(t, "authorized", "withholding_07.xml"))
	for _, want := range []string{
		"COMPROBANTE DE RETENCIÓN", "TOTAL RETENIDO", "6.90", "2.40", "4.50", "RENTA", "IVA", "FACTURA", "Período Fiscal: 11/2026", "001-001-000000123", "2.00 %", "30.00 %", "312",
	} {
		assert.Contains(t, texts, want)
	}
}

func TestInformativeWithholdingTotalsZero(t *testing.T) {
	texts := render1(t, renderers["informative withholding"].render, testdata(t, "authorized", "withholding_07_informative.xml"))
	assert.Contains(t, texts, "TOTAL RETENIDO")
	assert.Contains(t, texts, "0.00")
}

func TestQuantityFormat(t *testing.T) {
	for in, want := range map[string]string{"1.000000": "1.00", "2.5": "2.50", "0.333333": "0.333333", "10.120000": "10.12", "7": "7.00", "100.00": "100.00"} {
		assert.Equal(t, want, quantity(decimal.RequireFromString(in)), in)
	}
}

func TestWrapText(t *testing.T) {
	assert.Equal(t, []string{""}, wrapText("", 10))
	assert.Equal(t, []string{"uno dos", "tres"}, wrapText("uno dos tres", 8))
	assert.Equal(t, []string{"abcd", "efgh", "ij"}, wrapText("abcdefghij", 4))
	assert.Equal(t, []string{"x", "abcd", "efgh"}, wrapText("x abcdefgh", 4))
}

func TestEnvironmentAndRimpe(t *testing.T) {
	assert.Equal(t, "PRODUCCIÓN", environmentName(sri.EnvironmentProduction))
	assert.Equal(t, "PRUEBAS", environmentName(sri.EnvironmentTest))
	assert.Equal(t, "PRUEBAS", environmentName(0), "a malformed code never claims production")
	assert.Equal(t, "", rimpeLegend(sri.RimpeNone))
	assert.Equal(t, "CONTRIBUYENTE RÉGIMEN RIMPE", rimpeLegend(sri.RimpePopularBusiness))
}

// The totals block prints each amount next to its label, in order.
func TestInvoiceTotalsOrder(t *testing.T) {
	texts := render1(t, renderers["invoice"].render, testdata(t, "authorized", "invoice_01.xml"))
	i := slices.Index(texts, "Subtotal 15 %")
	require.GreaterOrEqual(t, i, 0)
	assert.Equal(t, []string{"Subtotal 15 %", "Subtotal 0 %", "Descuento", "IVA 15 %", "VALOR TOTAL", "100.00", "18.00", "2.00", "15.00", "133.00"}, texts[i:i+10])
}

func TestMissingAuthorizationDateIsRefused(t *testing.T) {
	re := regexp.MustCompile(`<fechaAutorizacion>[^<]*</fechaAutorizacion>`)
	for name, r := range renderers {
		t.Run(name, func(t *testing.T) {
			x := testdata(t, "authorized", r.file)
			require.True(t, re.Match(x), "fixture must carry the date")
			var out bytes.Buffer
			err := r.render(&out, re.ReplaceAll(x, nil), Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "fecha de autorización")
			assert.Zero(t, out.Len(), "nothing is written")
		})
	}
}
