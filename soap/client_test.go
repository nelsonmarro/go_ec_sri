package soap

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nelsonmarro/go_ec_sri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "0101202601179000000200110010010000000011234567814"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}

type captured struct {
	method, contentType, path, body string
}

// serve answers every request with status and body and records the last one.
func serve(t *testing.T, status int, body []byte) (*httptest.Server, *captured) {
	t.Helper()
	got := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = captured{r.Method, r.Header.Get("Content-Type"), r.URL.Path, string(b)}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func clientFor(url string, opts ...Option) *Client {
	return New(append([]Option{WithURLs(URLs{
		ReceptionTest: url + "/rt", AuthorizationTest: url + "/at",
		ReceptionProduction: url + "/rp", AuthorizationProduction: url + "/ap",
	})}, opts...)...)
}

func TestDefaultURLs(t *testing.T) {
	assert.Equal(t, "https://celcer.sri.gob.ec/comprobantes-electronicos-ws/RecepcionComprobantesOffline", DefaultURLs().ReceptionTest)
	assert.Equal(t, "https://celcer.sri.gob.ec/comprobantes-electronicos-ws/AutorizacionComprobantesOffline", DefaultURLs().AuthorizationTest)
	assert.Equal(t, "https://cel.sri.gob.ec/comprobantes-electronicos-ws/RecepcionComprobantesOffline", DefaultURLs().ReceptionProduction)
	assert.Equal(t, "https://cel.sri.gob.ec/comprobantes-electronicos-ws/AutorizacionComprobantesOffline", DefaultURLs().AuthorizationProduction)
}

func TestSend_RequestShapeAndEnvironmentRouting(t *testing.T) {
	tests := []struct {
		env  sri.Environment
		path string
	}{{sri.EnvironmentTest, "/rt"}, {sri.EnvironmentProduction, "/rp"}}
	for _, tt := range tests {
		srv, got := serve(t, 200, fixture(t, "reception_received.xml"))
		_, err := clientFor(srv.URL).Send(context.Background(), tt.env, []byte("<factura>ñ</factura>"))
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, got.method)
		assert.Equal(t, "text/xml; charset=UTF-8", got.contentType)
		assert.Equal(t, tt.path, got.path)
		assert.Contains(t, got.body, "<xml>"+base64.StdEncoding.EncodeToString([]byte("<factura>ñ</factura>"))+"</xml>")
		assert.Contains(t, got.body, "http://ec.gob.sri.ws.recepcion")
		assert.Contains(t, got.body, "<ec:validarComprobante>")
	}
}

func TestSend_Received(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "reception_received.xml"))
	res, err := clientFor(srv.URL).Send(context.Background(), sri.EnvironmentTest, []byte("<x/>"))
	require.NoError(t, err)
	assert.Equal(t, sri.ReceptionReceived, res.Status)
	assert.Empty(t, res.Messages)
}

func TestSend_ReturnedWithTwoMessages(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "reception_returned.xml"))
	res, err := clientFor(srv.URL).Send(context.Background(), sri.EnvironmentTest, []byte("<x/>"))
	require.NoError(t, err)
	assert.Equal(t, sri.ReceptionReturned, res.Status)
	assert.Equal(t, testKey, res.AccessKey)
	require.Len(t, res.Messages, 2)
	assert.Equal(t, sri.Message{Identifier: "35", Text: "ARCHIVO NO CUMPLE ESTRUCTURA XML", AdditionalInfo: "Fallo en el elemento total", Type: sri.MessageError}, res.Messages[0])
	assert.Equal(t, sri.Message{Identifier: "43", Text: "CLAVE ACCESO REGISTRADA", Type: sri.MessageWarning}, res.Messages[1])
}

func TestAuthorize_RequestShapeAndEnvironmentRouting(t *testing.T) {
	tests := []struct {
		env  sri.Environment
		path string
	}{{sri.EnvironmentTest, "/at"}, {sri.EnvironmentProduction, "/ap"}}
	for _, tt := range tests {
		srv, got := serve(t, 200, fixture(t, "authorization_none.xml"))
		key := keyFor(t, tt.env)
		_, err := clientFor(srv.URL).Authorize(context.Background(), tt.env, key)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, got.method)
		assert.Equal(t, "text/xml; charset=UTF-8", got.contentType)
		assert.Equal(t, tt.path, got.path)
		assert.Contains(t, got.body, "<claveAccesoComprobante>"+key+"</claveAccesoComprobante>")
		assert.Contains(t, got.body, "http://ec.gob.sri.ws.autorizacion")
	}
}

