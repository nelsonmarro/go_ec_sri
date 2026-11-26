package ride

import (
	"io"
	"strconv"

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

// Invoice writes the RIDE (PDF) of an authorized invoice to w. authorizedXML
// must be the <autorizacion> envelope the SRI returns: a bare receipt has no
// authorization number or date and is refused. The amounts are the ones
// printed in the XML.
func Invoice(w io.Writer, authorizedXML []byte, opts Options) error {
	p, err := sri.ParseInvoice(authorizedXML)
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
	inv := p.Invoice
	head := issuerHeader(header{title: "FACTURA", number: documentNumber(inv.Header), issuer: inv.Issuer,
		key: inv.Header.AccessKey, auth: *p.Authorization, logo: logo})
	return render(w, "la factura",
		head,
		partyBlock(inv.Buyer, inv.Header.IssueDate, ""),
		itemsTable(inv.Lines, p.Totals.Lines),
		invoiceTotalsBlock(inv.Payments, totalsRows(p.Totals)),
	)
}

var itemColumns = []struct {
	title string
	size  int
	align align.Type
}{
	{"Cód.", 2, align.Left}, {"Descripción", 4, align.Left}, {"Cant.", 1, align.Right},
	{"P. Unitario", 2, align.Right}, {"Descuento", 1, align.Right}, {"Total", 2, align.Right},
}

// descriptionLineChars approximates how many 7 pt characters fit in the description column.
const descriptionLineChars = 36

// itemsTable draws the detail lines; each line's total is the subtotal printed
// in the XML.
func itemsTable(lines []sri.Line, totals []sri.LineTotals) []core.Row {
	header := row.New(8)
	for _, h := range itemColumns {
		header.Add(col.New(h.size).WithStyle(cellBorder).
			Add(text.New(h.title, props.Text{Style: fontstyle.Bold, Size: 7, Align: align.Center, Top: 1})))
	}
	rows := []core.Row{header}
	for i, l := range lines {
		var subtotal decimal.Decimal
		if i < len(totals) {
			subtotal = totals[i].Subtotal
		}
		cells := []string{l.Code, l.Description, quantity(l.Quantity), quantity(l.UnitPrice), money(l.Discount), money(subtotal)}
		n := (len([]rune(l.Description)) + descriptionLineChars - 1) / descriptionLineChars
		if n < 1 {
			n = 1
		}
		line := row.New(float64(3*n + 4))
		for j, v := range cells {
			line.Add(col.New(itemColumns[j].size).WithStyle(cellBorder).
				Add(text.New(v, props.Text{Size: 7, Align: itemColumns[j].align, Top: 1, Left: 1, Right: 1})))
		}
		rows = append(rows, line)
	}
	return rows
}

// totalsRows returns the label/amount pairs of the totals block: the 15 % and
// 0 % subtotals (0 % also when nothing is taxed), the discount, the VAT and
// the grand total.
func totalsRows(t sri.Totals) [][2]string {
	var rows [][2]string
	if !t.Subtotal15.IsZero() {
		rows = append(rows, [2]string{"Subtotal 15 %", money(t.Subtotal15)})
	}
	if !t.Subtotal0.IsZero() || t.Subtotal15.IsZero() {
		rows = append(rows, [2]string{"Subtotal 0 %", money(t.Subtotal0)})
	}
	rows = append(rows, [2]string{"Descuento", money(t.Discount)})
	if !t.Subtotal15.IsZero() {
		rows = append(rows, [2]string{"IVA 15 %", money(t.IVA)})
	}
	return append(rows, [2]string{"VALOR TOTAL", money(t.Total)})
}

// paymentRows returns name, amount and term of each payment; an unknown code
// keeps the raw code so it is never mislabeled.
func paymentRows(pays []sri.Payment) [][3]string {
	names := sri.PaymentMethods()
	var rows [][3]string
	for _, p := range pays {
		name, ok := names[p.Method]
		if !ok {
			name = p.Method
		}
		term := ""
		if p.TermDays > 0 {
			term = strconv.Itoa(p.TermDays) + " días"
		}
		rows = append(rows, [3]string{name, money(p.Amount), term})
	}
	return rows
}

// invoiceTotalsBlock puts the payment methods on the left and the totals on the right.
func invoiceTotalsBlock(pays []sri.Payment, totals [][2]string) []core.Row {
	payments := paymentRows(pays)
	left := []core.Component{text.New("FORMA DE PAGO", props.Text{Style: fontstyle.Bold, Size: 7, Top: 1, Left: 2})}
	for i, p := range payments {
		line := p[0] + "  " + p[1]
		if p[2] != "" {
			line += "  (plazo: " + p[2] + ")"
		}
		left = append(left, text.New(line, props.Text{Size: 7, Top: 6 + float64(i)*5, Left: 2}))
	}
	height := float64(len(totals)) * 6
	if min := 6 + float64(len(payments))*5 + 2; height < min {
		height = min
	}
	out := row.New(height)
	out.Add(col.New(6).WithStyle(cellBorder).Add(left...))
	labels, values := totalsColumns(totals)
	out.Add(col.New(4).Add(labels...), col.New(2).WithStyle(cellBorder).Add(values...))
	return []core.Row{out}
}

// totalsColumns lays the totals out as a column of labels and a column of
// values; the last row (the grand total) is bold.
func totalsColumns(totals [][2]string) (labels, values []core.Component) {
	for i, t := range totals {
		style := fontstyle.Normal
		if i == len(totals)-1 {
			style = fontstyle.Bold
		}
		labels = append(labels, text.New(t[0], props.Text{Style: style, Size: 7, Top: 1 + float64(i)*6, Align: align.Right, Right: 2}))
		values = append(values, text.New(t[1], props.Text{Style: style, Size: 7, Top: 1 + float64(i)*6, Align: align.Right, Right: 2}))
	}
	return labels, values
}
