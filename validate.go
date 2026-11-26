package sri

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

var (
	sequentialDigits = regexp.MustCompile(`^\d{1,9}$`)
	// withholdingAgentResolution and specialTaxpayerNumber are the XSD
	// patterns of agenteRetencion and contribuyenteEspecial.
	withholdingAgentResolution = regexp.MustCompile(`^\d{1,8}$`)
	specialTaxpayerNumber      = regexp.MustCompile(`^[A-Za-z0-9]{3,13}$`)
)

// Decimal places the schemas accept: quantity and unit price take 6,
// every other amount 2.
const (
	quantityPlaces = 6
	moneyPlaces    = 2
)

// tooManyDecimals reports whether d has more significant decimals than places
// (trailing zeros do not count).
func tooManyDecimals(d decimal.Decimal, places int32) bool { return !d.Equal(d.Round(places)) }

// paddedSequential completes a sequential of up to 9 digits with zeros.
func paddedSequential(s string) string {
	if len(s) >= 9 {
		return s
	}
	return strings.Repeat("0", 9-len(s)) + s
}

func (is Issuer) validate(v *validation) {
	if !ValidRUC(is.RUC) {
		v.add("issuer.ruc", CodeIssuerRUCInvalid)
	}
	if normalizeText(is.BusinessName) == "" {
		v.add("issuer.business_name", CodeIssuerNameRequired)
	}
	if normalizeText(is.MainAddress) == "" {
		v.add("issuer.main_address", CodeIssuerAddressRequired)
	}
	if is.WithholdingAgentResolution != "" && !withholdingAgentResolution.MatchString(is.WithholdingAgentResolution) {
		v.add("issuer.withholding_agent_resolution", CodeIssuerWithholdingAgentInvalid)
	}
	if is.SpecialTaxpayerNumber != "" && !specialTaxpayerNumber.MatchString(is.SpecialTaxpayerNumber) {
		v.add("issuer.special_taxpayer_number", CodeIssuerSpecialTaxpayerInvalid)
	}
	switch is.Rimpe {
	case RimpeNone, RimpeEntrepreneur, RimpePopularBusiness:
	default:
		v.add("issuer.rimpe", CodeIssuerRimpeInvalid)
	}
}

// validate checks the header of a document of type doc issued by ruc,
// including the access key when one is given: its fields must match the
// document, and its emission type must be 1 (normal), the one the XML prints.
func (h Header) validate(v *validation, doc DocType, ruc string) {
	if h.Environment != EnvironmentTest && h.Environment != EnvironmentProduction {
		v.add("header.environment", CodeHeaderEnvironmentInvalid)
	}
	if !threeDigits.MatchString(h.Establishment) {
		v.add("header.establishment", CodeHeaderEstablishmentInvalid)
	}
	if !threeDigits.MatchString(h.EmissionPoint) {
		v.add("header.emission_point", CodeHeaderEmissionPointInvalid)
	}
	if !sequentialDigits.MatchString(h.Sequential) || strings.Trim(h.Sequential, "0") == "" {
		v.add("header.sequential", CodeHeaderSequentialInvalid)
	}
	if h.IssueDate.IsZero() {
		v.add("header.issue_date", CodeHeaderIssueDateRequired)
	}
	if h.AccessKey == "" {
		return
	}
	k, err := ParseAccessKey(h.AccessKey)
	if err != nil {
		v.add("header.access_key", CodeAccessKeyInvalid)
		return
	}
	if k.IssueDate.Format(accessKeyDateForm) != h.IssueDate.Format(accessKeyDateForm) || k.Doc != doc || k.RUC != ruc ||
		k.Environment != h.Environment || k.Establishment != h.Establishment || k.EmissionPoint != h.EmissionPoint ||
		k.Sequential != paddedSequential(h.Sequential) || k.EmissionType != emissionTypeCode {
		v.add("header.access_key", CodeAccessKeyMismatch)
	}
}

