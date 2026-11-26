package sri

import (
	"fmt"
	"regexp"

	"github.com/shopspring/decimal"
)

var (
	// supportAuthorization is a supplier's authorization number: 10 digits
	// (pre-printed), 37 (old electronic) or 49 (access key).
	supportAuthorization = regexp.MustCompile(`^(\d{10}|\d{37}|\d{49})$`)
	maxPercentage        = decimal.NewFromInt(100)
	hundred              = decimal.NewFromInt(100)
)

// maxWithholdingCodeLen is the length of codigoRetencion in the 07 schema.
const maxWithholdingCodeLen = 5

// Validate checks the withholding and returns nil or ValidationErrors with
// every problem found. The supplier cannot be a final consumer nor have a
// foreign identification (08).
func (w Withholding) Validate() error {
	var v validation
	w.Issuer.validate(&v)
	w.Header.validate(&v, DocWithholding, w.Issuer.RUC)
	w.Supplier.validate(&v, "supplier", CodePartyIDTypeInvalid)
	if w.Supplier.IDType == IDForeign {
		// A foreign supplier needs tipoSujetoRetenido and a foreign payment,
		// which this version does not build.
		v.add("supplier.id_type", CodePartyIDTypeInvalid)
	}
	if w.FiscalPeriod.IsZero() {
		v.add("fiscal_period", CodeWithholdingPeriodRequired)
	}
	w.Support.validate(&v)
	validateWithholdingLines(&v, w.Lines)
	validateAdditional(&v, w.Additional)
	return v.result()
}

// validate checks the support document: type, number, date, authorization,
// sustento code for its type, totals (they add up, the base is positive and
// VAT has a 15 % base) and payments.
func (s SupportDocument) validate(v *validation) {
	docOK := s.DocType == DocInvoice || s.DocType == DocSalesNote
	if !docOK {
		v.add("support.doc_type", CodeSupportDocumentInvalid)
	}
	if !documentNumber.MatchString(s.Number) {
		v.add("support.number", CodeSupportDocumentInvalid)
	}
	if s.IssueDate.IsZero() {
		v.add("support.issue_date", CodeSupportDocumentInvalid)
	}
	if s.Authorization != "" && !supportAuthorization.MatchString(s.Authorization) {
		v.add("support.authorization", CodeSupportDocumentInvalid)
	}
	if docOK && !sustentoAllowed(s.DocType, s.SustentoCode) {
		v.add("support.sustento_code", CodeSupportSustentoInvalid)
	}
	for _, f := range []struct {
		name  string
		value decimal.Decimal
	}{{"subtotal0", s.Subtotal0}, {"subtotal15", s.Subtotal15}, {"iva", s.IVA}, {"total", s.Total}} {
		if tooManyDecimals(f.value, moneyPlaces) {
			v.add("support."+f.name, CodeAmountTooManyDecimals)
		}
	}
	negative := s.Subtotal0.IsNegative() || s.Subtotal15.IsNegative() || s.IVA.IsNegative() || s.Total.IsNegative()
	if negative || !s.Subtotal0.Add(s.Subtotal15).Add(s.IVA).Equal(s.Total) {
		v.add("support.total", CodeSupportTotalsMismatch)
	}
	// The 07 needs at least one tax of the support document with a base, and
	// VAT only exists on a 15 % base.
	if !negative && (!s.Subtotal0.Add(s.Subtotal15).IsPositive() || (s.IVA.IsPositive() && s.Subtotal15.IsZero())) {
		v.add("support", CodeSupportTotalsMismatch)
	}
	validatePayments(v, "support.payments", s.Payments, s.Total)
}

// validateWithholdingLines checks each line and, when its tax, base and
// percentage are valid, that its amount is base × percentage rounded to the
// cent, within one cent.
func validateWithholdingLines(v *validation, lines []WithholdingLine) {
	if len(lines) == 0 {
		v.add("lines", CodeLinesRequired)
		return
	}
	for i, l := range lines {
		p := fmt.Sprintf("lines[%d]", i)
		ok := true
		bad := func(field, code string) {
			v.add(p+"."+field, code)
			ok = false
		}
		if l.Tax != WithholdingRenta && l.Tax != WithholdingIVA {
			bad("tax", CodeWithholdingLineInvalid)
		}
		if code := normalizeText(l.Code); code == "" || textTooLong(code, maxWithholdingCodeLen) {
			bad("code", CodeWithholdingLineInvalid)
		}
		if !l.Base.IsPositive() {
			bad("base", CodeWithholdingLineInvalid)
		} else if tooManyDecimals(l.Base, moneyPlaces) {
			bad("base", CodeAmountTooManyDecimals)
		}
		if l.Percentage.IsNegative() || l.Percentage.GreaterThan(maxPercentage) || !l.Percentage.Equal(l.Percentage.Round(2)) {
			bad("percentage", CodeWithholdingLineInvalid)
		} else if l.Tax == WithholdingIVA {
			if code, known := ivaWithholdingCode(l.Percentage); !known {
				bad("percentage", CodeWithholdingIVAPercentage)
			} else if code != normalizeText(l.Code) {
				bad("code", CodeWithholdingLineInvalid)
			}
		}
		if tooManyDecimals(l.Amount, moneyPlaces) {
			v.add(p+".amount", CodeAmountTooManyDecimals)
		} else if ok && l.Amount.Sub(withholdingAmount(l.Base, l.Percentage)).Abs().GreaterThan(oneCent) {
			v.add(p+".amount", CodeWithholdingAmountMismatch)
		}
	}
}

// withholdingAmount is base × percentage ÷ 100, rounded to the cent.
func withholdingAmount(base, pct decimal.Decimal) decimal.Decimal {
	return base.Mul(pct).Div(hundred).Round(2)
}
