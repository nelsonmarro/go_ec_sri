package ride

import (
	"io"

	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/shopspring/decimal"

	sri "github.com/nelsonmarro/go_ec_sri"
)

// Withholding writes the RIDE (PDF) of an authorized withholding (07) to w.
// authorizedXML must be the <autorizacion> envelope; a bare receipt is
// refused. The values are the ones printed in the XML.
func Withholding(w io.Writer, authorizedXML []byte, opts Options) error {
	p, err := sri.ParseWithholding(authorizedXML)
	if err != nil {
		return err
	}
	if err := checkAuthorized(p.Authorization); err != nil {
		return err
	}
	logo, err := prepareLogo(opts.Logo)
	if err != nil {
		return err
	}
	wh := p.Withholding
	head := issuerHeader(header{title: "COMPROBANTE DE RETENCIÓN", number: documentNumber(wh.Header), issuer: wh.Issuer,
		key: wh.Header.AccessKey, auth: *p.Authorization, logo: logo})
	period := wh.FiscalPeriod.Format("01/2006")
	return render(w, "la retención",
		head,
		partyBlock(wh.Supplier, wh.Header.IssueDate, "Período Fiscal: "+period),
		withholdingTable(withholdingRowValues(wh, period), money(withheldTotal(wh.Lines))),
	)
}

var withholdingColumns = []struct {
	title string
	size  int
}{
	{"Comprobante", 1}, {"Número", 2}, {"Fecha Emisión", 1}, {"Ejercicio Fiscal", 1},
	{"Base Imponible", 2}, {"Impuesto", 1}, {"Código", 1}, {"Porcentaje", 1}, {"Valor Retenido", 2},
}

// withholdingTable draws the header, one row per withholding line and the total.
func withholdingTable(values [][]string, total string) []core.Row {
	header := row.New(8)
	for _, h := range withholdingColumns {
		header.Add(col.New(h.size).WithStyle(cellBorder).
			Add(text.New(h.title, props.Text{Style: fontstyle.Bold, Size: 7, Align: align.Center, Top: 1})))
	}
	rows := []core.Row{header}
	for _, cells := range values {
		line := row.New(7)
		for i, v := range cells {
			line.Add(col.New(withholdingColumns[i].size).WithStyle(cellBorder).
				Add(text.New(v, props.Text{Size: 7, Align: align.Center, Top: 1})))
		}
		rows = append(rows, line)
	}
	return append(rows, row.New(8).Add(
		col.New(10).Add(text.New("TOTAL RETENIDO", props.Text{Style: fontstyle.Bold, Size: 8, Align: align.Right, Top: 2, Right: 2})),
		col.New(2).WithStyle(cellBorder).Add(text.New(total, props.Text{Style: fontstyle.Bold, Size: 8, Align: align.Center, Top: 2})),
	))
}

// withholdingRowValues returns the nine cells printed for each withholding
// line, in column order.
func withholdingRowValues(wh sri.Withholding, period string) [][]string {
	var rows [][]string
	for _, l := range wh.Lines {
		rows = append(rows, []string{
			docName(wh.Support.DocType), wh.Support.Number, formatDate(wh.Support.IssueDate), period,
			money(l.Base), taxName(l.Tax), l.Code, quantity(l.Percentage) + " %", money(l.Amount),
		})
	}
	return rows
}

func taxName(t sri.WithholdingTax) string {
	switch t {
	case sri.WithholdingRenta:
		return "RENTA"
	case sri.WithholdingIVA:
		return "IVA"
	}
	return string(t)
}

func docName(d sri.DocType) string {
	switch d {
	case sri.DocInvoice:
		return "FACTURA"
	case sri.DocSalesNote:
		return "NOTA DE VENTA"
	}
	return string(d)
}

// withheldTotal adds the withheld values as printed in each line.
func withheldTotal(lines []sri.WithholdingLine) decimal.Decimal {
	total := decimal.Zero
	for _, l := range lines {
		total = total.Add(l.Amount)
	}
	return total
}
