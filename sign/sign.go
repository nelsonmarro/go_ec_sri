// Package sign signs SRI documents with XAdES-BES from in-memory bytes and
// inspects signing certificates. It never reads files nor keeps state.
package sign

import (
	"fmt"

	sri "github.com/nelsonmarro/go_ec_sri"
	"github.com/nelsonmarro/go_ec_sri_invoice_signer/pkg/signer"
)

type signFunc func(string, []byte, *signer.SignOptions) (string, error)

// Sign signs xml (an invoice, credit note or withholding) with the PKCS#12
// certificate p12 using XAdES-BES with SHA-1, the algorithm the SRI accepts.
// Errors are in Spanish and never include the password.
func Sign(doc sri.DocType, xml []byte, p12 []byte, password string) ([]byte, error) {
	var fn signFunc
	switch doc {
	case sri.DocInvoice:
		fn = signer.SignInvoice
	case sri.DocCreditNote:
		fn = signer.SignCreditNote
	case sri.DocWithholding:
		fn = signer.SignWithholdingCertificate
	default:
		return nil, fmt.Errorf("el comprobante tipo %q no se puede firmar", string(doc))
	}
	if len(p12) == 0 {
		return nil, &spanishError{msg: msgMissingCertificate}
	}
	signed, err := fn(string(xml), p12, &signer.SignOptions{Password: password, Algorithm: signer.SHA1})
	if err != nil {
		// The library error may describe the certificate; it never includes the password.
		return nil, &spanishError{msg: msgSignFailed, cause: err}
	}
	return []byte(signed), nil
}
