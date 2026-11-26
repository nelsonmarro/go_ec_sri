package sri

import "github.com/shopspring/decimal"

// LineTotals are the computed amounts of one line.
type LineTotals struct {
	Subtotal, IVA decimal.Decimal
}

// Totals are the amounts the library computes from the lines. Discount is the
// sum of the line discounts rounded to 2 decimals.
type Totals struct {
	Subtotal0, Subtotal15, Discount, IVA, Total decimal.Decimal
	Lines                                       []LineTotals
}

// Totals computes the invoice totals from its lines (see computeTotals).
func (i Invoice) Totals() Totals { return computeTotals(i.Lines) }

// Totals computes the credit note totals from its lines (see computeTotals).
func (c CreditNote) Totals() Totals { return computeTotals(c.Lines) }

var (
	rate15  = decimal.RequireFromString("0.15")
	oneCent = decimal.New(1, -2)
)

// computeTotals applies the SRI rounding rule:
//   - line subtotal = quantity × unit price − discount, rounded to 2 decimals;
//   - total VAT = (sum of the 15 % bases) × 15 %, rounded to 2 decimals;
//   - line VAT = its base × its rate, rounded to 2 decimals; when the lines do
//     not add up to the total VAT, the gap is spread one cent at a time over
//     the 15 % lines with a positive base, from the last to the first, never
//     leaving a line negative (the SRI checks the lines against the totals);
//   - total = subtotals + VAT.
//
// It assumes rates 0 or 15; Validate reports any other rate (treated here as 0 %).
func computeTotals(lines []Line) Totals {
	t := Totals{Lines: make([]LineTotals, len(lines))}
	var lineIVA decimal.Decimal
	for i, l := range lines {
		sub := l.Quantity.Mul(l.UnitPrice).Sub(l.Discount).Round(2)
		t.Lines[i].Subtotal = sub
		t.Discount = t.Discount.Add(l.Discount)
		if l.TaxRate == 15 {
			t.Subtotal15 = t.Subtotal15.Add(sub)
			t.Lines[i].IVA = sub.Mul(rate15).Round(2)
			lineIVA = lineIVA.Add(t.Lines[i].IVA)
		} else {
			t.Subtotal0 = t.Subtotal0.Add(sub)
		}
	}
	t.Discount = t.Discount.Round(2)
	t.IVA = t.Subtotal15.Mul(rate15).Round(2)
	spreadGap(lines, t.Lines, t.IVA.Sub(lineIVA))
	t.Total = t.Subtotal0.Add(t.Subtotal15).Add(t.IVA)
	return t
}

// spreadGap moves gap onto the line VATs one cent at a time, last to first,
// over the 15 % lines with a positive base, skipping a line that would go
// negative. With the total VAT computed from the bases, the gap never exceeds
// what one pass can absorb.
func spreadGap(lines []Line, totals []LineTotals, gap decimal.Decimal) {
	step := oneCent
	if gap.IsNegative() {
		step = oneCent.Neg()
	}
	for i := len(lines) - 1; i >= 0 && !gap.IsZero(); i-- {
		if lines[i].TaxRate != 15 || !totals[i].Subtotal.IsPositive() {
			continue
		}
		moved := totals[i].IVA.Add(step)
		if moved.IsNegative() {
			continue
		}
		totals[i].IVA = moved
		gap = gap.Sub(step)
	}
}
