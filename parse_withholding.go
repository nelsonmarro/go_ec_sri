package sri

import (
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// readWithholding is what ParseWithholding reads: version 2.0.0 groups the
// withholdings by support document (docsSustento); version 1.0.0 lists taxes
// that each name their support document (impuestos).
type readWithholding struct {
	XMLName           xml.Name
	InfoTributaria    *xmlInfoTributaria  `xml:"infoTributaria"`
	InfoCompRetencion *xmlWithholdingInfo `xml:"infoCompRetencion"`
	DocsSustento      []xmlDocSustento    `xml:"docsSustento>docSustento"`
	Impuestos         []readTaxV100       `xml:"impuestos>impuesto"`
	InfoAdicional     *xmlInfoAdicional   `xml:"infoAdicional"`
}

// readTaxV100 is a version 1.0.0 <impuesto>: one withholding and its support
// document.
type readTaxV100 struct {
	xmlRetencion
	CodDocSustento          string `xml:"codDocSustento"`
	NumDocSustento          string `xml:"numDocSustento"`
	FechaEmisionDocSustento string `xml:"fechaEmisionDocSustento"`
}

const errSeveralSupports = "la retención tiene varios documentos de sustento y solo se lee uno"

// ParseWithholding reads a withholding certificate (comprobante de retención
// 07, versions 2.0.0 and 1.0.0). The same rules as ParseInvoice apply.
//
// Withholding models one support document, so this version reads a
// certificate on a single document: one on several documents (a customer
// withholding several invoices at once) is an error. The support document's
// taxes must be VAT 0 % or 15 %, and each withholding renta or IVA. A 1.0.0
// support document carries no sustento code, authorization, totals or
// payments, and its date is optional, so those come back empty.
func ParseWithholding(data []byte) (*ParsedWithholding, error) {
	w, info, err := parseWithholding(data)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la retención: %w", err)
	}
	return &ParsedWithholding{Withholding: w, Authorization: info}, nil
}

// ParsedWithholding is a withholding certificate read from its XML. Its
// amounts are the ones printed: the support document's totals and each
// line's withheld value.
type ParsedWithholding struct {
	Withholding Withholding
	// Authorization is nil for a bare receipt.
	Authorization *AuthorizationInfo
}

func parseWithholding(data []byte) (Withholding, *AuthorizationInfo, error) {
	var doc readWithholding
	env, err := decodeReceipt(data, &doc)
	if err != nil {
		return Withholding{}, nil, err
	}
	const wrongType = "el XML no es un comprobante de retención (07)"
	if doc.XMLName.Local != "comprobanteRetencion" {
		return Withholding{}, nil, errors.New(wrongType)
	}
	r, err := readTributaria(doc.InfoTributaria, DocWithholding, wrongType)
	if err != nil {
		return Withholding{}, nil, err
	}
	wi := doc.InfoCompRetencion
	if wi == nil {
		return Withholding{}, nil, errors.New("falta infoCompRetencion")
	}
	w := Withholding{Header: r.header, Issuer: r.issuer, Additional: readAdditional(doc.InfoAdicional)}
	if w.Header.IssueDate, err = readDate(wi.FechaEmision); err != nil {
		return Withholding{}, nil, err
	}
	if w.FiscalPeriod, err = readPeriod(wi.PeriodoFiscal); err != nil {
		return Withholding{}, nil, err
	}
	w.Issuer.EstablishmentAddress = trim(wi.DirEstablecimiento)
	w.Issuer.SpecialTaxpayerNumber = trim(wi.ContribuyenteEspecial)
	w.Issuer.KeepsAccounting = trim(wi.ObligadoContabilidad) == "SI"
	w.Supplier = Party{IDType: IDType(trim(wi.TipoIdentificacionSujetoRetenido)), ID: trim(wi.IdentificacionSujetoRetenido),
		Name: trim(wi.RazonSocialSujetoRetenido), RelatedParty: trim(wi.ParteRel) == "SI"}
	switch {
	case len(doc.DocsSustento) > 1:
		return Withholding{}, nil, errors.New(errSeveralSupports)
	case len(doc.DocsSustento) == 1:
		w.Support, w.Lines, err = readSupportV200(doc.DocsSustento[0])
	case len(doc.Impuestos) > 0:
		w.Support, w.Lines, err = readSupportV100(doc.Impuestos)
	default:
		err = errors.New("la retención no tiene documento de sustento")
	}
	if err != nil {
		return Withholding{}, nil, err
	}
	info, err := env.info(r)
	if err != nil {
		return Withholding{}, nil, err
	}
	return w, info, nil
}

