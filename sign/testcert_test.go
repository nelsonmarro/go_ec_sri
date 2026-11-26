package sign

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// testPassphrase protects the in-memory test certificate. No .p12 file is
// ever written to the repository.
const testPassphrase = "prueba"

// testCertSubject is the common name of the generated certificate.
const testCertSubject = "EMISOR DE PRUEBA"

// newTestP12 builds, in memory, a self-signed RSA 2048 certificate valid for
// one year from notBefore and returns it encoded as PKCS#12.
func newTestP12(t *testing.T, notBefore time.Time, passphrase string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: testCertSubject},
		NotBefore:    notBefore,
		NotAfter:     notBefore.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	p12, err := pkcs12.Modern.Encode(key, cert, nil, passphrase)
	require.NoError(t, err)
	return p12
}
