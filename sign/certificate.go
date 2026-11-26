package sign

import (
	"time"

	signercrypto "github.com/nelsonmarro/go_ec_sri_invoice_signer/pkg/crypto"
)

// Certificate describes a signing certificate.
type Certificate struct {
	Subject   string
	NotBefore time.Time
	NotAfter  time.Time
}

// Inspect reads a PKCS#12 signing certificate with the same parser Sign uses,
// so a file accepted here is a file Sign can use. The error never includes
// the password.
func Inspect(p12 []byte, password string) (*Certificate, error) {
	if len(p12) == 0 {
		return nil, &spanishError{msg: msgMissingCertificate}
	}
	_, cert, err := signercrypto.ParsePKCS12(p12, password)
	if err != nil {
		return nil, &spanishError{msg: msgInspectFailed, cause: err}
	}
	return &Certificate{
		Subject:   cert.Subject.CommonName,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
	}, nil
}
