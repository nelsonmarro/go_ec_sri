package sri

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

// Reading the authorized XML: ParseInvoice, ParseCreditNote and
// ParseWithholding accept the bare receipt or the <autorizacion> envelope (the
// receipt in CDATA or as escaped text, at any nesting, as the SRI answers it),
// in UTF-8 or ISO-8859-1. They only turn the XML into the library types: they
// do not validate the document (call Validate for that) and they do not check
// the signature.

// readInvoice is what ParseInvoice reads; the root element is checked apart so
// a wrong document gets a clear error. Unknown elements are ignored.
type readInvoice struct {
	XMLName        xml.Name
	InfoTributaria *xmlInfoTributaria `xml:"infoTributaria"`
	InfoFactura    *xmlInvoiceInfo    `xml:"infoFactura"`
	Detalles       []xmlInvoiceLine   `xml:"detalles>detalle"`
	InfoAdicional  *xmlInfoAdicional  `xml:"infoAdicional"`
}

type readCreditNote struct {
	XMLName         xml.Name
	InfoTributaria  *xmlInfoTributaria  `xml:"infoTributaria"`
	InfoNotaCredito *xmlCreditNoteInfo  `xml:"infoNotaCredito"`
	Detalles        []xmlCreditNoteLine `xml:"detalles>detalle"`
	InfoAdicional   *xmlInfoAdicional   `xml:"infoAdicional"`
}

// ParsedInvoice is an invoice read from its XML.
type ParsedInvoice struct {
	Invoice Invoice
	// Totals are the amounts printed in the XML, the ones the SRI authorized,
	// not a recomputation from the lines.
	Totals Totals
	// Authorization is nil for a bare receipt.
	Authorization *AuthorizationInfo
}

// ParseInvoice reads an invoice (factura 01, versions 1.0.0 and 1.1.0) from
// the bare receipt or its <autorizacion> envelope; Authorization is nil for a
// bare receipt. An envelope must be authorized: entries whose estado is not
// AUTORIZADO are skipped and, when none is, it is an error.
//
// Only VAT 0 % and 15 % are read: any other tax (ICE, IRBPNR, another VAT
// rate) is an error. Some data cannot come back as it was built:
//   - calendar dates come back at midnight in Ecuador's time;
//   - the buyer's email and phone are not in the XML (they stay in
//     Additional when the issuer wrote them there);
//   - the schema has one RIMPE legend for both regimes, so it reads as
//     RimpeEntrepreneur (an older negocio popular legend reads as
//     RimpePopularBusiness);
//   - a payment term in months counts 30 days per month;
//   - amounts keep the scale written in the XML (2.000000 equals 2).
//
// Errors start with a Spanish sentence; the XML decoder's own detail, in
// English, may follow in parentheses.
func ParseInvoice(data []byte) (*ParsedInvoice, error) {
	p, err := parseInvoice(data)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la factura: %w", err)
	}
	return p, nil
}

func parseInvoice(data []byte) (*ParsedInvoice, error) {
	var doc readInvoice
	env, err := decodeReceipt(data, &doc)
	if err != nil {
		return nil, err
	}
	if doc.XMLName.Local != "factura" {
		return nil, errors.New("el XML no es una factura (01)")
	}
	r, err := readTributaria(doc.InfoTributaria, DocInvoice, "el XML no es una factura (01)")
	if err != nil {
		return nil, err
	}
	fi := doc.InfoFactura
	if fi == nil {
		return nil, errors.New("falta infoFactura")
	}
	inv := Invoice{Header: r.header, Issuer: r.issuer, Additional: readAdditional(doc.InfoAdicional)}
	if inv.Header.IssueDate, err = readDate(fi.FechaEmision); err != nil {
		return nil, err
	}
	var totals Totals
	taxes := make([]taxTotal, len(fi.TotalConImpuestos))
	for i, t := range fi.TotalConImpuestos {
		taxes[i] = taxTotal(t)
	}
	if err := readTaxTotals(&totals, taxes); err != nil {
		return nil, err
	}
	if totals.Discount, err = readAmount(fi.TotalDescuento); err != nil {
		return nil, err
	}
	if totals.Total, err = readAmount(fi.ImporteTotal); err != nil {
		return nil, err
	}
	inv.Issuer.EstablishmentAddress = trim(fi.DirEstablecimiento)
	inv.Issuer.SpecialTaxpayerNumber = trim(fi.ContribuyenteEspecial)
	inv.Issuer.KeepsAccounting = trim(fi.ObligadoContabilidad) == "SI"
	inv.Buyer = Party{IDType: IDType(trim(fi.TipoIdentificacionComprador)), ID: trim(fi.IdentificacionComprador),
		Name: trim(fi.RazonSocialComprador), Address: trim(fi.DireccionComprador)}
	for _, d := range doc.Detalles {
		l, lt, err := readLine(d.CodigoPrincipal, d.CodigoAuxiliar, d.Descripcion, d.Cantidad, d.PrecioUnitario, d.Descuento, d.PrecioTotalSinImpuesto, d.Impuestos)
		if err != nil {
			return nil, err
		}
		inv.Lines = append(inv.Lines, l)
		totals.Lines = append(totals.Lines, lt)
	}
	for _, p := range fi.Pagos {
		pay, err := readPayment(p)
		if err != nil {
			return nil, err
		}
		inv.Payments = append(inv.Payments, pay)
	}
	info, err := env.info(r)
	if err != nil {
		return nil, err
	}
	return &ParsedInvoice{Invoice: inv, Totals: totals, Authorization: info}, nil
}

