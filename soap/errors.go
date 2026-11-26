package soap

import (
	"context"
	"errors"
	"fmt"
)

// ErrorKind classifies a failure of a call to the SRI web services.
type ErrorKind int

const (
	// KindNetwork is a connection failure (refused, reset, DNS, TLS).
	KindNetwork ErrorKind = iota + 1
	// KindTimeout is a request that ran out of time.
	KindTimeout
	// KindServer is an HTTP 5xx answer.
	KindServer
	// KindBadResponse is an answer that is not the expected SOAP message
	// (an HTML maintenance page, a SOAP fault, a broken XML).
	KindBadResponse
	// KindClient is an HTTP 4xx answer, or a request that cannot be made.
	KindClient
)

func (k ErrorKind) String() string {
	switch k {
	case KindNetwork:
		return "network"
	case KindTimeout:
		return "timeout"
	case KindServer:
		return "server"
	case KindBadResponse:
		return "bad response"
	case KindClient:
		return "client"
	}
	return "unknown"
}

// Error is the failure of a call to the SRI. StatusCode is the HTTP status
// when there was an answer, else 0.
type Error struct {
	Kind       ErrorKind
	StatusCode int
	Err        error
}

// Error returns a short Spanish description by kind; the underlying cause is
// available through Unwrap.
func (e *Error) Error() string {
	switch e.Kind {
	case KindNetwork:
		return "No se pudo conectar con el SRI."
	case KindTimeout:
		return "El SRI no respondió a tiempo."
	case KindServer:
		return fmt.Sprintf("El SRI tuvo un error (HTTP %d).", e.StatusCode)
	case KindBadResponse:
		return "La respuesta del SRI no es la esperada."
	case KindClient:
		if e.StatusCode != 0 {
			return fmt.Sprintf("El SRI rechazó la solicitud (HTTP %d).", e.StatusCode)
		}
		return "No se pudo hacer la consulta al SRI."
	}
	return "Falló la llamada al SRI."
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error { return e.Err }

// Retryable reports whether calling again later can succeed: network,
// timeout, server (5xx) and bad-response failures. Client (4xx) errors, a
// canceled context and errors that are not an *Error are not retryable. The
// library never retries by itself.
func Retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	switch e.Kind {
	case KindNetwork, KindTimeout, KindServer, KindBadResponse:
		return true
	}
	return false
}