func TestAuthorize_Authorized(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "authorization_authorized.xml"))
	a, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentTest, testKey)
	require.NoError(t, err)
	assert.Equal(t, sri.StatusAuthorized, a.Status)
	assert.Equal(t, testKey, a.AccessKey)
	assert.Equal(t, testKey, a.Number)
	assert.Equal(t, sri.EnvironmentTest, a.Environment)
	assert.Equal(t, time.Date(2026, 11, 18, 15, 30, 15, 123000000, time.UTC), a.Date.UTC())
	assert.Equal(t, -5*3600, offset(a.Date))
	assert.Contains(t, string(a.XML), `<razonSocial>Ñandú Pruebas</razonSocial>`)
	assert.Contains(t, string(a.XML), `<factura id="comprobante"`)
}

func offset(t time.Time) int { _, o := t.Zone(); return o }

func TestAuthorize_LegacyDateFormatIsReadInEcuadorTime(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "authorization_legacy_date.xml"))
	a, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentTest, testKey)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 18, 15, 30, 15, 0, time.UTC), a.Date.UTC())
	assert.Equal(t, "<factura/>", string(a.XML))
}

func TestAuthorize_NotAuthorizedKeepsMessages(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "authorization_not_authorized.xml"))
	a, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentTest, testKey)
	require.NoError(t, err)
	assert.Equal(t, sri.StatusNotAuthorized, a.Status)
	require.Len(t, a.Messages, 1)
	assert.Equal(t, "39", a.Messages[0].Identifier)
	assert.Equal(t, "FIRMA INVALIDA", a.Messages[0].Text)
	assert.Equal(t, sri.MessageError, a.Messages[0].Type)
}

func TestAuthorize_InProcess(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "authorization_in_process.xml"))
	a, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentTest, testKey)
	require.NoError(t, err)
	assert.Equal(t, sri.StatusInProcess, a.Status)
	assert.Equal(t, testKey, a.AccessKey)
	assert.True(t, a.Date.IsZero())
}

func TestAuthorize_ZeroAuthorizationsIsStatusNone(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "authorization_none.xml"))
	a, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentTest, testKey)
	require.NoError(t, err)
	assert.Equal(t, sri.StatusNone, a.Status)
	assert.Equal(t, testKey, a.AccessKey)
}

func TestAuthorize_AuthorizedEntryWinsAndStatusIsNormalized(t *testing.T) {
	srv, _ := serve(t, 200, fixture(t, "authorization_multiple.xml"))
	a, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentTest, testKey)
	require.NoError(t, err)
	assert.Equal(t, sri.StatusAuthorized, a.Status)
	assert.Equal(t, "<factura/>", string(a.XML))
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("connection refused is network", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		url := "http://" + l.Addr().String()
		require.NoError(t, l.Close())
		_, err = clientFor(url).Authorize(ctx, sri.EnvironmentTest, testKey)
		assertKind(t, err, KindNetwork, 0)
		assert.True(t, Retryable(err))
	})
	t.Run("slow server is timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-r.Context().Done():
			}
		}))
		defer srv.Close()
		_, err := clientFor(srv.URL, WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond})).Send(ctx, sri.EnvironmentTest, []byte("<x/>"))
		assertKind(t, err, KindTimeout, 0)
		assert.True(t, Retryable(err))
	})
	t.Run("503 is server", func(t *testing.T) {
		srv, _ := serve(t, 503, []byte("down"))
		_, err := clientFor(srv.URL).Authorize(ctx, sri.EnvironmentTest, testKey)
		assertKind(t, err, KindServer, 503)
		assert.True(t, Retryable(err))
	})
	t.Run("500 with a fault is server", func(t *testing.T) {
		srv, _ := serve(t, 500, fixture(t, "fault.xml"))
		_, err := clientFor(srv.URL).Send(ctx, sri.EnvironmentTest, []byte("<x/>"))
		assertKind(t, err, KindServer, 500)
	})
	t.Run("400 is client", func(t *testing.T) {
		srv, _ := serve(t, 400, []byte("bad"))
		_, err := clientFor(srv.URL).Send(ctx, sri.EnvironmentTest, []byte("<x/>"))
		assertKind(t, err, KindClient, 400)
		assert.False(t, Retryable(err))
	})
	t.Run("html with 200 is bad response", func(t *testing.T) {
		srv, _ := serve(t, 200, fixture(t, "maintenance.html"))
		_, err := clientFor(srv.URL).Authorize(ctx, sri.EnvironmentTest, testKey)
		assertKind(t, err, KindBadResponse, 200)
		assert.True(t, Retryable(err))
		_, err = clientFor(srv.URL).Send(ctx, sri.EnvironmentTest, []byte("<x/>"))
		assertKind(t, err, KindBadResponse, 200)
	})
	t.Run("fault with 200 is bad response", func(t *testing.T) {
		srv, _ := serve(t, 200, fixture(t, "fault.xml"))
		_, err := clientFor(srv.URL).Authorize(ctx, sri.EnvironmentTest, testKey)
		assertKind(t, err, KindBadResponse, 200)
	})
	t.Run("canceled context is not retryable", func(t *testing.T) {
		srv, _ := serve(t, 200, fixture(t, "authorization_none.xml"))
		c, cancel := context.WithCancel(ctx)
		cancel()
		_, err := clientFor(srv.URL).Authorize(c, sri.EnvironmentTest, testKey)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
		assert.False(t, Retryable(err))
	})
	t.Run("unknown environment is client error without request", func(t *testing.T) {
		srv, got := serve(t, 200, nil)
		_, err := clientFor(srv.URL).Authorize(ctx, sri.Environment(7), testKey)
		assertKind(t, err, KindClient, 0)
		assert.Empty(t, got.method)
	})
}

