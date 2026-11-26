package sign

// spanishError prints only its Spanish message; the library cause stays
// reachable through Unwrap (errors.Is/As) but is never printed, so English
// library text and anything it might echo never reach the user.
type spanishError struct {
	msg   string
	cause error
}

func (e *spanishError) Error() string { return e.msg }
func (e *spanishError) Unwrap() error { return e.cause }

const (
	msgMissingCertificate = "falta el certificado de firma electrónica"
	msgSignFailed         = "no se pudo firmar el comprobante: revisa el certificado de firma electrónica y su clave"
	msgInspectFailed      = "no se pudo leer el certificado de firma electrónica: revisa el archivo y su clave"
)
