package sri

import (
	"time"

	"github.com/shopspring/decimal"
)

// IDType is the SRI identification type of a buyer or supplier (ficha
// técnica, tabla 6).
type IDType string

const (
	// IDRUC is the RUC (Registro Único de Contribuyentes), code 04.
	IDRUC IDType = "04"
	// IDCedula is the Ecuadorian national ID (cédula), code 05.
	IDCedula IDType = "05"
	// IDPassport is a passport, code 06.
	IDPassport IDType = "06"
	// IDFinalConsumer is the final consumer (consumidor final), code 07.
	IDFinalConsumer IDType = "07"
	// IDForeign is a foreign identification (identificación del exterior), code 08.
	IDForeign IDType = "08"
)

// FinalConsumerID is the only identification a final consumer may carry.
const FinalConsumerID = "9999999999999"

var finalConsumerLimit = decimal.RequireFromString("50.00")

// FinalConsumerLimit returns the highest invoice total (VAT included) the SRI
// accepts for a final consumer (USD 50).
func FinalConsumerLimit() decimal.Decimal { return finalConsumerLimit }

// RimpeRegime is the RIMPE regime of the issuer, if any.
type RimpeRegime string

const (
	// RimpeNone means the issuer is not in the RIMPE regime.
	RimpeNone RimpeRegime = ""
	// RimpeEntrepreneur is the RIMPE emprendedor regime.
	RimpeEntrepreneur RimpeRegime = "emprendedor"
	// RimpePopularBusiness is the RIMPE negocio popular regime.
	RimpePopularBusiness RimpeRegime = "negocio_popular"
)

// Issuer is the business that issues the document (emisor).
type Issuer struct {
	// RUC, BusinessName (razón social), TradeName (nombre comercial),
	// MainAddress (dirección matriz) and EstablishmentAddress (dirección del
	// establecimiento).
	RUC, BusinessName, TradeName, MainAddress, EstablishmentAddress string
	// KeepsAccounting is obligado a llevar contabilidad.
	KeepsAccounting bool
	// SpecialTaxpayerNumber is the contribuyente especial resolution number;
	// WithholdingAgentResolution is the agente de retención resolution number.
	SpecialTaxpayerNumber, WithholdingAgentResolution string
	// Rimpe is the RIMPE regime, RimpeNone when it does not apply.
	Rimpe RimpeRegime
}

// Party is the buyer of an invoice or credit note, or the supplier of a
// withholding (sujeto retenido).
type Party struct {
	IDType                          IDType
	ID, Name, Address, Email, Phone string
	// RelatedParty marks a supplier that is a related party of the issuer
	// (parte relacionada); the withholding prints parteRel SI.
	RelatedParty bool
}

// Header holds the fields every document shares.
type Header struct {
	Environment Environment
	// Establishment and EmissionPoint have 3 digits; Sequential has 1 to 9
	// digits and is padded with zeros to 9.
	Establishment, EmissionPoint, Sequential string
	// IssueDate: the calendar date (year, month, day, as given and in the
	// value's own location) is what is printed in the XML and in the access
	// key. It must be the date in Ecuador: pass sri.Today() or a DATE value;
	// an instant from a server in UTC would print tomorrow's date after 19:00
	// in Ecuador.
	IssueDate time.Time
	// AccessKey is optional: when empty, the XML builder generates it.
	AccessKey string
}

// Line is one item of an invoice or credit note.
type Line struct {
	Code, AuxCode, Description    string
	Quantity, UnitPrice, Discount decimal.Decimal
	// TaxRate is the VAT rate in percent: 0 or 15.
	TaxRate int
}

// Payment is one payment of an invoice or of a withholding's support document.
type Payment struct {
	// Method is a code of the SRI payment-method catalog (PaymentMethods).
	Method   string
	Amount   decimal.Decimal
	TermDays int
}

// AdditionalField is one campoAdicional (up to 15, 300 characters each).
type AdditionalField struct {
	Name, Value string
}

// Invoice is a factura (01).
type Invoice struct {
	Header     Header
	Issuer     Issuer
	Buyer      Party
	Lines      []Line
	Payments   []Payment
	Additional []AdditionalField
}

// CreditNote is a nota de crédito (04) that modifies an invoice.
type CreditNote struct {
	Header Header
	Issuer Issuer
	Buyer  Party
	// ModifiedNumber is the modified invoice number (001-001-000000123) and
	// ModifiedDate its issue date.
	ModifiedNumber string
	ModifiedDate   time.Time
	Reason         string
	Lines          []Line
	Additional     []AdditionalField
}

// WithholdingTax is the tax a withholding line withholds.
type WithholdingTax string

const (
	// WithholdingRenta is the income tax withholding (código 1).
	WithholdingRenta WithholdingTax = "renta"
	// WithholdingIVA is the VAT withholding (código 2).
	WithholdingIVA WithholdingTax = "iva"
)

// WithholdingLine is one withholding (retención) on the support document.
type WithholdingLine struct {
	Tax WithholdingTax
	// Code is the codigoRetencion: the ATS income-tax concept or the VAT
	// withholding code.
	Code                     string
	Base, Percentage, Amount decimal.Decimal
}

// SupportDocument is the supplier's document the withholding is made on
// (documento de sustento).
type SupportDocument struct {
	// DocType is DocInvoice (01) or DocSalesNote (02).
	DocType DocType
	// Number is the document number, 001-001-000000123.
	Number string
	// IssueDate: its calendar date is printed as given; it must be the
	// document's date in Ecuador (see Header.IssueDate).
	IssueDate time.Time
	// Authorization is the supplier's authorization number (10, 37 or 49
	// digits), optional.
	Authorization string
	// SustentoCode is the ATS sustento code (tabla 5).
	SustentoCode                      string
	Subtotal0, Subtotal15, IVA, Total decimal.Decimal
	Payments                          []Payment
}

// Withholding is a comprobante de retención (07), version 2.0.0.
type Withholding struct {
	Header   Header
	Issuer   Issuer
	Supplier Party
	// FiscalPeriod is the fiscal period; only its year and month, as given in
	// the value's own location, are used.
	FiscalPeriod time.Time
	Support      SupportDocument
	Lines        []WithholdingLine
	Additional   []AdditionalField
}
