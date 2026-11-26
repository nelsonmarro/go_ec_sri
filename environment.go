package sri

// Environment is the SRI environment a document is issued in.
type Environment int

const (
	// EnvironmentTest is the SRI test environment (pruebas).
	EnvironmentTest Environment = 1
	// EnvironmentProduction is the SRI production environment.
	EnvironmentProduction Environment = 2
)

// DocType is the SRI voucher type code.
type DocType string

const (
	// DocInvoice is the invoice (factura), code 01.
	DocInvoice DocType = "01"
	// DocSalesNote is the sales note (nota de venta), code 02. It is only a
	// withholding's support document: this package does not issue it.
	DocSalesNote DocType = "02"
	// DocCreditNote is the credit note (nota de crédito), code 04.
	DocCreditNote DocType = "04"
	// DocWithholding is the withholding certificate (comprobante de retención), code 07.
	DocWithholding DocType = "07"
)