// ParseCreditNote reads a credit note (nota de crédito 04, versions 1.0.0 and
// 1.1.0) that modifies an invoice. The same rules as ParseInvoice apply; the
// credit note prints no total discount, so Totals.Discount is the sum of the
// printed line discounts.
func ParseCreditNote(data []byte) (*ParsedCreditNote, error) {
	p, err := parseCreditNote(data)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la nota de crédito: %w", err)
	}
	return p, nil
}

// ParsedCreditNote is a credit note read from its XML.
type ParsedCreditNote struct {
	CreditNote CreditNote
	// Totals are the amounts printed in the XML (see ParsedInvoice).
	Totals Totals
	// Authorization is nil for a bare receipt.
	Authorization *AuthorizationInfo
}

func parseCreditNote(data []byte) (*ParsedCreditNote, error) {
	var doc readCreditNote
	env, err := decodeReceipt(data, &doc)
	if err != nil {
		return nil, err
	}
	if doc.XMLName.Local != "notaCredito" {
		return nil, errors.New("el XML no es una nota de crédito (04)")
	}
	r, err := readTributaria(doc.InfoTributaria, DocCreditNote, "el XML no es una nota de crédito (04)")
	if err != nil {
		return nil, err
	}
	ni := doc.InfoNotaCredito
	if ni == nil {
		return nil, errors.New("falta infoNotaCredito")
	}
	if trim(ni.CodDocModificado) != string(DocInvoice) {
		return nil, errors.New("la nota de crédito no modifica una factura (01)")
	}
	cn := CreditNote{Header: r.header, Issuer: r.issuer, Additional: readAdditional(doc.InfoAdicional),
		ModifiedNumber: trim(ni.NumDocModificado), Reason: trim(ni.Motivo)}
	if cn.Header.IssueDate, err = readDate(ni.FechaEmision); err != nil {
		return nil, err
	}
	if cn.ModifiedDate, err = readDate(ni.FechaEmisionDocSustento); err != nil {
		return nil, err
	}
	var totals Totals
	taxes := make([]taxTotal, len(ni.TotalConImpuestos))
	for i, t := range ni.TotalConImpuestos {
		taxes[i] = taxTotal{Codigo: t.Codigo, CodigoPorcentaje: t.CodigoPorcentaje, BaseImponible: t.BaseImponible, Valor: t.Valor}
	}
	if err := readTaxTotals(&totals, taxes); err != nil {
		return nil, err
	}
	if totals.Total, err = readAmount(ni.ValorModificacion); err != nil {
		return nil, err
	}
	cn.Issuer.EstablishmentAddress = trim(ni.DirEstablecimiento)
	cn.Issuer.SpecialTaxpayerNumber = trim(ni.ContribuyenteEspecial)
	cn.Issuer.KeepsAccounting = trim(ni.ObligadoContabilidad) == "SI"
	cn.Buyer = Party{IDType: IDType(trim(ni.TipoIdentificacionComprador)), ID: trim(ni.IdentificacionComprador),
		Name: trim(ni.RazonSocialComprador)}
	for _, d := range doc.Detalles {
		l, lt, err := readLine(d.CodigoInterno, d.CodigoAdicional, d.Descripcion, d.Cantidad, d.PrecioUnitario, d.Descuento, d.PrecioTotalSinImpuesto, d.Impuestos)
		if err != nil {
			return nil, err
		}
		cn.Lines = append(cn.Lines, l)
		totals.Lines = append(totals.Lines, lt)
		// The credit note prints no total discount: it is the sum of the
		// printed line discounts.
		totals.Discount = totals.Discount.Add(l.Discount)
	}
	info, err := env.info(r)
	if err != nil {
		return nil, err
	}
	return &ParsedCreditNote{CreditNote: cn, Totals: totals, Authorization: info}, nil
}

