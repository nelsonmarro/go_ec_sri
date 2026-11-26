package sri

// CreditNoteXML validates the credit note and builds its unsigned XML
// (notaCredito, schema 1.1.0) for the invoice it modifies. Invalid input
// returns the ValidationErrors of Validate and no XML. When Header.AccessKey
// is empty a key is generated with a random numeric code (crypto/rand).
//
// The totals and texts follow the same rules as InvoiceXML; the value of the
// modification is the total with VAT, and the schema has no payments and no
// buyer address. ModifiedDate is printed as a calendar date in its own
// location.
func CreditNoteXML(cn CreditNote) (Built, error) {
	if err := cn.Validate(); err != nil {
		return Built{}, err
	}
	// Validate guarantees a positive base (total_zero), so at least one
	// subtotal is positive and totalConImpuestos is never empty.
	totals := cn.Totals()
	key, err := accessKey(cn.Header, DocCreditNote, cn.Issuer.RUC)
	if err != nil {
		return Built{}, err
	}
	doc := xmlCreditNote{
		ID: documentID, Version: "1.1.0",
		InfoTributaria: infoTributaria(cn.Issuer, cn.Header, DocCreditNote, key),
		InfoNotaCredito: xmlCreditNoteInfo{
			FechaEmision:                cn.Header.IssueDate.Format(dateLayout),
			DirEstablecimiento:          xmlText(cn.Issuer.EstablishmentAddress, maxTextLen),
			TipoIdentificacionComprador: string(cn.Buyer.IDType),
			RazonSocialComprador:        buyerName(cn.Buyer),
			IdentificacionComprador:     cn.Buyer.ID,
			ContribuyenteEspecial:       cn.Issuer.SpecialTaxpayerNumber,
			ObligadoContabilidad:        siNo(cn.Issuer.KeepsAccounting),
			CodDocModificado:            string(DocInvoice),
			NumDocModificado:            cn.ModifiedNumber,
			FechaEmisionDocSustento:     cn.ModifiedDate.Format(dateLayout),
			TotalSinImpuestos:           totals.Subtotal0.Add(totals.Subtotal15).StringFixed(2),
			ValorModificacion:           totals.Total.StringFixed(2),
			Moneda:                      currency,
			Motivo:                      xmlText(cn.Reason, maxTextLen),
		},
	}
	for _, t := range taxTotals(totals) {
		doc.InfoNotaCredito.TotalConImpuestos = append(doc.InfoNotaCredito.TotalConImpuestos, xmlCreditNoteTotalTax{
			Codigo: t.Codigo, CodigoPorcentaje: t.CodigoPorcentaje, BaseImponible: t.BaseImponible, Valor: t.Valor,
		})
	}
	for i, l := range cn.Lines {
		doc.Detalles = append(doc.Detalles, xmlCreditNoteLine{
			CodigoInterno: lineCode(l.Code), CodigoAdicional: xmlText(l.AuxCode, maxCodeLen),
			Descripcion: xmlText(l.Description, maxTextLen),
			Cantidad:    l.Quantity.StringFixed(6), PrecioUnitario: l.UnitPrice.StringFixed(6),
			Descuento: l.Discount.StringFixed(2), PrecioTotalSinImpuesto: totals.Lines[i].Subtotal.StringFixed(2),
			Impuestos: lineTax(l, totals.Lines[i]),
		})
	}
	doc.InfoAdicional = infoAdicional(cn.Additional)
	out, err := marshalDocument(doc)
	if err != nil {
		return Built{}, err
	}
	return Built{XML: out, AccessKey: key}, nil
}
