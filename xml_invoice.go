package sri

import (
	"strconv"

	"github.com/shopspring/decimal"
)

// InvoiceXML validates the invoice and builds its unsigned XML (factura,
// schema 1.1.0). Invalid input returns the ValidationErrors of Validate and
// no XML. When Header.AccessKey is empty a key is generated with a random
// numeric code (crypto/rand).
//
// The totals come from Totals(): each line prints its own VAT, cent spread
// included. Texts are trimmed, their line breaks collapsed and capped at the
// schema length; an empty item code becomes GEN; a final consumer is printed
// as CONSUMIDOR FINAL. The buyer's address goes to direccionComprador; the
// email and phone are not printed (add them as additional fields).
func InvoiceXML(inv Invoice) (Built, error) {
	if err := inv.Validate(); err != nil {
		return Built{}, err
	}
	// Validate guarantees a positive base (total_zero), so at least one
	// subtotal is positive and totalConImpuestos is never empty.
	totals := inv.Totals()
	key, err := accessKey(inv.Header, DocInvoice, inv.Issuer.RUC)
	if err != nil {
		return Built{}, err
	}
	doc := xmlInvoice{
		ID: documentID, Version: "1.1.0",
		InfoTributaria: infoTributaria(inv.Issuer, inv.Header, DocInvoice, key),
		InfoFactura: xmlInvoiceInfo{
			FechaEmision:                inv.Header.IssueDate.Format(dateLayout),
			DirEstablecimiento:          xmlText(inv.Issuer.EstablishmentAddress, maxTextLen),
			ContribuyenteEspecial:       inv.Issuer.SpecialTaxpayerNumber,
			ObligadoContabilidad:        siNo(inv.Issuer.KeepsAccounting),
			TipoIdentificacionComprador: string(inv.Buyer.IDType),
			RazonSocialComprador:        buyerName(inv.Buyer),
			IdentificacionComprador:     inv.Buyer.ID,
			DireccionComprador:          xmlText(inv.Buyer.Address, maxTextLen),
			TotalSinImpuestos:           totals.Subtotal0.Add(totals.Subtotal15).StringFixed(2),
			TotalDescuento:              totals.Discount.StringFixed(2),
			Propina:                     "0.00",
			ImporteTotal:                totals.Total.StringFixed(2),
			Moneda:                      currency,
		},
	}
	for _, t := range taxTotals(totals) {
		doc.InfoFactura.TotalConImpuestos = append(doc.InfoFactura.TotalConImpuestos, xmlInvoiceTotalTax(t))
	}
	for _, p := range inv.Payments {
		pay := xmlInvoicePayment{FormaPago: p.Method, Total: p.Amount.StringFixed(2)}
		if p.TermDays > 0 {
			pay.Plazo, pay.UnidadTiempo = strconv.Itoa(p.TermDays), termUnitDays
		}
		doc.InfoFactura.Pagos = append(doc.InfoFactura.Pagos, pay)
	}
	for i, l := range inv.Lines {
		doc.Detalles = append(doc.Detalles, xmlInvoiceLine{
			CodigoPrincipal: lineCode(l.Code), CodigoAuxiliar: xmlText(l.AuxCode, maxCodeLen),
			Descripcion: xmlText(l.Description, maxTextLen),
			Cantidad:    l.Quantity.StringFixed(6), PrecioUnitario: l.UnitPrice.StringFixed(6),
			Descuento: l.Discount.StringFixed(2), PrecioTotalSinImpuesto: totals.Lines[i].Subtotal.StringFixed(2),
			Impuestos: lineTax(l, totals.Lines[i]),
		})
	}
	doc.InfoAdicional = infoAdicional(inv.Additional)
	out, err := marshalDocument(doc)
	if err != nil {
		return Built{}, err
	}
	return Built{XML: out, AccessKey: key}, nil
}

// taxTotal is one VAT total of an invoice or credit note.
type taxTotal struct {
	Codigo, CodigoPorcentaje, BaseImponible, Tarifa, Valor string
}

// taxTotals returns one VAT total per rate with a positive base, 0 % first.
func taxTotals(t Totals) []taxTotal {
	var out []taxTotal
	if t.Subtotal0.IsPositive() {
		out = append(out, taxTotal{vatTaxCode, "0", t.Subtotal0.StringFixed(2), "0", decimal.Zero.StringFixed(2)})
	}
	if t.Subtotal15.IsPositive() {
		out = append(out, taxTotal{vatTaxCode, "4", t.Subtotal15.StringFixed(2), "15", t.IVA.StringFixed(2)})
	}
	return out
}