// tributaria is what infoTributaria gives: the header (without the issue
// date, which lives in the info block), the issuer and the access key fields.
type tributaria struct {
	header Header
	issuer Issuer
	parts  AccessKeyParts
}

// readTributaria reads infoTributaria and checks that it is a doc and that
// its access key is valid and belongs to it (same RUC and type).
func readTributaria(it *xmlInfoTributaria, doc DocType, wrongType string) (tributaria, error) {
	if it == nil || trim(it.CodDoc) != string(doc) {
		return tributaria{}, errors.New(wrongType)
	}
	key := trim(it.ClaveAcceso)
	parts, err := ParseAccessKey(key)
	if err != nil {
		return tributaria{}, err
	}
	ruc := trim(it.Ruc)
	if parts.RUC != ruc || parts.Doc != doc {
		return tributaria{}, errors.New("la clave de acceso no corresponde al comprobante")
	}
	var env Environment
	switch trim(it.Ambiente) {
	case "1":
		env = EnvironmentTest
	case "2":
		env = EnvironmentProduction
	default:
		return tributaria{}, fmt.Errorf("el ambiente no es válido: %q", trim(it.Ambiente))
	}
	return tributaria{
		header: Header{Environment: env, Establishment: trim(it.Estab), EmissionPoint: trim(it.PtoEmi),
			Sequential: trim(it.Secuencial), AccessKey: key},
		issuer: Issuer{RUC: ruc, BusinessName: trim(it.RazonSocial), TradeName: trim(it.NombreComercial),
			MainAddress: trim(it.DirMatriz), WithholdingAgentResolution: trim(it.AgenteRetencion),
			Rimpe: readRimpe(it.ContribuyenteRimpe)},
		parts: parts,
	}, nil
}

// readRimpe maps the RIMPE legend: the current one is the same for both
// regimes and reads as RimpeEntrepreneur; the older negocio popular legend
// reads as RimpePopularBusiness.
func readRimpe(legend string) RimpeRegime {
	switch l := strings.ToUpper(trim(legend)); {
	case l == "":
		return RimpeNone
	case strings.Contains(l, "NEGOCIO POPULAR"):
		return RimpePopularBusiness
	default:
		return RimpeEntrepreneur
	}
}

// readLine reads an invoice or credit note item with its printed subtotal
// and VAT. Its only tax must be VAT 0 % or 15 % (the rate comes from the
// percentage code; a tarifa, when written, must agree).
func readLine(code, aux, description, qty, price, discount, subtotal string, taxes []xmlLineTax) (Line, LineTotals, error) {
	l := Line{Code: trim(code), AuxCode: trim(aux), Description: trim(description)}
	var lt LineTotals
	var err error
	for _, f := range []struct {
		dst *decimal.Decimal
		src string
	}{{&l.Quantity, qty}, {&l.UnitPrice, price}, {&l.Discount, discount}, {&lt.Subtotal, subtotal}} {
		if *f.dst, err = readAmount(f.src); err != nil {
			return Line{}, LineTotals{}, err
		}
	}
	switch {
	case len(taxes) == 0:
		return Line{}, LineTotals{}, errors.New("un ítem no tiene IVA")
	case len(taxes) > 1:
		for _, t := range taxes {
			if _, ok := vatRate(t.Codigo, t.CodigoPorcentaje); !ok {
				return Line{}, LineTotals{}, otherTax("un ítem", t.Codigo, t.CodigoPorcentaje)
			}
		}
		return Line{}, LineTotals{}, errors.New("un ítem tiene más de un IVA")
	}
	t := taxes[0]
	rate, ok := vatRate(t.Codigo, t.CodigoPorcentaje)
	if !ok {
		return Line{}, LineTotals{}, otherTax("un ítem", t.Codigo, t.CodigoPorcentaje)
	}
	if tarifa := trim(t.Tarifa); tarifa != "" {
		if d, err := decimal.NewFromString(tarifa); err != nil || !d.Equal(decimal.NewFromInt(int64(rate))) {
			return Line{}, LineTotals{}, fmt.Errorf("la tarifa de IVA no es válida: %q", tarifa)
		}
	}
	l.TaxRate = rate
	if lt.IVA, err = readAmount(t.Valor); err != nil {
		return Line{}, LineTotals{}, err
	}
	return l, lt, nil
}

