package sri

import "strings"

// FixIn says where the cause of an SRI rejection is fixed. The values are
// stable.
type FixIn string

const (
	// FixIssuer: the issuer's data (RUC, withholding agent resolution).
	FixIssuer FixIn = "issuer"
	// FixSignature: the electronic signature certificate.
	FixSignature FixIn = "signature"
	// FixEmissionPoint: the emission point or its sequential.
	FixEmissionPoint FixIn = "emission_point"
	// FixBuyer: the buyer of an invoice or credit note.
	FixBuyer FixIn = "buyer"
	// FixSupplier: the supplier (sujeto retenido) of a withholding.
	FixSupplier FixIn = "supplier"
	// FixDocument: the document's own content, or the document it refers to
	// (the invoice a credit note modifies, a withholding's support document).
	FixDocument FixIn = "document"
	// FixSRI: a procedure at the SRI itself.
	FixSRI FixIn = "sri"
	// FixRetry: issuing the document again is enough.
	FixRetry FixIn = "retry"
	// FixWait: nothing to fix, only wait.
	FixWait FixIn = "wait"
	// FixCounterparty: the buyer or the supplier, when Explain does not know
	// the document type (ExplainFor tells them apart).
	FixCounterparty FixIn = "counterparty"
	// FixSupport: a fault of the software that builds the document, or a
	// message the catalog does not know.
	FixSupport FixIn = "support"
)

// Explanation is an SRI message in plain Spanish: Title and Advice are fit
// for the end user, FixIn says where to fix it and Original keeps the SRI's
// own text.
type Explanation struct {
	Code, Title, Advice string
	FixIn               FixIn
	Original            string
}

type rejectionEntry struct {
	title, advice string
	fixIn         FixIn
}

const (
	unknownTitle  = "El SRI rechazó el comprobante"
	unknownAdvice = "Lee el mensaje original del SRI para saber qué corregir y emite el comprobante de nuevo."
	// counterpartyAdvice is the advice for the identification codes when the
	// document type is unknown.
	counterpartyAdvice = "Corrige la identificación del comprador (en una factura o nota de crédito) o del proveedor (en una retención) y emite el comprobante de nuevo."
	// supplierRUCAdvice is the advice for the identification codes on a withholding.
	supplierRUCAdvice = "El RUC del proveedor no es válido para el SRI. Corrige el proveedor y emite el comprobante de nuevo."
	buyerTitle        = "La identificación del comprador no es válida para el SRI"
	// softwareAdvice is the advice for faults of the software that builds the document.
	softwareAdvice = "Es un error del sistema que genera el comprobante: repórtalo a quien le da soporte, con el mensaje original del SRI."
	// againWithNewKey is the advice for the codes that need a new access key.
	againWithNewKey = "Emite el comprobante de nuevo con la fecha de hoy y una clave de acceso nueva."
)

