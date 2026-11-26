// Package soap is the client of the SRI's reception and authorization web
// services. It makes one call per method and never retries or polls: the
// caller decides when to ask again (see Retryable).
package soap

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nelsonmarro/go_ec_sri"
)

// URLs are the four endpoints of the SRI offline scheme.
type URLs struct {
	ReceptionTest, AuthorizationTest, ReceptionProduction, AuthorizationProduction string
}

var defaultURLs = URLs{
	ReceptionTest:           "https://celcer.sri.gob.ec/comprobantes-electronicos-ws/RecepcionComprobantesOffline",
	AuthorizationTest:       "https://celcer.sri.gob.ec/comprobantes-electronicos-ws/AutorizacionComprobantesOffline",
	ReceptionProduction:     "https://cel.sri.gob.ec/comprobantes-electronicos-ws/RecepcionComprobantesOffline",
	AuthorizationProduction: "https://cel.sri.gob.ec/comprobantes-electronicos-ws/AutorizacionComprobantesOffline",
}

// DefaultURLs returns the official SRI endpoints.
func DefaultURLs() URLs { return defaultURLs }

// maxBody caps the answer read from the SRI.
const maxBody = 32 << 20

// ecuadorTime is Ecuador's time zone (America/Guayaquil), fixed at UTC-5.
var ecuadorTime = time.FixedZone("ECT", -5*60*60)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client (default: 45 s timeout).
func WithHTTPClient(c *http.Client) Option { return func(cl *Client) { cl.http = c } }

// WithURLs overrides the endpoints (tests, proxies).
func WithURLs(u URLs) Option { return func(cl *Client) { cl.urls = u } }

// Client calls the SRI reception and authorization services.
type Client struct {
	http *http.Client
	urls URLs
}

// New builds a Client.
func New(opts ...Option) *Client {
	c := &Client{http: &http.Client{Timeout: 45 * time.Second}, urls: defaultURLs}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Send submits a signed receipt to the reception service.
func (c *Client) Send(ctx context.Context, env sri.Environment, signedXML []byte) (*sri.Reception, error) {
	url, err := c.endpoint(env, true)
	if err != nil {
		return nil, err
	}
	envelope := `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ec="http://ec.gob.sri.ws.recepcion">
   <soapenv:Header/>
   <soapenv:Body>
      <ec:validarComprobante>
         <xml>` + base64.StdEncoding.EncodeToString(signedXML) + `</xml>
      </ec:validarComprobante>
   </soapenv:Body>
</soapenv:Envelope>`
	body, err := c.post(ctx, url, envelope)
	if err != nil {
		return nil, err
	}
	var env2 receptionEnvelope
	if err := decode(body, &env2); err != nil {
		return nil, err
	}
	if strings.TrimSpace(env2.Body.Response.Result.Status) == "" {
		return nil, badResponse(errors.New("la respuesta no trae el estado de la recepción"))
	}
	r := env2.Body.Response.Result
	res := &sri.Reception{Status: sri.ReceptionStatus(strings.ToUpper(strings.TrimSpace(r.Status)))}
	for _, v := range r.Receipts.Receipt {
		if res.AccessKey == "" {
			res.AccessKey = strings.TrimSpace(v.AccessKey)
		}
		res.Messages = append(res.Messages, v.Messages.list()...)
	}
	return res, nil
}

// Authorize asks the authorization service about an access key. When the
// SRI has no authorization for the key the status is sri.StatusNone; when it
// answers several entries, an AUTORIZADO one wins, else the most recent. A
// valid access key whose environment differs from env is a KindClient error
// and nothing is sent.
func (c *Client) Authorize(ctx context.Context, env sri.Environment, accessKey string) (*sri.Authorization, error) {
	url, err := c.endpoint(env, false)
	if err != nil {
		return nil, err
	}
	// A valid key names its environment: asking the other service would
	// answer "no authorization" for a receipt that may exist.
	if parts, perr := sri.ParseAccessKey(accessKey); perr == nil && parts.Environment != env {
		return nil, &Error{Kind: KindClient, Err: errors.New("la clave de acceso es de otro ambiente")}
	}
	var esc bytes.Buffer
	_ = xml.EscapeText(&esc, []byte(accessKey))
	envelope := `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ec="http://ec.gob.sri.ws.autorizacion">
   <soapenv:Header/>
   <soapenv:Body>
      <ec:autorizacionComprobante>
         <claveAccesoComprobante>` + esc.String() + `</claveAccesoComprobante>
      </ec:autorizacionComprobante>
   </soapenv:Body>
</soapenv:Envelope>`
	body, err := c.post(ctx, url, envelope)
	if err != nil {
		return nil, err
	}
	var env2 authorizationEnvelope
	if err := decode(body, &env2); err != nil {
		return nil, err
	}
	if env2.Body.Response.Result == nil {
		return nil, badResponse(errors.New("la respuesta no trae la autorización"))
	}
	res := &sri.Authorization{AccessKey: accessKey, Environment: env}
	if parts, err := sri.ParseAccessKey(accessKey); err == nil {
		res.Environment = parts.Environment
	}
	entries := env2.Body.Response.Result.Authorizations.Entry
	if len(entries) == 0 {
		return res, nil
	}
	e := pick(entries)
	res.Status = sri.AuthorizationStatus(strings.ToUpper(strings.TrimSpace(e.Status)))
	res.Number = strings.TrimSpace(e.Number)
	res.Date, _ = parseDate(e.Date)
	res.XML = []byte(e.Receipt)
	res.Messages = e.Messages.list()
	return res, nil
}

func (c *Client) endpoint(env sri.Environment, reception bool) (string, error) {
	switch {
	case env == sri.EnvironmentTest && reception:
		return c.urls.ReceptionTest, nil
	case env == sri.EnvironmentTest:
		return c.urls.AuthorizationTest, nil
	case env == sri.EnvironmentProduction && reception:
		return c.urls.ReceptionProduction, nil
	case env == sri.EnvironmentProduction:
		return c.urls.AuthorizationProduction, nil
	}
	return "", &Error{Kind: KindClient, Err: fmt.Errorf("ambiente desconocido: %d", env)}
}

func (c *Client) post(ctx context.Context, url, envelope string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(envelope))
	if err != nil {
		return nil, &Error{Kind: KindClient, Err: err}
	}
	req.Header.Set("Content-Type", "text/xml; charset=UTF-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, transportError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, transportError(err)
	}
	switch {
	case resp.StatusCode >= 500:
		return nil, &Error{Kind: KindServer, StatusCode: resp.StatusCode, Err: errors.New(snippet(body))}
	case resp.StatusCode >= 400:
		return nil, &Error{Kind: KindClient, StatusCode: resp.StatusCode, Err: errors.New(snippet(body))}
	}
	return body, nil
}