// vatRate is the rate of VAT 0 % (code 2, percentage 0) or 15 % (code 2,
// percentage 4); ok is false for any other tax.
func vatRate(codigo, pct string) (rate int, ok bool) {
	if trim(codigo) != vatTaxCode {
		return 0, false
	}
	switch trim(pct) {
	case "0":
		return 0, true
	case "4":
		return 15, true
	}
	return 0, false
}

// otherTax is the error for a tax this version does not read.
func otherTax(where, codigo, pct string) error {
	return fmt.Errorf("%s tiene un impuesto que no es IVA 0 %% ni 15 %%: %q", where, trim(codigo)+"/"+trim(pct))
}

// readTaxTotals adds the printed VAT totals to t: the 0 % base to Subtotal0,
// the 15 % base and its VAT to Subtotal15 and IVA. Any other tax is an error.
func readTaxTotals(t *Totals, taxes []taxTotal) error {
	for _, tax := range taxes {
		rate, ok := vatRate(tax.Codigo, tax.CodigoPorcentaje)
		if !ok {
			return otherTax("el comprobante", tax.Codigo, tax.CodigoPorcentaje)
		}
		base, err := readAmount(tax.BaseImponible)
		if err != nil {
			return err
		}
		value, err := readAmount(tax.Valor)
		if err != nil {
			return err
		}
		if rate == 15 {
			t.Subtotal15, t.IVA = t.Subtotal15.Add(base), t.IVA.Add(value)
		} else {
			t.Subtotal0 = t.Subtotal0.Add(base)
		}
	}
	return nil
}

// readPayment reads an invoice payment; the term is turned into days (a
// month counts as 30).
func readPayment(p xmlInvoicePayment) (Payment, error) {
	pay := Payment{Method: trim(p.FormaPago)}
	var err error
	if pay.Amount, err = readAmount(p.Total); err != nil {
		return Payment{}, err
	}
	if trim(p.Plazo) == "" {
		return pay, nil
	}
	term, err := decimal.NewFromString(trim(p.Plazo))
	if err != nil || !term.IsInteger() || term.IsNegative() {
		return Payment{}, fmt.Errorf("el plazo no es válido: %q", trim(p.Plazo))
	}
	switch unit := strings.ToLower(trim(p.UnidadTiempo)); unit {
	case "", termUnitDays, "días":
		pay.TermDays = int(term.IntPart())
	case "meses":
		pay.TermDays = int(term.IntPart()) * 30
	default:
		return Payment{}, fmt.Errorf("la unidad del plazo no es válida: %q", trim(p.UnidadTiempo))
	}
	return pay, nil
}

func readAdditional(info *xmlInfoAdicional) []AdditionalField {
	if info == nil || len(info.Campos) == 0 {
		return nil
	}
	out := make([]AdditionalField, len(info.Campos))
	for i, c := range info.Campos {
		out[i] = AdditionalField{Name: trim(c.Nombre), Value: trim(c.Valor)}
	}
	return out
}

func readAmount(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(trim(s))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("el valor no es un número: %q", trim(s))
	}
	return d, nil
}

// readDate reads a dd/mm/aaaa date as midnight in Ecuador's time.
func readDate(s string) (time.Time, error) {
	d, err := time.ParseInLocation(dateLayout, trim(s), ecuadorTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("la fecha no es válida: %q", trim(s))
	}
	return d, nil
}

func trim(s string) string { return strings.TrimSpace(s) }

// envelope is what an <autorizacion> says around the receipt.
type envelope struct {
	found        bool
	number, date string
}

// info returns the authorization of an enveloped receipt (nil for a bare
// one): the number, or the access key when it is empty, and the date in
// Ecuador's time (zero when the envelope has none).
func (e envelope) info(r tributaria) (*AuthorizationInfo, error) {
	if !e.found {
		return nil, nil
	}
	info := &AuthorizationInfo{Number: trim(e.number), Environment: r.parts.Environment}
	if info.Number == "" {
		info.Number = r.header.AccessKey
	}
	if d := trim(e.date); d != "" {
		t, err := readAuthorizationDate(d)
		if err != nil {
			return nil, err
		}
		info.Date = t
	}
	return info, nil
}

// readAuthorizationDate reads the fechaAutorizacion the SRI writes: RFC 3339
// (with or without fractions of a second) or the older dd/mm/aaaa hh:mm:ss in
// Ecuador's time.
func readAuthorizationDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(ecuadorTime), nil
	}
	if t, err := time.ParseInLocation("02/01/2006 15:04:05", s, ecuadorTime); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("la fecha de autorización no es válida: %q", s)
}