func assertKind(t *testing.T, err error, kind ErrorKind, status int) {
	t.Helper()
	var e *Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, kind, e.Kind)
	assert.Equal(t, status, e.StatusCode)
	assert.NotEmpty(t, e.Error())
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"network", &Error{Kind: KindNetwork}, true},
		{"timeout", &Error{Kind: KindTimeout}, true},
		{"server", &Error{Kind: KindServer, StatusCode: 502}, true},
		{"bad response", &Error{Kind: KindBadResponse}, true},
		{"client", &Error{Kind: KindClient, StatusCode: 404}, false},
		{"canceled", context.Canceled, false},
		{"wrapped canceled", &Error{Kind: KindNetwork, Err: context.Canceled}, false},
		{"foreign error", errors.New("x"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, Retryable(tt.err)) })
	}
}

func TestErrorUnwrap(t *testing.T) {
	inner := errors.New("boom")
	e := &Error{Kind: KindNetwork, Err: inner}
	assert.ErrorIs(t, e, inner)
}

func TestErrorMessageIsSpanishPerKind(t *testing.T) {
	tests := []struct {
		err  *Error
		want string
	}{
		{&Error{Kind: KindNetwork, Err: errors.New("dial tcp: refused")}, "No se pudo conectar con el SRI."},
		{&Error{Kind: KindTimeout}, "El SRI no respondió a tiempo."},
		{&Error{Kind: KindServer, StatusCode: 503}, "El SRI tuvo un error (HTTP 503)."},
		{&Error{Kind: KindBadResponse, StatusCode: 200}, "La respuesta del SRI no es la esperada."},
		{&Error{Kind: KindClient, StatusCode: 404}, "El SRI rechazó la solicitud (HTTP 404)."},
		{&Error{Kind: KindClient}, "No se pudo hacer la consulta al SRI."},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.err.Error())
	}
}

func TestAuthorize_KeyEnvironmentMismatchIsClientErrorWithoutRequest(t *testing.T) {
	srv, got := serve(t, 200, fixture(t, "authorization_none.xml"))
	require.Equal(t, sri.EnvironmentTest, mustParse(t, testKey).Environment, "fixture key must be a test-environment key")
	_, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentProduction, testKey)
	assertKind(t, err, KindClient, 0)
	assert.Empty(t, got.method)
}

func TestAuthorize_InvalidKeyStillGoesToTheSRI(t *testing.T) {
	srv, got := serve(t, 200, fixture(t, "authorization_none.xml"))
	res, err := clientFor(srv.URL).Authorize(context.Background(), sri.EnvironmentProduction, "123")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, sri.StatusNone, res.Status)
}

func mustParse(t *testing.T, key string) sri.AccessKeyParts {
	t.Helper()
	p, err := sri.ParseAccessKey(key)
	require.NoError(t, err)
	return p
}

func keyFor(t *testing.T, env sri.Environment) string {
	t.Helper()
	key, err := sri.GenerateAccessKey(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), sri.DocInvoice, "1790000002001", env, "001", "001", "000000001", bytes.NewReader([]byte("12345678")))
	require.NoError(t, err)
	return key
}