// rejectionCatalog follows chapter 11 of the SRI ficha técnica (offline
// scheme, version 2.34) plus code 96, seen in the test environment. The
// identification codes (59, 62, 69) concern the receiver of the document: the
// buyer of an invoice or credit note, the supplier of a withholding; without
// the document type they name both (ExplainFor tells them apart).
var rejectionCatalog = map[string]rejectionEntry{
	"2":  {"El RUC del emisor no está activo en el SRI", "Reactiva el RUC en el SRI y emite el comprobante de nuevo.", FixSRI},
	"10": {"El establecimiento está clausurado", "El SRI lo habilita solo cuando termina la clausura. Después, emite el comprobante de nuevo.", FixSRI},
	"26": {"El comprobante es demasiado grande", "El archivo supera el tamaño que acepta el SRI: reduce los campos adicionales o reparte los ítems en varios comprobantes y emítelo de nuevo.", FixDocument},
	"27": {"La clase de contribuyente del emisor no puede emitir comprobantes electrónicos", "Consúltalo con el SRI.", FixSRI},
	"28": {"Falta aceptar el acuerdo de medios electrónicos", "Acéptalo en SRI en línea y emite el comprobante de nuevo.", FixSRI},
	"35": {"El SRI no pudo leer el comprobante", "Revisa que los datos del comprobante no tengan símbolos raros ni campos vacíos y emítelo de nuevo. Si se repite, " + lowerFirst(softwareAdvice), FixSupport},
	"36": {"El formato del comprobante ya no está vigente", "El comprobante debe generarse con el formato vigente del SRI. " + softwareAdvice, FixSupport},
	"37": {"El RUC del emisor no tiene autorización para emitir comprobantes electrónicos", "Solicítala en SRI en línea (también para el ambiente que se está usando) y emite el comprobante de nuevo.", FixSRI},
	"39": {"La firma electrónica no es válida", "Revisa el certificado de firma (.p12) y que la fecha y la hora del equipo estén bien, y emite el comprobante de nuevo.", FixSignature},
	"40": {"El SRI no pudo leer el certificado de firma", "Vuelve a cargar el certificado de firma (.p12) y emite el comprobante de nuevo.", FixSignature},
	"42": {"La firma electrónica caducó o fue revocada", "Renueva la firma electrónica y emite el comprobante de nuevo con el certificado nuevo.", FixSignature},
	"43": {"El SRI ya tiene este comprobante", "No hay nada que corregir: consulta su autorización en unos minutos.", FixWait},
	"45": {"Ese número de comprobante ya fue usado", "Emite el comprobante con el siguiente secuencial libre del punto de emisión: saldrá con un número y una clave de acceso nuevos.", FixEmissionPoint},
	"46": {"El RUC del emisor no existe en el SRI", "Revisa el RUC del emisor y emite el comprobante de nuevo.", FixIssuer},
	"47": {"El SRI no reconoce el tipo de comprobante", softwareAdvice, FixSupport},
	"48": {"El SRI no encontró el formato de este comprobante", softwareAdvice, FixSupport},
	"49": {"El envío llegó incompleto al SRI", "Envía el comprobante de nuevo. Si se repite, " + lowerFirst(softwareAdvice), FixSupport},
	"50": {"El SRI tuvo un error interno", "No es un problema del comprobante: inténtalo de nuevo más tarde.", FixWait},
	"52": {"Los valores del comprobante no cuadran", "Es un error de cálculo del sistema que genera el comprobante: repórtalo a quien le da soporte, con el mensaje original del SRI.", FixSupport},
	"56": {"El establecimiento está cerrado en el RUC del emisor", "Ábrelo en el SRI, o emite desde un punto de emisión de un establecimiento abierto.", FixEmissionPoint},
	"57": {"La autorización para emitir está suspendida", "Consulta con el SRI el motivo de la suspensión.", FixSRI},
	"58": {"La clave de acceso no coincide con el comprobante", "Emite el comprobante de nuevo con una clave de acceso nueva.", FixRetry},
	"59": {"El SRI no encontró la identificación del comprador o del proveedor", counterpartyAdvice, FixCounterparty},
	"62": {"La identificación del comprador o del proveedor es incorrecta", counterpartyAdvice, FixCounterparty},
	"63": {"El RUC del emisor está clausurado", "Revisa la situación del RUC en el SRI.", FixSRI},
	"64": {"El SRI no reconoce el tipo del documento de sustento", "Revisa el tipo, el número y la fecha del documento de sustento y emite el comprobante de nuevo.", FixDocument},
	"65": {"El comprobante se envió demasiado tarde", againWithNewKey, FixRetry},
	"67": {"La fecha del comprobante no es válida", againWithNewKey, FixRetry},
	"68": {"El SRI no encontró el documento de sustento", "Revisa el número y la fecha del documento de sustento y emite el comprobante de nuevo.", FixDocument},
	"69": {"El RUC del comprador o del proveedor no existe en el SRI", counterpartyAdvice, FixCounterparty},
	"70": {"El SRI todavía está procesando este comprobante", "No lo vuelvas a enviar: espera su respuesta (hasta 24 horas).", FixWait},
	"96": {"El SRI no reconoce al emisor como agente de retención", "Corrige o deja vacía la resolución de agente de retención del emisor y emite el comprobante de nuevo.", FixIssuer},
}