// readSupportV200 reads a 2.0.0 docSustento: its VAT at 0 % or 15 % (any other
// tax is an error), totals, payments and withholdings.
func readSupportV200(d xmlDocSustento) (SupportDocument, []WithholdingLine, error) {
	s := SupportDocument{DocType: DocType(trim(d.CodDocSustento)), Number: readDocumentNumber(d.NumDocSustento),
		Authorization: trim(d.NumAutDocSustento), SustentoCode: trim(d.CodSustento)}
	var err error
	if s.IssueDate, err = readDate(d.FechaEmisionDocSustento); err != nil {
		return SupportDocument{}, nil, err
	}
	if s.Total, err = readAmount(d.ImporteTotal); err != nil {
		return SupportDocument{}, nil, err
	}
	for _, t := range d.Impuestos {
		base, err := readAmount(t.BaseImponible)
		if err != nil {
			return SupportDocument{}, nil, err
		}
		value, err := readAmount(t.ValorImpuesto)
		if err != nil {
			return SupportDocument{}, nil, err
		}
		rate, ok := vatRate(t.CodImpuestoDocSustento, t.CodigoPorcentaje)
		switch {
		case !ok:
			return SupportDocument{}, nil, otherTax("el documento de sustento", t.CodImpuestoDocSustento, t.CodigoPorcentaje)
		case rate == 15:
			s.Subtotal15, s.IVA = s.Subtotal15.Add(base), s.IVA.Add(value)
		default:
			s.Subtotal0 = s.Subtotal0.Add(base)
		}
	}
	for _, p := range d.Pagos {
		amount, err := readAmount(p.Total)
		if err != nil {
			return SupportDocument{}, nil, err
		}
		s.Payments = append(s.Payments, Payment{Method: trim(p.FormaPago), Amount: amount})
	}
	var lines []WithholdingLine
	for _, r := range d.Retenciones {
		l, err := readWithholdingLine(r)
		if err != nil {
			return SupportDocument{}, nil, err
		}
		lines = append(lines, l)
	}
	return s, lines, nil
}

// readSupportV100 reads the 1.0.0 taxes, which must all name the same support
// document.
func readSupportV100(taxes []readTaxV100) (SupportDocument, []WithholdingLine, error) {
	first := taxes[0]
	s := SupportDocument{DocType: DocType(trim(first.CodDocSustento)), Number: readDocumentNumber(first.NumDocSustento)}
	// The date is optional in 1.0.0: an empty one stays zero.
	if date := trim(first.FechaEmisionDocSustento); date != "" {
		var err error
		if s.IssueDate, err = readDate(date); err != nil {
			return SupportDocument{}, nil, err
		}
	}
	lines := make([]WithholdingLine, 0, len(taxes))
	for _, t := range taxes {
		if trim(t.CodDocSustento) != string(s.DocType) || readDocumentNumber(t.NumDocSustento) != s.Number {
			return SupportDocument{}, nil, errors.New(errSeveralSupports)
		}
		l, err := readWithholdingLine(t.xmlRetencion)
		if err != nil {
			return SupportDocument{}, nil, err
		}
		lines = append(lines, l)
	}
	return s, lines, nil
}

func readWithholdingLine(r xmlRetencion) (WithholdingLine, error) {
	l := WithholdingLine{Code: trim(r.CodigoRetencion)}
	switch trim(r.Codigo) {
	case "1":
		l.Tax = WithholdingRenta
	case "2":
		l.Tax = WithholdingIVA
	default:
		return WithholdingLine{}, fmt.Errorf("el impuesto retenido no es renta ni IVA: %q", trim(r.Codigo))
	}
	var err error
	if l.Base, err = readAmount(r.BaseImponible); err != nil {
		return WithholdingLine{}, err
	}
	if l.Percentage, err = readAmount(r.PorcentajeRetener); err != nil {
		return WithholdingLine{}, err
	}
	if l.Amount, err = readAmount(r.ValorRetenido); err != nil {
		return WithholdingLine{}, err
	}
	return l, nil
}

var fifteenDigits = regexp.MustCompile(`^\d{15}$`)

// readDocumentNumber prints a 15-digit document number as 001-001-000000123;
// any other text is kept as written.
func readDocumentNumber(s string) string {
	s = trim(s)
	if fifteenDigits.MatchString(s) {
		return s[0:3] + "-" + s[3:6] + "-" + s[6:]
	}
	return s
}

// readPeriod reads a mm/aaaa fiscal period as the first day of the month in
// Ecuador's time.
func readPeriod(s string) (time.Time, error) {
	t, err := time.ParseInLocation(periodLayout, trim(s), ecuadorTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("el período fiscal no es válido: %q", trim(s))
	}
	return t, nil
}
