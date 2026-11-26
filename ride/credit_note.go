package ride

import (
	"io"
	"strings"

	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"

	sri "github.com/nelsonmarro/go_ec_sri"
)

// creditNoteReasonChars is how many characters of the reason go on one line of
// the full-width «Comprobante que se modifica» block. It is deliberately
// conservative (capital letters are wider than the average): the row may end
// up a little taller than needed, never shorter.
const creditNoteReasonChars = 95

// CreditNote writes the RIDE (PDF) of an authorized credit note to w.
// authorizedXML must be the <autorizacion> envelope; a bare receipt is
// refused. The amounts are the ones printed in the XML.
func CreditNote(w io.Writer, authorizedXML []byte, opts Options) error {
	p, err := sri.ParseCreditNote(authorizedXML)
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
	cn := p.CreditNote
	head := issuerHeader(header{title: "NOTA DE CRÉDITO", number: documentNumber(cn.Header), issuer: cn.Issuer,
		key: cn.Header.AccessKey, auth: *p.Authorization, logo: logo})
	return render(w, "la nota de crédito",
		head,
		partyBlock(cn.Buyer, cn.Header.IssueDate, ""),
		modifiedBlock(cn),
		itemsTable(cn.Lines, p.Totals.Lines),
		creditNoteTotalsBlock(totalsRows(p.Totals)),
	)
}

// wrapText breaks s into lines of at most width runes, at spaces; a word
// longer than width is split. It always returns at least one line.
func wrapText(s string, width int) []string {
	var lines []string
	cur := ""
	flush := func() {
		lines = append(lines, cur)
		cur = ""
	}
	for _, word := range strings.Fields(s) {
		w := []rune(word)
		for len(w) > width {
			if cur != "" {
				flush()
			}
			lines = append(lines, string(w[:width]))
			w = w[width:]
		}
		word = string(w)
		switch {
		case cur == "":
			cur = word
		case len([]rune(cur))+1+len(w) <= width:
			cur += " " + word
		default:
			flush()
			cur = word
		}
	}
	if cur != "" || len(lines) == 0 {
		flush()
	}
	return lines
}

// modifiedBlock draws the invoice the credit note modifies; the reason wraps
// over as many lines as it needs.
func modifiedBlock(cn sri.CreditNote) []core.Row {
	reason := wrapText("Razón de modificación: "+cn.Reason, creditNoteReasonChars)
	height := 16 + float64(len(reason))*4 + 2
	comps := []core.Component{
		text.New("Comprobante que se modifica", props.Text{Style: fontstyle.Bold, Size: 7, Top: 1, Left: 2}),
		text.New("FACTURA "+cn.ModifiedNumber, props.Text{Size: 7, Top: 6, Left: 2}),
		text.New("Fecha de emisión: "+formatDate(cn.ModifiedDate), props.Text{Size: 7, Top: 11, Left: 2}),
	}
	for i, l := range reason {
		comps = append(comps, text.New(l, props.Text{Size: 7, Top: 16 + float64(i)*4, Left: 2}))
	}
	return []core.Row{row.New(height).WithStyle(cellBorder).Add(col.New(12).Add(comps...))}
}

// creditNoteTotalsBlock puts the totals in the right half; there is no payment block.
func creditNoteTotalsBlock(totals [][2]string) []core.Row {
	labels, values := totalsColumns(totals)
	return []core.Row{
		row.New(float64(len(totals))*6).Add(col.New(6), col.New(4).Add(labels...), col.New(2).WithStyle(cellBorder).Add(values...)),
	}
}
