package sri

import "regexp"

// documentNumber is the printed number of a document: 001-001-000000123.
var documentNumber = regexp.MustCompile(`^\d{3}-\d{3}-\d{9}$`)

// Validate checks the credit note and returns nil or ValidationErrors with
// every problem found. The buyer cannot be a final consumer: the SRI does not
// accept a credit note on an invoice issued to one. A credit note carries no
// payments.
func (c CreditNote) Validate() error {
	var v validation
	c.Issuer.validate(&v)
	c.Header.validate(&v, DocCreditNote, c.Issuer.RUC)
	c.Buyer.validate(&v, "buyer", CodeCreditNoteFinalConsumer)
	if !documentNumber.MatchString(c.ModifiedNumber) {
		v.add("modified_number", CodeCreditNoteModifiedInvalid)
	}
	if c.ModifiedDate.IsZero() {
		v.add("modified_date", CodeCreditNoteModifiedInvalid)
	}
	if normalizeText(c.Reason) == "" || textTooLong(c.Reason, maxTextLen) {
		v.add("reason", CodeCreditNoteReasonRequired)
	}
	validateLines(&v, c.Lines, c.Totals())
	validateAdditional(&v, c.Additional)
	return v.result()
}
