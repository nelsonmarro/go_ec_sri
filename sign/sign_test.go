package sign

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sri "github.com/nelsonmarro/go_ec_sri"
	"github.com/nelsonmarro/go_ec_sri_invoice_signer/pkg/signer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var goldens = []struct {
	name   string
	doc    sri.DocType
	file   string
	schema string
}{
	{"invoice", sri.DocInvoice, "invoice_01.golden.xml", "factura_V1.1.0.xsd"},
	{"credit note", sri.DocCreditNote, "credit_note_04.golden.xml", "NotaCredito_V1.1.0.xsd"},
	{"withholding", sri.DocWithholding, "withholding_07.golden.xml", "ComprobanteRetencion_V2.0.0.xsd"},
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "golden", name))
	require.NoError(t, err)
	return b
}

// validateXSD checks doc against an official schema with xmllint.
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
	out, err := exec.Command(xmllint, "--noout", "--nonet", "--schema", filepath.Join("..", "testdata", "xsd", schema), file).CombinedOutput()
	assert.NoError(t, err, string(out))
}

func TestSignProducesVerifiableValidDocuments(t *testing.T) {
	p12 := newTestP12(t, time.Now().Add(-time.Hour), testPassphrase)
	for _, g := range goldens {
		t.Run(g.name, func(t *testing.T) {
			signed, err := Sign(g.doc, readGolden(t, g.file), p12, testPassphrase)
			require.NoError(t, err)
			assert.Contains(t, string(signed), "<ds:Signature")
			assert.Contains(t, string(signed), "http://www.w3.org/2000/09/xmldsig#rsa-sha1")

			require.NoError(t, signer.Verify(string(signed)))

			validateXSD(t, g.schema, signed)
		})
	}
}

func TestSignRejectsUnsupportedDocType(t *testing.T) {
	p12 := newTestP12(t, time.Now(), testPassphrase)
	for _, doc := range []sri.DocType{sri.DocSalesNote, "99", ""} {
		_, err := Sign(doc, readGolden(t, "invoice_01.golden.xml"), p12, testPassphrase)
		require.Error(t, err)
		assert.Equal(t, "el comprobante tipo \""+string(doc)+"\" no se puede firmar", err.Error())
	}
}

func TestSignWrongPasswordNeverLeaksIt(t *testing.T) {
	p12 := newTestP12(t, time.Now(), testPassphrase)
	const wrong = "clave-equivocada"
	_, err := Sign(sri.DocInvoice, readGolden(t, "invoice_01.golden.xml"), p12, wrong)
	require.Error(t, err)
	assert.Equal(t, wantSignFailed, err.Error())
	assert.NotContains(t, err.Error(), wrong)
	assert.NotContains(t, err.Error(), testPassphrase)
	assert.NotNil(t, errors.Unwrap(err), "the library cause stays reachable")
	assert.NotContains(t, fmt.Sprintf("%v %+v", err, err), wrong)
}

func TestSignEmptyCertificate(t *testing.T) {
	_, err := Sign(sri.DocInvoice, readGolden(t, "invoice_01.golden.xml"), nil, testPassphrase)
	require.Error(t, err)
	assert.Equal(t, "falta el certificado de firma electrónica", err.Error())
}

func TestSignGarbageCertificate(t *testing.T) {
	_, err := Sign(sri.DocInvoice, readGolden(t, "invoice_01.golden.xml"), []byte("no es un p12"), testPassphrase)
	require.Error(t, err)
	assert.Equal(t, wantSignFailed, err.Error())
	assert.NotContains(t, err.Error(), testPassphrase)
}

func TestInspectReturnsSubjectAndValidity(t *testing.T) {
	notBefore := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	c, err := Inspect(newTestP12(t, notBefore, testPassphrase), testPassphrase)
	require.NoError(t, err)
	assert.Equal(t, "EMISOR DE PRUEBA", c.Subject)
	assert.True(t, c.NotBefore.Equal(notBefore), c.NotBefore)
	assert.True(t, c.NotAfter.Equal(time.Date(2027, 3, 1, 10, 0, 0, 0, time.UTC)), c.NotAfter)
}

func TestInspectErrors(t *testing.T) {
	good := newTestP12(t, time.Now(), testPassphrase)
	c, err := Inspect(good, "otra-clave")
	require.Error(t, err)
	assert.Nil(t, c)
	assert.Equal(t, wantInspectFailed, err.Error())
	assert.NotContains(t, err.Error(), "otra-clave")
	assert.NotContains(t, err.Error(), testPassphrase)

	c, err = Inspect([]byte("basura"), testPassphrase)
	require.Error(t, err)
	assert.Nil(t, c)
	assert.Equal(t, wantInspectFailed, err.Error())

	c, err = Inspect(nil, testPassphrase)
	require.Error(t, err)
	assert.Nil(t, c)
	assert.Equal(t, "falta el certificado de firma electrónica", err.Error())
}

// Literal messages: they are user-facing and must not drift.
const (
	wantSignFailed    = "no se pudo firmar el comprobante: revisa el certificado de firma electrónica y su clave"
	wantInspectFailed = "no se pudo leer el certificado de firma electrónica: revisa el archivo y su clave"
)
