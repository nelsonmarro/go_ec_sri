package sri

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"strconv"
)

// Built is a document's unsigned XML and its access key.
type Built struct {
	// XML is the document, UTF-8 with its XML declaration, ready to sign.
	XML []byte
	// AccessKey is the header's access key, or the one generated for it.
	AccessKey string
}

// Fixed values of the schemas.
const (
	documentID     = "comprobante"
	xmlDeclaration = `<?xml version="1.0" encoding="UTF-8"?>`
	// rimpeLegend is the only text the schemas accept in contribuyenteRimpe.
	rimpeLegend = "CONTRIBUYENTE RÉGIMEN RIMPE"
	// finalConsumerName is the name the SRI expects for a final consumer.
	finalConsumerName = "CONSUMIDOR FINAL"
	vatTaxCode        = "2" // codigo of the VAT in the totals and the lines
	dateLayout        = "02/01/2006"
	periodLayout      = "01/2006"
	currency          = "DOLAR"
	termUnitDays      = "dias"
)

// xmlInfoTributaria is the infoTributaria block every document shares.
type xmlInfoTributaria struct {
	Ambiente           string `xml:"ambiente"`
	TipoEmision        string `xml:"tipoEmision"`
	RazonSocial        string `xml:"razonSocial"`
	NombreComercial    string `xml:"nombreComercial,omitempty"`
	Ruc                string `xml:"ruc"`
	ClaveAcceso        string `xml:"claveAcceso"`
	CodDoc             string `xml:"codDoc"`
	Estab              string `xml:"estab"`
	PtoEmi             string `xml:"ptoEmi"`
	Secuencial         string `xml:"secuencial"`
	DirMatriz          string `xml:"dirMatriz"`
	AgenteRetencion    string `xml:"agenteRetencion,omitempty"`
	ContribuyenteRimpe string `xml:"contribuyenteRimpe,omitempty"`
}

// xmlInfoAdicional is the optional infoAdicional block (up to 15 fields).
type xmlInfoAdicional struct {
	Campos []xmlCampoAdicional `xml:"campoAdicional"`
}

type xmlCampoAdicional struct {
	Nombre string `xml:"nombre,attr"`
	Valor  string `xml:",chardata"`
}

// xmlInvoice is the invoice (01), schema version 1.1.0.
type xmlInvoice struct {
	XMLName        xml.Name          `xml:"factura"`
	ID             string            `xml:"id,attr"`
	Version        string            `xml:"version,attr"`
	InfoTributaria xmlInfoTributaria `xml:"infoTributaria"`
	InfoFactura    xmlInvoiceInfo    `xml:"infoFactura"`
	Detalles       []xmlInvoiceLine  `xml:"detalles>detalle"`
	InfoAdicional  *xmlInfoAdicional `xml:"infoAdicional,omitempty"`
}

type xmlInvoiceInfo struct {
	FechaEmision                string               `xml:"fechaEmision"`
	DirEstablecimiento          string               `xml:"dirEstablecimiento,omitempty"`
	ContribuyenteEspecial       string               `xml:"contribuyenteEspecial,omitempty"`
	ObligadoContabilidad        string               `xml:"obligadoContabilidad"`
	TipoIdentificacionComprador string               `xml:"tipoIdentificacionComprador"`
	RazonSocialComprador        string               `xml:"razonSocialComprador"`
	IdentificacionComprador     string               `xml:"identificacionComprador"`
	DireccionComprador          string               `xml:"direccionComprador,omitempty"`
	TotalSinImpuestos           string               `xml:"totalSinImpuestos"`
	TotalDescuento              string               `xml:"totalDescuento"`
	TotalConImpuestos           []xmlInvoiceTotalTax `xml:"totalConImpuestos>totalImpuesto"`
	Propina                     string               `xml:"propina"`
	ImporteTotal                string               `xml:"importeTotal"`
	Moneda                      string               `xml:"moneda"`
	Pagos                       []xmlInvoicePayment  `xml:"pagos>pago"`
}

type xmlInvoiceTotalTax struct {
	Codigo           string `xml:"codigo"`
	CodigoPorcentaje string `xml:"codigoPorcentaje"`
	BaseImponible    string `xml:"baseImponible"`
	Tarifa           string `xml:"tarifa"`
	Valor            string `xml:"valor"`
}

type xmlInvoicePayment struct {
	FormaPago    string `xml:"formaPago"`
	Total        string `xml:"total"`
	Plazo        string `xml:"plazo,omitempty"`
	UnidadTiempo string `xml:"unidadTiempo,omitempty"`
}

type xmlInvoiceLine struct {
	CodigoPrincipal        string       `xml:"codigoPrincipal"`
	CodigoAuxiliar         string       `xml:"codigoAuxiliar,omitempty"`
	Descripcion            string       `xml:"descripcion"`
	Cantidad               string       `xml:"cantidad"`
	PrecioUnitario         string       `xml:"precioUnitario"`
	Descuento              string       `xml:"descuento"`
	PrecioTotalSinImpuesto string       `xml:"precioTotalSinImpuesto"`
	Impuestos              []xmlLineTax `xml:"impuestos>impuesto"`
}