func transportError(err error) error {
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return &Error{Kind: KindNetwork, Err: err}
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return &Error{Kind: KindTimeout, Err: err}
	}
	return &Error{Kind: KindNetwork, Err: err}
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

type fault struct {
	String string `xml:"faultstring"`
}

type messages struct {
	Message []struct {
		Identifier     string `xml:"identificador"`
		Text           string `xml:"mensaje"`
		AdditionalInfo string `xml:"informacionAdicional"`
		Type           string `xml:"tipo"`
	} `xml:"mensaje"`
}

func (m messages) list() []sri.Message {
	var out []sri.Message
	for _, v := range m.Message {
		out = append(out, sri.Message{
			Identifier: strings.TrimSpace(v.Identifier), Text: v.Text, AdditionalInfo: v.AdditionalInfo,
			Type: sri.MessageType(strings.ToUpper(strings.TrimSpace(v.Type))),
		})
	}
	return out
}

type receptionEnvelope struct {
	Body struct {
		Fault    *fault `xml:"Fault"`
		Response struct {
			Result struct {
				Status   string `xml:"estado"`
				Receipts struct {
					Receipt []struct {
						AccessKey string   `xml:"claveAcceso"`
						Messages  messages `xml:"mensajes"`
					} `xml:"comprobante"`
				} `xml:"comprobantes"`
			} `xml:"RespuestaRecepcionComprobante"`
		} `xml:"validarComprobanteResponse"`
	} `xml:"Body"`
}

type authEntry struct {
	Status   string   `xml:"estado"`
	Number   string   `xml:"numeroAutorizacion"`
	Date     string   `xml:"fechaAutorizacion"`
	Receipt  string   `xml:"comprobante"`
	Messages messages `xml:"mensajes"`
}

type authorizationEnvelope struct {
	Body struct {
		Fault    *fault `xml:"Fault"`
		Response struct {
			Result *struct {
				Authorizations struct {
					Entry []authEntry `xml:"autorizacion"`
				} `xml:"autorizaciones"`
			} `xml:"RespuestaAutorizacionComprobante"`
		} `xml:"autorizacionComprobanteResponse"`
	} `xml:"Body"`
}

// decode unmarshals a SOAP answer; anything that is not XML or carries a
// fault is a bad response.
func decode(body []byte, v interface {
	fault() *fault
}) error {
	if err := xml.Unmarshal(body, v); err != nil {
		return badResponse(fmt.Errorf("la respuesta no es un mensaje SOAP: %w", err))
	}
	if f := v.fault(); f != nil {
		return badResponse(fmt.Errorf("SOAP fault: %s", f.String))
	}
	return nil
}

func badResponse(err error) *Error {
	return &Error{Kind: KindBadResponse, StatusCode: http.StatusOK, Err: err}
}

func (e *receptionEnvelope) fault() *fault     { return e.Body.Fault }
func (e *authorizationEnvelope) fault() *fault { return e.Body.Fault }

// pick chooses the entry that decides the receipt: an AUTORIZADO one, else
// the one with the most recent parseable date, else the first.
func pick(entries []authEntry) authEntry {
	best, bestDate, found := entries[0], time.Time{}, false
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Status), string(sri.StatusAuthorized)) {
			return e
		}
		if d, ok := parseDate(e.Date); ok && (!found || d.After(bestDate)) {
			best, bestDate, found = e, d, true
		}
	}
	return best
}

// parseDate reads fechaAutorizacion: RFC 3339 (with or without fractions) or
// the older dd/mm/aaaa hh:mm:ss in Ecuador's time.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(ecuadorTime), true
	}
	if t, err := time.ParseInLocation("02/01/2006 15:04:05", s, ecuadorTime); err == nil {
		return t, true
	}
	return time.Time{}, false
}