// validate checks a buyer or supplier under the field prefix. A final
// consumer is accepted when refuseFinalConsumer is empty; otherwise it is
// reported on id_type with that code.
func (p Party) validate(v *validation, prefix, refuseFinalConsumer string) {
	switch p.IDType {
	case IDRUC:
		if !ValidRUC(p.ID) {
			v.add(prefix+".id", CodePartyRUCInvalid)
		}
	case IDCedula:
		if !ValidCedula(p.ID) {
			v.add(prefix+".id", CodePartyCedulaInvalid)
		}
	case IDPassport, IDForeign:
		if !validForeignID(p.ID) {
			v.add(prefix+".id", CodePartyPassportInvalid)
		}
	case IDFinalConsumer:
		if refuseFinalConsumer != "" {
			v.add(prefix+".id_type", refuseFinalConsumer)
		} else if p.ID != FinalConsumerID {
			v.add(prefix+".id", CodePartyFinalConsumerID)
		}
	default:
		v.add(prefix+".id_type", CodePartyIDTypeInvalid)
	}
	if normalizeText(p.Name) == "" {
		v.add(prefix+".name", CodePartyNameRequired)
	}
	if e := strings.TrimSpace(p.Email); e != "" && !validEmail(e) {
		v.add(prefix+".email", CodePartyEmailInvalid)
	}
}

// validateLines checks the items of an invoice or credit note, and that
// their computed base (the subtotals at 0 % and 15 %) is above zero. The
// total_zero check is skipped when an item already has an amount problem
// (negative, or a discount above its value), so one cause is reported once.
func validateLines(v *validation, lines []Line, totals Totals) {
	if len(lines) == 0 {
		v.add("lines", CodeLinesRequired)
		return
	}
	amountsOK := true
	for i, l := range lines {
		p := fmt.Sprintf("lines[%d]", i)
		if normalizeText(l.Description) == "" {
			v.add(p+".description", CodeLineDescriptionRequired)
		}
		negative := false
		for _, f := range []struct {
			name   string
			value  decimal.Decimal
			places int32
		}{{"quantity", l.Quantity, quantityPlaces}, {"unit_price", l.UnitPrice, quantityPlaces}, {"discount", l.Discount, moneyPlaces}} {
			if f.value.IsNegative() {
				v.add(p+"."+f.name, CodeLineNegativeAmount)
				negative = true
			}
			if tooManyDecimals(f.value, f.places) {
				v.add(p+"."+f.name, CodeAmountTooManyDecimals)
			}
		}
		if negative {
			amountsOK = false
		} else if l.Discount.GreaterThan(l.Quantity.Mul(l.UnitPrice)) {
			v.add(p+".discount", CodeLineDiscountExceeds)
			amountsOK = false
		}
		if l.TaxRate != 0 && l.TaxRate != 15 {
			v.add(p+".tax_rate", CodeLineTaxRateUnsupported)
		}
		if textTooLong(l.Code, maxCodeLen) {
			v.add(p+".code", CodeTextTooLong)
		}
		if textTooLong(l.AuxCode, maxCodeLen) {
			v.add(p+".aux_code", CodeTextTooLong)
		}
	}
	if amountsOK && !totals.Subtotal0.Add(totals.Subtotal15).IsPositive() {
		v.add("lines", CodeTotalZero)
	}
}

// validatePayments checks the payments under the field prefix: at least one,
// each with a catalog method, a positive amount of up to 2 decimals and a
// term that is not negative, adding up to total.
func validatePayments(v *validation, prefix string, payments []Payment, total decimal.Decimal) {
	if len(payments) == 0 {
		v.add(prefix, CodePaymentsRequired)
		return
	}
	var sum decimal.Decimal
	for i, p := range payments {
		if !validPaymentMethod(p.Method) {
			v.add(fmt.Sprintf("%s[%d].method", prefix, i), CodePaymentMethodInvalid)
		}
		if !p.Amount.IsPositive() {
			v.add(fmt.Sprintf("%s[%d].amount", prefix, i), CodePaymentAmountInvalid)
		}
		if tooManyDecimals(p.Amount, moneyPlaces) {
			v.add(fmt.Sprintf("%s[%d].amount", prefix, i), CodeAmountTooManyDecimals)
		}
		if p.TermDays < 0 {
			v.add(fmt.Sprintf("%s[%d].term_days", prefix, i), CodePaymentAmountInvalid)
		}
		sum = sum.Add(p.Amount)
	}
	if !sum.Equal(total) {
		v.add(prefix, CodePaymentsTotalMismatch)
	}
}

// maxAdditionalFields is the most campoAdicional elements the schemas accept.
const maxAdditionalFields = 15

func validateAdditional(v *validation, fields []AdditionalField) {
	if len(fields) > maxAdditionalFields {
		v.add("additional", CodeAdditionalFieldsLimit)
	}
	for i, f := range fields {
		name, value := normalizeText(f.Name), normalizeText(f.Value)
		if name == "" || value == "" || textTooLong(name, maxTextLen) || textTooLong(value, maxTextLen) {
			v.add(fmt.Sprintf("additional[%d]", i), CodeAdditionalFieldInvalid)
		}
	}
}