// xmlLineTax is the VAT of one line, the same in the invoice and the credit note.
type xmlLineTax struct {
	Codigo           string `xml:"codigo"`
	CodigoPorcentaje string `xml:"codigoPorcentaje"`
	Tarifa           string `xml:"tarifa"`
	BaseImponible    string `xml:"baseImponible"`
	Valor            string `xml:"valor"`
}

// xmlCreditNote is the credit note (04), schema version 1.1.0. Its element
// order differs from the invoice.
type xmlCreditNote struct {
	XMLName         xml.Name            `xml:"notaCredito"`
	ID              string              `xml:"id,attr"`
	Version         string              `xml:"version,attr"`
	InfoTributaria  xmlInfoTributaria   `xml:"infoTributaria"`
	InfoNotaCredito xmlCreditNoteInfo   `xml:"infoNotaCredito"`
	Detalles        []xmlCreditNoteLine `xml:"detalles>detalle"`
	InfoAdicional   *xmlInfoAdicional   `xml:"infoAdicional,omitempty"`
}

type xmlCreditNoteInfo struct {
	FechaEmision                string                  `xml:"fechaEmision"`
	DirEstablecimiento          string                  `xml:"dirEstablecimiento,omitempty"`
	TipoIdentificacionComprador string                  `xml:"tipoIdentificacionComprador"`
	RazonSocialComprador        string                  `xml:"razonSocialComprador"`
	IdentificacionComprador     string                  `xml:"identificacionComprador"`
	ContribuyenteEspecial       string                  `xml:"contribuyenteEspecial,omitempty"`
	ObligadoContabilidad        string                  `xml:"obligadoContabilidad"`
	CodDocModificado            string                  `xml:"codDocModificado"`
	NumDocModificado            string                  `xml:"numDocModificado"`
	FechaEmisionDocSustento     string                  `xml:"fechaEmisionDocSustento"`
	TotalSinImpuestos           string                  `xml:"totalSinImpuestos"`
	ValorModificacion           string                  `xml:"valorModificacion"`
	Moneda                      string                  `xml:"moneda"`
	TotalConImpuestos           []xmlCreditNoteTotalTax `xml:"totalConImpuestos>totalImpuesto"`
	Motivo                      string                  `xml:"motivo"`
}

// xmlCreditNoteTotalTax has no tarifa: the schema does not allow it here.
type xmlCreditNoteTotalTax struct {
	Codigo           string `xml:"codigo"`
	CodigoPorcentaje string `xml:"codigoPorcentaje"`
	BaseImponible    string `xml:"baseImponible"`
	Valor            string `xml:"valor"`
}

type xmlCreditNoteLine struct {
	CodigoInterno          string       `xml:"codigoInterno"`
	CodigoAdicional        string       `xml:"codigoAdicional,omitempty"`
	Descripcion            string       `xml:"descripcion"`
	Cantidad               string       `xml:"cantidad"`
	PrecioUnitario         string       `xml:"precioUnitario"`
	Descuento              string       `xml:"descuento"`
	PrecioTotalSinImpuesto string       `xml:"precioTotalSinImpuesto"`
	Impuestos              []xmlLineTax `xml:"impuestos>impuesto"`
}

// xmlWithholding is the withholding certificate (07), schema version 2.0.0.
type xmlWithholding struct {
	XMLName           xml.Name           `xml:"comprobanteRetencion"`
	ID                string             `xml:"id,attr"`
	Version           string             `xml:"version,attr"`
	InfoTributaria    xmlInfoTributaria  `xml:"infoTributaria"`
	InfoCompRetencion xmlWithholdingInfo `xml:"infoCompRetencion"`
	DocsSustento      []xmlDocSustento   `xml:"docsSustento>docSustento"`
	InfoAdicional     *xmlInfoAdicional  `xml:"infoAdicional,omitempty"`
}

type xmlWithholdingInfo struct {
	FechaEmision                     string `xml:"fechaEmision"`
	DirEstablecimiento               string `xml:"dirEstablecimiento,omitempty"`
	ContribuyenteEspecial            string `xml:"contribuyenteEspecial,omitempty"`
	ObligadoContabilidad             string `xml:"obligadoContabilidad"`
	TipoIdentificacionSujetoRetenido string `xml:"tipoIdentificacionSujetoRetenido"`
	ParteRel                         string `xml:"parteRel"`
	RazonSocialSujetoRetenido        string `xml:"razonSocialSujetoRetenido"`
	IdentificacionSujetoRetenido     string `xml:"identificacionSujetoRetenido"`
	PeriodoFiscal                    string `xml:"periodoFiscal"`
}

