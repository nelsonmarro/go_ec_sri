package sri

import (
	"strings"

	"github.com/shopspring/decimal"
)

// WithholdingXML validates the withholding and builds its unsigned XML
// (comprobanteRetencion, schema 2.0.0) with one support document. Invalid
// input returns the ValidationErrors of Validate and no XML. When
// Header.AccessKey is empty a key is generated with a random numeric code
// (crypto/rand).
//
// The support document's amounts and payments are printed as given (Validate
// checks that they add up); parteRel is SI when Supplier.RelatedParty is set
// and NO otherwise, and the payment is local (pagoLocExt 01). The fiscal
// period is the month of FiscalPeriod. Texts follow the same rules as
// InvoiceXML.
func WithholdingXML(w Withholding) (Built, error) {
	if err := w.Validate(); err != nil {
		return Built{}, err
	}
	key, err := accessKey(w.Header, DocWithholding, w.Issuer.RUC)
	if err != nil {
		return Built{}, err
	}
	doc := xmlWithholding{
		ID: documentID, Version: "2.0.0",
		InfoTributaria: infoTributaria(w.Issuer, w.Header, DocWithholding, key),
		InfoCompRetencion: xmlWithholdingInfo{
			FechaEmision:                     w.Header.IssueDate.Format(dateLayout),
			DirEstablecimiento:               xmlText(w.Issuer.EstablishmentAddress, maxTextLen),
			ContribuyenteEspecial:            w.Issuer.SpecialTaxpayerNumber,
			ObligadoContabilidad:             siNo(w.Issuer.KeepsAccounting),
			TipoIdentificacionSujetoRetenido: string(w.Supplier.IDType),
			ParteRel:                         siNo(w.Supplier.RelatedParty),
			RazonSocialSujetoRetenido:        xmlText(w.Supplier.Name, maxTextLen),
			IdentificacionSujetoRetenido:     w.Supplier.ID,
			PeriodoFiscal:                    w.FiscalPeriod.Format(periodLayout),
		},
		DocsSustento: []xmlDocSustento{supportXML(w.Support, w.Lines)},
	}
	doc.InfoAdicional = infoAdicional(w.Additional)
	out, err := marshalDocument(doc)
	if err != nil {
		return Built{}, err
	}
	return Built{XML: out, AccessKey: key}, nil
}

// supportXML is the docSustento with its taxes, withholdings and payments.
// Validate guarantees that Subtotal0+Subtotal15 is positive and that VAT comes
// with a 15 % base, so impuestosDocSustento is never empty.
func supportXML(s SupportDocument, lines []WithholdingLine) xmlDocSustento {
	d := xmlDocSustento{
		CodSustento:             s.SustentoCode,
		CodDocSustento:          string(s.DocType),
		NumDocSustento:          strings.ReplaceAll(s.Number, "-", ""),
		FechaEmisionDocSustento: s.IssueDate.Format(dateLayout),
		NumAutDocSustento:       s.Authorization,
		PagoLocExt:              "01",
		TotalSinImpuestos:       s.Subtotal0.Add(s.Subtotal15).StringFixed(2),
		ImporteTotal:            s.Total.StringFixed(2),
	}
	if s.Subtotal0.IsPositive() {
		d.Impuestos = append(d.Impuestos, xmlSupportTax{CodImpuestoDocSustento: vatTaxCode, CodigoPorcentaje: "0",
			BaseImponible: s.Subtotal0.StringFixed(2), Tarifa: "0", ValorImpuesto: decimal.Zero.StringFixed(2)})
	}
	if s.Subtotal15.IsPositive() {
		d.Impuestos = append(d.Impuestos, xmlSupportTax{CodImpuestoDocSustento: vatTaxCode, CodigoPorcentaje: "4",
			BaseImponible: s.Subtotal15.StringFixed(2), Tarifa: "15", ValorImpuesto: s.IVA.StringFixed(2)})
	}
	for _, l := range lines {
		codigo := "1"
		if l.Tax == WithholdingIVA {
			codigo = "2"
		}
		d.Retenciones = append(d.Retenciones, xmlRetencion{
			Codigo: codigo, CodigoRetencion: normalizeText(l.Code),
			BaseImponible:     l.Base.StringFixed(2),
			PorcentajeRetener: l.Percentage.String(),
			ValorRetenido:     l.Amount.StringFixed(2),
		})
	}
	for _, p := range s.Payments {
		d.Pagos = append(d.Pagos, xmlSupportPayment{FormaPago: p.Method, Total: p.Amount.StringFixed(2)})
	}
	return d
}
