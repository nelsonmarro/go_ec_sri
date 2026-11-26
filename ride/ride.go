// Package ride draws the RIDE (representación impresa del documento
// electrónico) of authorized invoices, credit notes and withholdings as PDF.
//
// The RIDE prints the amounts written in the authorized XML, never a
// recomputation, and needs the <autorizacion> envelope: the authorization
// number and date come from it.
package ride

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder for image.DecodeConfig
	_ "image/png"  // registers the PNG decoder for image.DecodeConfig
	"io"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/code"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	pdfimage "github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/shopspring/decimal"

	sri "github.com/nelsonmarro/go_ec_sri"
)

// Options are the optional settings of a RIDE.
type Options struct {
	// Logo is the issuer's logo, PNG or JPEG; empty prints no logo.
	Logo []byte
}

var errNotAuthorized = errors.New("el comprobante no está autorizado: el RIDE necesita el XML autorizado con su sobre <autorizacion> (número y fecha de autorización)")

var errNoAuthorizationDate = errors.New("el comprobante no tiene fecha de autorización: el RIDE necesita el sobre <autorizacion> con su fecha")

// checkAuthorized refuses a receipt without the authorization data the RIDE
// prints (number and date).
func checkAuthorized(a *sri.AuthorizationInfo) error {
	if a == nil {
		return errNotAuthorized
	}
	if a.Date.IsZero() {
		return errNoAuthorizationDate
	}
	return nil
}

// header holds what the issuer block and the authorization box print.
type header struct {
	title  string
	number string
	issuer sri.Issuer
	key    string
	auth   sri.AuthorizationInfo
	logo   core.Component
}

var cellBorder = &props.Cell{BorderType: border.Full, BorderThickness: 0.1}

// prepareLogo validates the logo before maroto sees it (maroto panics on an
// image it cannot read) and returns the component that draws it, or the
// «NO TIENE LOGO» placeholder when there is none.
func prepareLogo(b []byte) (core.Component, error) {
	if len(b) == 0 {
		return text.New("NO TIENE LOGO", props.Text{Style: fontstyle.Bold, Size: 16, Align: align.Center, Top: 10}), nil
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, errors.New("el logo no es una imagen PNG o JPEG válida")
	}
	ext := extension.Png
	if format == "jpeg" {
		ext = extension.Jpg
	}
	return pdfimage.NewFromBytes(b, ext, props.Rect{Percent: 35}), nil
}

func environmentName(e sri.Environment) string {
	if e == sri.EnvironmentProduction {
		return "PRODUCCIÓN"
	}
	return "PRUEBAS"
}

func documentNumber(h sri.Header) string {
	seq := h.Sequential
	if len(seq) < 9 {
		seq = strings.Repeat("0", 9-len(seq)) + seq
	}
	return fmt.Sprintf("%s-%s-%s", h.Establishment, h.EmissionPoint, seq)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006")
}

// rimpeLegend is the legend the SRI prints for a RIMPE issuer.
func rimpeLegend(r sri.RimpeRegime) string {
	switch r {
	case sri.RimpeEntrepreneur, sri.RimpePopularBusiness:
		return "CONTRIBUYENTE RÉGIMEN RIMPE"
	}
	return ""
}

