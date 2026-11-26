package sri

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite testdata/authorized from the goldens")

// authorizedInvoice is the authorization of the invoice golden: authorized at
// 15:30 UTC, 10:30 in Ecuador.
func authorizedInvoice(t *testing.T) Authorization {
	t.Helper()
	return Authorization{
		Status: StatusAuthorized, AccessKey: "1811202601179000000200110010010000000071234567818",
		Number:      "1811202601179000000200110010010000000071234567818",
		Date:        time.Date(2026, 11, 18, 15, 30, 0, 0, time.UTC),
		Environment: EnvironmentTest, XML: readGolden(t, "invoice_01.golden.xml"),
	}
}

func TestAuthorizedXML(t *testing.T) {
	a := authorizedInvoice(t)
	got, err := AuthorizedXML(a)
	require.NoError(t, err)
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<autorizacion>\n" +
		"  <estado>AUTORIZADO</estado>\n" +
		"  <numeroAutorizacion>1811202601179000000200110010010000000071234567818</numeroAutorizacion>\n" +
		"  <fechaAutorizacion>2026-11-18T10:30:00-05:00</fechaAutorizacion>\n" +
		"  <ambiente>PRUEBAS</ambiente>\n" +
		"  <comprobante><![CDATA[" + string(readGolden(t, "invoice_01.golden.xml")) + "]]></comprobante>\n" +
		"  <mensajes/>\n" +
		"</autorizacion>\n"
	assert.Equal(t, want, string(got))
}

func TestAuthorizedXMLVariants(t *testing.T) {
	t.Run("production", func(t *testing.T) {
		a := authorizedInvoice(t)
		a.Environment = EnvironmentProduction
		got, err := AuthorizedXML(a)
		require.NoError(t, err)
		assert.Contains(t, string(got), "\n  <ambiente>PRODUCCIÓN</ambiente>\n")
	})
	t.Run("the access key stands in for a missing number", func(t *testing.T) {
		a := authorizedInvoice(t)
		a.Number = ""
		got, err := AuthorizedXML(a)
		require.NoError(t, err)
		assert.Contains(t, string(got), "<numeroAutorizacion>1811202601179000000200110010010000000071234567818</numeroAutorizacion>")
	})
	t.Run("the number is escaped", func(t *testing.T) {
		a := authorizedInvoice(t)
		a.Number = "1<2"
		got, err := AuthorizedXML(a)
		require.NoError(t, err)
		assert.Contains(t, string(got), "<numeroAutorizacion>1&lt;2</numeroAutorizacion>")
	})
	t.Run("a CDATA end inside the receipt is split", func(t *testing.T) {
		a := authorizedInvoice(t)
		a.XML = []byte("<x>a]]>b</x>")
		got, err := AuthorizedXML(a)
		require.NoError(t, err)
		assert.Contains(t, string(got), "<comprobante><![CDATA[<x>a]]]]><![CDATA[>b</x>]]></comprobante>")
	})
	t.Run("the date is converted to Ecuador's time", func(t *testing.T) {
		a := authorizedInvoice(t)
		a.Date = time.Date(2026, 11, 19, 2, 0, 0, 0, time.UTC)
		got, err := AuthorizedXML(a)
		require.NoError(t, err)
		assert.Contains(t, string(got), "<fechaAutorizacion>2026-11-18T21:00:00-05:00</fechaAutorizacion>")
	})
}

func TestAuthorizedXMLRefuses(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Authorization)
		want   string
	}{
		"not authorized":  {func(a *Authorization) { a.Status = StatusNotAuthorized }, "solo un comprobante autorizado tiene el XML autorizado"},
		"in process":      {func(a *Authorization) { a.Status = StatusInProcess }, "solo un comprobante autorizado tiene el XML autorizado"},
		"no status":       {func(a *Authorization) { a.Status = StatusNone }, "solo un comprobante autorizado tiene el XML autorizado"},
		"no date":         {func(a *Authorization) { a.Date = time.Time{} }, "el comprobante autorizado no tiene fecha de autorización"},
		"no XML":          {func(a *Authorization) { a.XML = []byte(" \n") }, "el comprobante autorizado no tiene su XML"},
		"no number":       {func(a *Authorization) { a.Number, a.AccessKey = "", "" }, "el comprobante autorizado no tiene número de autorización"},
		"bad environment": {func(a *Authorization) { a.Environment = 0 }, "el ambiente debe ser 1 (pruebas) o 2 (producción)"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := authorizedInvoice(t)
			c.mutate(&a)
			got, err := AuthorizedXML(a)
			require.EqualError(t, err, c.want)
			assert.Nil(t, got)
		})
	}
}

// authorizedSamples are the fictitious authorized envelopes of the goldens,
// kept in testdata/authorized for the readers and the RIDE.
func authorizedSamples(t *testing.T) map[string]Authorization {
	t.Helper()
	sample := func(golden, key string, env Environment, date time.Time) Authorization {
		return Authorization{Status: StatusAuthorized, AccessKey: key, Number: key, Date: date,
			Environment: env, XML: readGolden(t, golden)}
	}
	return map[string]Authorization{
		"invoice_01.xml":                 authorizedInvoice(t),
		"credit_note_04.xml":             sample("credit_note_04.golden.xml", "1911202604179000000200110010010000000081234567817", EnvironmentTest, time.Date(2026, 11, 19, 14, 5, 0, 0, time.UTC)),
		"withholding_07.xml":             sample("withholding_07.golden.xml", "1611202607179000000200110010020000000151234567819", EnvironmentTest, time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC)),
		"withholding_07_informative.xml": sample("withholding_07_informative.golden.xml", "1611202607179000000200110010020000000151234567819", EnvironmentTest, time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC)),
	}
}

func TestAuthorizedSamples(t *testing.T) {
	for name, a := range authorizedSamples(t) {
		t.Run(name, func(t *testing.T) {
			got, err := AuthorizedXML(a)
			require.NoError(t, err)
			path := filepath.Join("testdata", "authorized", name)
			if *update {
				require.NoError(t, os.WriteFile(path, got, 0o644))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(want), string(got))
		})
	}
}
