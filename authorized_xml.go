package sri

import (
	"bytes"
	"errors"
	"html"
	"time"
)

// ecuadorTime is Ecuador's time zone (America/Guayaquil). Ecuador has kept
// UTC−05:00 without daylight saving since 1993, so a fixed zone gives the same
// times without reading the system's time zone database.
var ecuadorTime = time.FixedZone("ECT", -5*60*60)

// AuthorizedXML wraps the authorized receipt in the SRI <autorizacion>
// envelope, the file the receiver keeps as the authorized receipt: state
// AUTORIZADO, the authorization number (the access key when Number is empty),
// the date in Ecuador's time (RFC 3339), the environment and the receipt in
// CDATA. It refuses an authorization that is not AUTORIZADO or lacks its
// date, number or XML.
func AuthorizedXML(a Authorization) ([]byte, error) {
	if a.Status != StatusAuthorized {
		return nil, errors.New("solo un comprobante autorizado tiene el XML autorizado")
	}
	if a.Date.IsZero() {
		return nil, errors.New("el comprobante autorizado no tiene fecha de autorización")
	}
	if len(bytes.TrimSpace(a.XML)) == 0 {
		return nil, errors.New("el comprobante autorizado no tiene su XML")
	}
	number := a.Number
	if number == "" {
		number = a.AccessKey
	}
	if number == "" {
		return nil, errors.New("el comprobante autorizado no tiene número de autorización")
	}
	var env string
	switch a.Environment {
	case EnvironmentTest:
		env = "PRUEBAS"
	case EnvironmentProduction:
		env = "PRODUCCIÓN"
	default:
		return nil, errors.New("el ambiente debe ser 1 (pruebas) o 2 (producción)")
	}
	cdata := bytes.ReplaceAll(a.XML, []byte("]]>"), []byte("]]]]><![CDATA[>"))
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<autorizacion>\n  <estado>AUTORIZADO</estado>\n")
	b.WriteString("  <numeroAutorizacion>" + html.EscapeString(number) + "</numeroAutorizacion>\n")
	b.WriteString("  <fechaAutorizacion>" + a.Date.In(ecuadorTime).Format(time.RFC3339) + "</fechaAutorizacion>\n")
	b.WriteString("  <ambiente>" + env + "</ambiente>\n  <comprobante><![CDATA[")
	b.Write(cdata)
	b.WriteString("]]></comprobante>\n  <mensajes/>\n</autorizacion>\n")
	return b.Bytes(), nil
}

// Today returns today's calendar date in Ecuador (America/Guayaquil) at
// midnight, in Ecuador's time zone. Use it for Header.IssueDate when the
// receipt is issued now: a server running in UTC would otherwise print
// tomorrow's date after 19:00 in Ecuador.
func Today() time.Time { return todayAt(time.Now()) }

func todayAt(now time.Time) time.Time {
	y, m, d := now.In(ecuadorTime).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, ecuadorTime)
}