// Explain explains an SRI message by its identifier: a plain Spanish title
// and advice, where to fix it and the original text, "[identifier] text
// (additional information)", leaving out the parts that are empty. An identifier
// the catalog does not know gets a generic explanation with FixSupport.
func Explain(m Message) Explanation {
	code := strings.TrimSpace(m.Identifier)
	entry, ok := rejectionCatalog[code]
	if !ok {
		entry = rejectionEntry{unknownTitle, unknownAdvice, FixSupport}
	}
	var parts []string
	if code != "" {
		parts = append(parts, "["+code+"]")
	}
	if text := strings.TrimSpace(m.Text); text != "" {
		parts = append(parts, text)
	}
	if info := strings.TrimSpace(m.AdditionalInfo); info != "" {
		parts = append(parts, "("+info+")")
	}
	return Explanation{Code: code, Title: entry.title, Advice: entry.advice, FixIn: entry.fixIn, Original: strings.Join(parts, " ")}
}

// lowerFirst lowers the first letter of an ASCII-initial sentence.
func lowerFirst(s string) string { return strings.ToLower(s[:1]) + s[1:] }

// documentCatalog replaces the catalog entries that depend on the document
// type: the identification codes name the buyer (01, 04) or the supplier
// (07), and the support document codes name what the document refers to.
var documentCatalog = map[DocType]map[string]rejectionEntry{
	DocInvoice: {
		"59": {buyerTitle, invoiceBuyerAdvice, FixBuyer},
		"62": {buyerTitle, invoiceBuyerAdvice, FixBuyer},
		"69": {buyerTitle, invoiceBuyerAdvice, FixBuyer},
		// An invoice refers to no document the caller writes.
		"64": {"El SRI no reconoce el documento relacionado", softwareAdvice, FixSupport},
		"68": {"El SRI no reconoce el documento relacionado", softwareAdvice, FixSupport},
	},
	DocCreditNote: {
		"59": {buyerTitle, creditNoteBuyerAdvice, FixBuyer},
		"62": {buyerTitle, creditNoteBuyerAdvice, FixBuyer},
		"69": {buyerTitle, creditNoteBuyerAdvice, FixBuyer},
		"64": {modifiedInvoiceTitle, modifiedInvoiceAdvice, FixDocument},
		"68": {modifiedInvoiceTitle, modifiedInvoiceAdvice, FixDocument},
	},
	DocWithholding: {
		"59": {"El SRI no encontró la identificación del proveedor", supplierRUCAdvice, FixSupplier},
		"62": {"La identificación del proveedor es incorrecta", supplierRUCAdvice, FixSupplier},
		"69": {"El RUC del proveedor no existe en el SRI", supplierRUCAdvice, FixSupplier},
		"64": {"El SRI no reconoce el tipo del documento del proveedor", "Revisa el tipo, el número y la fecha del documento del proveedor (documento de sustento) y emite la retención de nuevo.", FixDocument},
		"68": {"El SRI no encontró el documento del proveedor", "Revisa el número y la fecha del documento del proveedor (documento de sustento) y emite la retención de nuevo.", FixDocument},
	},
}

const (
	invoiceBuyerAdvice    = "Corrige la identificación del comprador y emite la factura de nuevo."
	creditNoteBuyerAdvice = "Corrige la identificación del comprador y emite la nota de crédito de nuevo. Si la identificación está mal en la factura que modifica, anula esa factura en el portal del SRI."
	modifiedInvoiceTitle  = "El SRI no reconoce la factura que modifica la nota de crédito"
	modifiedInvoiceAdvice = "Revisa el número y la fecha de la factura que modifica y emite la nota de crédito de nuevo."
)

// ExplainFor explains an SRI message of a document of type doc: like
// Explain, but the identification codes (59, 62, 69) name the buyer of an
// invoice or credit note (FixBuyer) or the supplier of a withholding
// (FixSupplier), and the support document codes (64, 68) name the invoice a
// credit note modifies or the supplier's document of a withholding. Any other
// type or code gets Explain's explanation.
func ExplainFor(doc DocType, m Message) Explanation {
	e := Explain(m)
	if entry, ok := documentCatalog[doc][e.Code]; ok {
		e.Title, e.Advice, e.FixIn = entry.title, entry.advice, entry.fixIn
	}
	return e
}