// decodeReceipt decodes the receipt in data into doc, from inside the
// envelope when there is one. Only bytes that are not valid UTF-8 are decoded
// by their declared Latin-1 label: the receipt the SRI returns is UTF-8 text
// that keeps the prolog its system wrote, and decoding it again would mangle
// every accented letter.
func decodeReceipt(data []byte, doc any) (envelope, error) {
	convert := !utf8.Valid(data)
	env, receipt, err := readEnvelope(data, convert)
	var notAuthorized errNotAuthorized
	if errors.As(err, &notAuthorized) {
		return envelope{}, err
	}
	if err != nil {
		return envelope{}, malformed(err)
	}
	if env.found {
		err = newDecoder([]byte(receipt), false).Decode(doc)
	} else {
		err = newDecoder(data, convert).Decode(doc)
	}
	if err != nil {
		return envelope{}, malformed(err)
	}
	return env, nil
}

func malformed(err error) error { return fmt.Errorf("el XML está mal formado (%v)", err) }

// readEnvelope finds the receipt of the first authorized entry, at any
// nesting, with the authorization number and date of that entry. An entry
// (<autorizacion>) whose estado is present and is not AUTORIZADO is skipped;
// when the answer has entries but none is authorized it is an error. An
// envelope without estado is read as authorized. A bare receipt has no
// entry (a receipt in CDATA or escaped text is not markup, so nothing inside
// it is taken for the envelope).
func readEnvelope(data []byte, convert bool) (envelope, string, error) {
	var env envelope
	var estado, rejected string
	dec := newDecoder(data, convert)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if rejected != "" {
				return envelope{}, "", errNotAuthorized{rejected}
			}
			return envelope{}, "", nil
		}
		if err != nil {
			return envelope{}, "", err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		var text string
		switch start.Name.Local {
		case "autorizacion":
			env, estado = envelope{}, ""
			continue
		case "estado", "numeroAutorizacion", "fechaAutorizacion", "comprobante":
			if err := dec.DecodeElement(&text, &start); err != nil {
				return envelope{}, "", err
			}
		default:
			continue
		}
		switch start.Name.Local {
		case "estado":
			if estado = trim(text); estado != string(StatusAuthorized) {
				rejected = estado
			}
		case "numeroAutorizacion":
			env.number = text
		case "fechaAutorizacion":
			env.date = text
		default:
			if estado != "" && estado != string(StatusAuthorized) {
				continue
			}
			env.found = true
			return env, trim(text), nil
		}
	}
}

// errNotAuthorized is an SRI answer whose entries are not authorized.
type errNotAuthorized struct{ state string }

func (e errNotAuthorized) Error() string {
	return "el comprobante no está autorizado (" + e.state + ")"
}

// newDecoder reads UTF-8 and, when convert is set, the single-byte charsets
// older systems declare. Without convert the content is already UTF-8 (the
// receipt inside an envelope, or valid UTF-8 text) whatever its declaration
// says, so the label is accepted and nothing is converted.
func newDecoder(data []byte, convert bool) *xml.Decoder {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(label string, in io.Reader) (io.Reader, error) {
		l := strings.ToLower(trim(label))
		if !convert || l == "utf-8" || l == "utf8" {
			return in, nil
		}
		table, ok := singleByteCharsets[l]
		if !ok {
			return nil, fmt.Errorf("codificación no admitida: %s", label)
		}
		return singleByteReader(in, table)
	}
	return dec
}

// singleByteCharsets maps a declared label to the runes of bytes 0x80–0x9F;
// every other byte is its own code point (ISO-8859-1). windows-1252 differs
// from ISO-8859-1 only in that range.
var singleByteCharsets = map[string]*[32]rune{
	"iso-8859-1": &latin1High, "iso8859-1": &latin1High, "latin1": &latin1High, "latin-1": &latin1High,
	"windows-1252": &cp1252High, "cp1252": &cp1252High,
}

var latin1High = func() (t [32]rune) {
	for i := range t {
		t[i] = rune(0x80 + i)
	}
	return t
}()

// cp1252High is windows-1252 from 0x80 to 0x9F; its five undefined bytes keep
// their ISO-8859-1 code point.
var cp1252High = [32]rune{
	'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
	0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ',
}

// singleByteReader decodes the whole single-byte input to UTF-8.
func singleByteReader(in io.Reader, high *[32]rune) (io.Reader, error) {
	raw, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(raw)+len(raw)/8)
	for _, b := range raw {
		r := rune(b)
		if b >= 0x80 && b <= 0x9F {
			r = high[b-0x80]
		}
		out = utf8.AppendRune(out, r)
	}
	return bytes.NewReader(out), nil
}
