package sri

// Validate checks the invoice and returns nil or ValidationErrors with every
// problem found.
func (i Invoice) Validate() error {
	var v validation
	i.Issuer.validate(&v)
	i.Header.validate(&v, DocInvoice, i.Issuer.RUC)
	i.Buyer.validate(&v, "buyer", "")
	totals := i.Totals()
	if i.Buyer.IDType == IDFinalConsumer && totals.Total.GreaterThan(finalConsumerLimit) {
		v.add("buyer", CodeBuyerFinalConsumerLimit)
	}
	validateLines(&v, i.Lines, totals)
	validatePayments(&v, "payments", i.Payments, totals.Total)
	validateAdditional(&v, i.Additional)
	return v.result()
}
