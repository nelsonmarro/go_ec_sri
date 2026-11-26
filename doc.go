// Package sri implements Ecuador's SRI electronic invoicing: access keys,
// XML documents (invoice, credit note and withholding), validation and the
// document types and environments the SRI defines.
//
// The package is free of I/O and state: only the soap subpackage uses the
// network, through an injected *http.Client.
package sri
