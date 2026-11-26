package sri

import "time"

// MessageType is the type of an SRI message.
type MessageType string

const (
	// MessageError is a message that rejects the document.
	MessageError MessageType = "ERROR"
	// MessageWarning is a warning that does not reject the document.
	MessageWarning MessageType = "ADVERTENCIA"
)

// Message is one message of an SRI answer (mensaje): its numeric identifier,
// the text, the additional information and the type.
type Message struct {
	Identifier, Text, AdditionalInfo string
	Type                             MessageType
}

// ReceptionStatus is the answer of the reception service.
type ReceptionStatus string

const (
	// ReceptionReceived means the SRI received the document (RECIBIDA).
	ReceptionReceived ReceptionStatus = "RECIBIDA"
	// ReceptionReturned means the SRI returned the document (DEVUELTA): read
	// the messages.
	ReceptionReturned ReceptionStatus = "DEVUELTA"
)

// Reception is the answer of the reception service for one document.
type Reception struct {
	Status    ReceptionStatus
	AccessKey string
	Messages  []Message
}

// AuthorizationStatus is the state of a document in the authorization
// service.
type AuthorizationStatus string

const (
	// StatusAuthorized is AUTORIZADO.
	StatusAuthorized AuthorizationStatus = "AUTORIZADO"
	// StatusNotAuthorized is NO AUTORIZADO: read the messages.
	StatusNotAuthorized AuthorizationStatus = "NO AUTORIZADO"
	// StatusInProcess is EN PROCESO: ask again later.
	StatusInProcess AuthorizationStatus = "EN PROCESO"
	// StatusNone means the SRI has no authorization for the access key yet.
	StatusNone AuthorizationStatus = ""
)

// Authorization is the answer of the authorization service for one access
// key.
type Authorization struct {
	Status            AuthorizationStatus
	AccessKey, Number string
	// Date is the authorization date in Ecuador's time (America/Guayaquil).
	Date        time.Time
	Environment Environment
	// XML is the signed receipt the SRI authorized.
	XML      []byte
	Messages []Message
}

// AuthorizationInfo is what the <autorizacion> envelope says about a receipt
// read by ParseInvoice, ParseCreditNote or ParseWithholding.
type AuthorizationInfo struct {
	// Number is the authorization number (the access key in the offline
	// scheme).
	Number string
	// Date is the authorization date in Ecuador's time.
	Date time.Time
	// Environment is the environment of the receipt's access key.
	Environment Environment
}
