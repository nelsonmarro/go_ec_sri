package sri

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func assertDec(t *testing.T, want string, got decimal.Decimal, label ...any) {
	t.Helper()
	assert.True(t, dec(want).Equal(got), "%s: want %s, got %s", fmt.Sprint(label...), want, got.String())
}

func line(qty, price, discount string, rate int) Line {
	return Line{Code: "P-001", Description: "Artículo", Quantity: dec(qty), UnitPrice: dec(price), Discount: dec(discount), TaxRate: rate}
}

func TestTotals(t *testing.T) {
	tests := []struct {
		name                              string
		lines                             []Line
		sub0, sub15, discount, iva, total string
		lineSubtotals, lineIVAs           []string
	}{
		{
			name:  "only 0 %",
			lines: []Line{line("2", "10", "0", 0), line("1", "5.565", "0", 0)}, // half away from zero, not to even
			sub0:  "25.57", sub15: "0", discount: "0", iva: "0", total: "25.57",
			lineSubtotals: []string{"20", "5.57"}, lineIVAs: []string{"0", "0"},
		},
		{
			name:  "only 15 %",
			lines: []Line{line("1", "100", "0", 15)},
			sub0:  "0", sub15: "100", discount: "0", iva: "15", total: "115",
			lineSubtotals: []string{"100"}, lineIVAs: []string{"15"},
		},
		{
			name:  "mixed 0 and 15 with a discount",
			lines: []Line{line("2", "10", "0", 0), line("1", "100", "10", 15)},
			sub0:  "20", sub15: "90", discount: "10", iva: "13.50", total: "123.50",
			lineSubtotals: []string{"20", "90"}, lineIVAs: []string{"0", "13.50"},
		},
		{
			name:  "a discount that leaves a fraction is rounded on the line",
			lines: []Line{line("3", "0.333333", "0.01", 15)},
			sub0:  "0", sub15: "0.99", discount: "0.01", iva: "0.15", total: "1.14",
			lineSubtotals: []string{"0.99"}, lineIVAs: []string{"0.15"},
		},
		{
			// Validate rejects a discount with 3 decimals; Totals still rounds the sum.
			name:  "the discount sum is rounded to 2 decimals",
			lines: []Line{line("1", "10", "0.333", 0), line("1", "10", "0.333", 0)},
			sub0:  "19.34", sub15: "0", discount: "0.67", iva: "0", total: "19.34",
			lineSubtotals: []string{"9.67", "9.67"}, lineIVAs: []string{"0", "0"},
		},
		{
			name:  "six decimal quantity and price",
			lines: []Line{line("0.333333", "3.141593", "0", 15)}, // 1.047197... -> 1.05
			sub0:  "0", sub15: "1.05", discount: "0", iva: "0.16", total: "1.21",
			lineSubtotals: []string{"1.05"}, lineIVAs: []string{"0.16"},
		},
		{
			name:  "rounding half away from zero, no gap",
			lines: []Line{line("1", "0.33", "0", 15), line("1", "0.33", "0", 15), line("1", "0.33", "0", 15)},
			sub0:  "0", sub15: "0.99", discount: "0", iva: "0.15", total: "1.14",
			lineSubtotals: []string{"0.33", "0.33", "0.33"}, lineIVAs: []string{"0.05", "0.05", "0.05"},
		},
		{
			name:  "three lines, gap of minus one cent on the last line",
			lines: []Line{line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15)},
			sub0:  "0", sub15: "0.30", discount: "0", iva: "0.05", total: "0.35",
			lineSubtotals: []string{"0.10", "0.10", "0.10"}, lineIVAs: []string{"0.02", "0.02", "0.01"},
		},
		{
			name:  "three lines, minus one cent skips a line that would go negative",
			lines: []Line{line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.03", "0", 15)},
			sub0:  "0", sub15: "0.23", discount: "0", iva: "0.03", total: "0.26",
			lineSubtotals: []string{"0.10", "0.10", "0.03"}, lineIVAs: []string{"0.02", "0.01", "0.00"},
		},
		{
			name:  "three lines, gap of plus one cent on the last line",
			lines: []Line{line("1", "0.03", "0", 15), line("1", "0.23", "0", 15), line("1", "0.43", "0", 15)},
			sub0:  "0", sub15: "0.69", discount: "0", iva: "0.10", total: "0.79",
			lineSubtotals: []string{"0.03", "0.23", "0.43"}, lineIVAs: []string{"0.00", "0.03", "0.07"},
		},
		{
			// With two-decimal bases three lines can only be one cent apart; a
			// two-cent gap needs four lines.
			name:  "four lines, gap of plus two cents on the last two",
			lines: []Line{line("1", "0.03", "0", 15), line("1", "0.03", "0", 15), line("1", "0.03", "0", 15), line("1", "0.03", "0", 15)},
			sub0:  "0", sub15: "0.12", discount: "0", iva: "0.02", total: "0.14",
			lineSubtotals: []string{"0.03", "0.03", "0.03", "0.03"}, lineIVAs: []string{"0", "0", "0.01", "0.01"},
		},
		{
			name:  "four lines, gap of minus two cents on the last two",
			lines: []Line{line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15)},
			sub0:  "0", sub15: "0.40", discount: "0", iva: "0.06", total: "0.46",
			lineSubtotals: []string{"0.10", "0.10", "0.10", "0.10"}, lineIVAs: []string{"0.02", "0.02", "0.01", "0.01"},
		},
		{
			name: "plus one cent skips a free line and a 0 % line",
			lines: []Line{
				line("1", "0.03", "0", 15), line("1", "0.23", "0", 15), line("1", "0.43", "0", 15),
				line("1", "0", "0", 15), line("1", "5", "0", 0),
			},
			sub0: "5", sub15: "0.69", discount: "0", iva: "0.10", total: "5.79",
			lineSubtotals: []string{"0.03", "0.23", "0.43", "0", "5"}, lineIVAs: []string{"0.00", "0.03", "0.07", "0", "0"},
		},
		{
			name: "minus one cent skips a free line and a 0 % line",
			lines: []Line{
				line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15),
				line("1", "0", "0", 15), line("1", "5", "0", 0),
			},
			sub0: "5", sub15: "0.30", discount: "0", iva: "0.05", total: "5.35",
			lineSubtotals: []string{"0.10", "0.10", "0.10", "0", "5"}, lineIVAs: []string{"0.02", "0.02", "0.01", "0", "0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Invoice{Lines: tt.lines}.Totals()
			assertDec(t, tt.sub0, got.Subtotal0, "subtotal 0")
			assertDec(t, tt.sub15, got.Subtotal15, "subtotal 15")
			assertDec(t, tt.discount, got.Discount, "discount")
			assertDec(t, tt.iva, got.IVA, "iva")
			assertDec(t, tt.total, got.Total, "total")
			require.Len(t, got.Lines, len(tt.lines))
			for i := range tt.lines {
				assertDec(t, tt.lineSubtotals[i], got.Lines[i].Subtotal, fmt.Sprintf("line %d subtotal", i))
				assertDec(t, tt.lineIVAs[i], got.Lines[i].IVA, fmt.Sprintf("line %d iva", i))
			}
		})
	}
}

func TestCreditNoteTotalsFollowTheSameRule(t *testing.T) {
	lines := []Line{line("1", "0.10", "0", 15), line("1", "0.10", "0", 15), line("1", "0.10", "0", 15)}
	got := CreditNote{Lines: lines}.Totals()
	assert.Equal(t, Invoice{Lines: lines}.Totals(), got)
	assertDec(t, "0.05", got.IVA)
}

func TestTotalsWithoutLines(t *testing.T) {
	got := Invoice{}.Totals()
	assertDec(t, "0", got.Total)
	assert.Empty(t, got.Lines)
}