// issuerHeader draws the issuer block and the authorization box shared by the
// three receipts; only the title differs.
func issuerHeader(h header) []core.Row {
	is := h.issuer
	obligado := "NO"
	if is.KeepsAccounting {
		obligado = "SI"
	}
	left := []core.Component{
		h.logo,
		text.New(is.BusinessName, props.Text{Style: fontstyle.Bold, Size: 8, Top: 40}),
		text.New(is.TradeName, props.Text{Size: 8, Top: 45}),
		text.New("Dirección Matriz: "+is.MainAddress, props.Text{Size: 7, Top: 52}),
		text.New("Dirección Establecimiento: "+is.EstablishmentAddress, props.Text{Size: 7, Top: 58}),
		text.New("OBLIGADO A LLEVAR CONTABILIDAD: "+obligado, props.Text{Size: 7, Top: 64}),
	}
	y := 70.0
	if is.SpecialTaxpayerNumber != "" {
		left = append(left, text.New("Contribuyente Especial Nro.: "+is.SpecialTaxpayerNumber, props.Text{Size: 7, Top: y}))
		y += 5
	}
	if is.WithholdingAgentResolution != "" {
		left = append(left, text.New("Agente de Retención Resolución No. "+is.WithholdingAgentResolution, props.Text{Size: 7, Top: y}))
		y += 5
	}
	if legend := rimpeLegend(is.Rimpe); legend != "" {
		left = append(left, text.New(legend, props.Text{Size: 7, Top: y}))
	}
	return []core.Row{
		row.New(92).Add(
			col.New(6).Add(left...),
			col.New(6).WithStyle(cellBorder).Add(
				text.New("R.U.C.: "+is.RUC, props.Text{Style: fontstyle.Bold, Size: 10, Top: 3, Left: 2}),
				text.New(h.title, props.Text{Style: fontstyle.Bold, Size: 12, Top: 9, Left: 2}),
				text.New("No. "+h.number, props.Text{Size: 10, Top: 15, Left: 2}),
				text.New("NÚMERO DE AUTORIZACIÓN", props.Text{Style: fontstyle.Bold, Size: 8, Top: 22, Left: 2}),
				text.New(h.auth.Number, props.Text{Size: 8, Top: 26, Left: 2}),
				text.New("FECHA Y HORA DE AUTORIZACIÓN", props.Text{Style: fontstyle.Bold, Size: 8, Top: 32, Left: 2}),
				text.New(h.auth.Date.Format("02/01/2006 15:04:05"), props.Text{Size: 8, Top: 36, Left: 2}),
				text.New("AMBIENTE: "+environmentName(h.auth.Environment), props.Text{Size: 8, Top: 42, Left: 2}),
				text.New("EMISIÓN: NORMAL", props.Text{Size: 8, Top: 48, Left: 2}),
				text.New("CLAVE DE ACCESO", props.Text{Style: fontstyle.Bold, Size: 8, Top: 60, Left: 2}),
				code.NewBar(h.key, props.Barcode{Percent: 67, Top: 68, Left: 16}),
				text.New(h.key, props.Text{Size: 8, Align: align.Center, Top: 82}),
			),
		),
	}
}

// partyBlock draws the buyer or supplier box.
func partyBlock(p sri.Party, issueDate time.Time, extra string) []core.Row {
	left := col.New(12)
	if extra != "" {
		left = col.New(8)
	}
	left.Add(
		text.New("Razón Social / Nombres y Apellidos: "+p.Name, props.Text{Style: fontstyle.Bold, Size: 7, Top: 2, Left: 2}),
		text.New("Identificación: "+p.ID, props.Text{Size: 7, Top: 8, Left: 2}),
		text.New("Fecha Emisión: "+formatDate(issueDate), props.Text{Size: 7, Top: 14, Left: 2}),
	)
	cols := []core.Col{left}
	if extra != "" {
		cols = append(cols, col.New(4).Add(text.New(extra, props.Text{Size: 7, Top: 8})))
	}
	return []core.Row{row.New(20).WithStyle(cellBorder).Add(cols...)}
}

// render builds the document and writes it to w only when it is complete, so
// a failure never leaves a partial PDF behind.
func render(w io.Writer, what string, rows ...[]core.Row) error {
	cfg := config.NewBuilder().WithPageSize(pagesize.A4).WithOrientation(orientation.Vertical).
		WithLeftMargin(10).WithRightMargin(10).WithTopMargin(10).Build()
	m := maroto.New(cfg)
	for i, r := range rows {
		if i > 0 {
			m.AddRow(2)
		}
		m.AddRows(r...)
	}
	doc, err := m.Generate()
	if err != nil {
		return fmt.Errorf("no se pudo generar el RIDE de %s: %w", what, err)
	}
	if _, err := w.Write(doc.GetBytes()); err != nil {
		return fmt.Errorf("no se pudo escribir el RIDE de %s: %w", what, err)
	}
	return nil
}

// money prints an amount with two decimals.
func money(d decimal.Decimal) string { return d.StringFixed(2) }

// quantity prints a quantity or unit price with at least two decimals and no
// trailing zeros beyond them: 1.000000 -> 1.00, 0.333333 stays.
func quantity(d decimal.Decimal) string {
	out := d.String() // without trailing zeros
	dot := strings.IndexByte(out, '.')
	if dot < 0 {
		return out + ".00"
	}
	if n := len(out) - dot - 1; n < 2 {
		out += strings.Repeat("0", 2-n)
	}
	return out
}