type xmlDocSustento struct {
	CodSustento             string              `xml:"codSustento"`
	CodDocSustento          string              `xml:"codDocSustento"`
	NumDocSustento          string              `xml:"numDocSustento"`
	FechaEmisionDocSustento string              `xml:"fechaEmisionDocSustento"`
	NumAutDocSustento       string              `xml:"numAutDocSustento,omitempty"`
	PagoLocExt              string              `xml:"pagoLocExt"`
	TotalSinImpuestos       string              `xml:"totalSinImpuestos"`
	ImporteTotal            string              `xml:"importeTotal"`
	Impuestos               []xmlSupportTax     `xml:"impuestosDocSustento>impuestoDocSustento"`
	Retenciones             []xmlRetencion      `xml:"retenciones>retencion"`
	Pagos                   []xmlSupportPayment `xml:"pagos>pago"`
}

type xmlSupportTax struct {
	CodImpuestoDocSustento string `xml:"codImpuestoDocSustento"`
	CodigoPorcentaje       string `xml:"codigoPorcentaje"`
	BaseImponible          string `xml:"baseImponible"`
	Tarifa                 string `xml:"tarifa"`
	ValorImpuesto          string `xml:"valorImpuesto"`
}

type xmlRetencion struct {
	Codigo            string `xml:"codigo"`
	CodigoRetencion   string `xml:"codigoRetencion"`
	BaseImponible     string `xml:"baseImponible"`
	PorcentajeRetener string `xml:"porcentajeRetener"`
	ValorRetenido     string `xml:"valorRetenido"`
}

type xmlSupportPayment struct {
	FormaPago string `xml:"formaPago"`
	Total     string `xml:"total"`
}

// infoTributaria fills the shared block; key is the access key to print.
func infoTributaria(is Issuer, h Header, doc DocType, key string) xmlInfoTributaria {
	it := xmlInfoTributaria{
		Ambiente:        strconv.Itoa(int(h.Environment)),
		TipoEmision:     emissionTypeCode,
		RazonSocial:     xmlText(is.BusinessName, maxTextLen),
		NombreComercial: xmlText(is.TradeName, maxTextLen),
		Ruc:             is.RUC,
		ClaveAcceso:     key,
		CodDoc:          string(doc),
		Estab:           h.Establishment,
		PtoEmi:          h.EmissionPoint,
		Secuencial:      paddedSequential(h.Sequential),
		DirMatriz:       xmlText(is.MainAddress, maxTextLen),
		AgenteRetencion: is.WithholdingAgentResolution,
	}
	if is.Rimpe != RimpeNone {
		it.ContribuyenteRimpe = rimpeLegend
	}
	return it
}

// accessKey returns the header's access key or, when it is empty, a new one
// with a random numeric code from crypto/rand.
func accessKey(h Header, doc DocType, ruc string) (string, error) {
	if h.AccessKey != "" {
		return h.AccessKey, nil
	}
	return GenerateAccessKey(h.IssueDate, doc, ruc, h.Environment, h.Establishment, h.EmissionPoint, paddedSequential(h.Sequential), rand.Reader)
}

// siNo is the SI/NO value of obligadoContabilidad and parteRel.
func siNo(b bool) string {
	if b {
		return "SI"
	}
	return "NO"
}

// infoAdicional returns the additional fields, or nil when there are none so
// the block is left out.
func infoAdicional(fields []AdditionalField) *xmlInfoAdicional {
	if len(fields) == 0 {
		return nil
	}
	out := &xmlInfoAdicional{Campos: make([]xmlCampoAdicional, len(fields))}
	for i, f := range fields {
		out.Campos[i] = xmlCampoAdicional{Nombre: xmlText(f.Name, maxTextLen), Valor: xmlText(f.Value, maxTextLen)}
	}
	return out
}

// buyerName is the razonSocialComprador: the fixed name for a final consumer,
// the normalized name otherwise.
func buyerName(p Party) string {
	if p.IDType == IDFinalConsumer {
		return finalConsumerName
	}
	return xmlText(p.Name, maxTextLen)
}

// lineCode is an item code capped at 25 characters, or GEN when it is empty.
func lineCode(code string) string {
	if c := xmlText(code, maxCodeLen); c != "" {
		return c
	}
	return "GEN"
}

// lineTax is the VAT element of a line with its computed totals.
func lineTax(l Line, t LineTotals) []xmlLineTax {
	pct, rate := "0", "0"
	if l.TaxRate == 15 {
		pct, rate = "4", "15"
	}
	return []xmlLineTax{{Codigo: vatTaxCode, CodigoPorcentaje: pct, Tarifa: rate,
		BaseImponible: t.Subtotal.StringFixed(2), Valor: t.IVA.StringFixed(2)}}
}

// marshalDocument serializes doc after the XML declaration.
func marshalDocument(doc any) ([]byte, error) {
	out, err := xml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serializar el comprobante: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(xmlDeclaration)
	buf.Write(out)
	return buf.Bytes(), nil
}
