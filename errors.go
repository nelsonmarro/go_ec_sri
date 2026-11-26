package sri

import "strings"

// ValidationError is one problem Validate found. Field is the path of the
// field (lines[2].quantity, buyer.id), Code is stable English snake_case and
// Message is clear Spanish fit for the end user.
type ValidationError struct{ Field, Code, Message string }

// ValidationErrors are all the problems Validate found, in document order.
type ValidationErrors []ValidationError

// Error joins every problem as "field: message", separated by "; ".
func (v ValidationErrors) Error() string {
	parts := make([]string, len(v))
	for i, e := range v {
		parts[i] = e.Field + ": " + e.Message
	}
	return strings.Join(parts, "; ")
}

// Validation codes: stable values of ValidationError.Code. Changing one is a
// breaking change.
const (
	CodeIssuerRUCInvalid              = "issuer_ruc_invalid"
	CodeIssuerNameRequired            = "issuer_name_required"
	CodeIssuerAddressRequired         = "issuer_address_required"
	CodeHeaderEnvironmentInvalid      = "header_environment_invalid"
	CodeHeaderEstablishmentInvalid    = "header_establishment_invalid"
	CodeHeaderEmissionPointInvalid    = "header_emission_point_invalid"
	CodeHeaderSequentialInvalid       = "header_sequential_invalid"
	CodeHeaderIssueDateRequired       = "header_issue_date_required"
	CodeAccessKeyInvalid              = "access_key_invalid"
	CodeAccessKeyMismatch             = "access_key_mismatch"
	CodePartyIDTypeInvalid            = "party_id_type_invalid"
	CodePartyRUCInvalid               = "party_ruc_invalid"
	CodePartyCedulaInvalid            = "party_cedula_invalid"
	CodePartyPassportInvalid          = "party_passport_invalid"
	CodePartyFinalConsumerID          = "party_final_consumer_id"
	CodePartyNameRequired             = "party_name_required"
	CodePartyEmailInvalid             = "party_email_invalid"
	CodeBuyerFinalConsumerLimit       = "buyer_final_consumer_limit"
	CodeLinesRequired                 = "lines_required"
	CodeLineDescriptionRequired       = "line_description_required"
	CodeLineNegativeAmount            = "line_negative_amount"
	CodeLineDiscountExceeds           = "line_discount_exceeds"
	CodeLineTaxRateUnsupported        = "line_tax_rate_unsupported"
	CodePaymentsRequired              = "payments_required"
	CodePaymentMethodInvalid          = "payment_method_invalid"
	CodePaymentsTotalMismatch         = "payments_total_mismatch"
	CodeAdditionalFieldsLimit         = "additional_fields_limit"
	CodeAdditionalFieldInvalid        = "additional_field_invalid"
	CodeCreditNoteModifiedInvalid     = "credit_note_modified_invalid"
	CodeCreditNoteReasonRequired      = "credit_note_reason_required"
	CodeWithholdingPeriodRequired     = "withholding_period_required"
	CodeSupportDocumentInvalid        = "support_document_invalid"
	CodeSupportSustentoInvalid        = "support_sustento_invalid"
	CodeSupportTotalsMismatch         = "support_totals_mismatch"
	CodeWithholdingLineInvalid        = "withholding_line_invalid"
	CodeWithholdingIVAPercentage      = "withholding_iva_percentage"
	CodeWithholdingAmountMismatch     = "withholding_amount_mismatch"
	CodeTextTooLong                   = "text_too_long"
	CodeIssuerWithholdingAgentInvalid = "issuer_withholding_agent_invalid"
	CodeIssuerSpecialTaxpayerInvalid  = "issuer_special_taxpayer_invalid"
	CodeIssuerRimpeInvalid            = "issuer_rimpe_invalid"
	CodeAmountTooManyDecimals         = "amount_too_many_decimals"
	CodePaymentAmountInvalid          = "payment_amount_invalid"
	CodeCreditNoteFinalConsumer       = "credit_note_final_consumer"
	CodeTotalZero                     = "total_zero"
)

// validationMessages maps each stable code to its message. Changing a code is
// a breaking change.
var validationMessages = map[string]string{
	CodeIssuerRUCInvalid:              "El RUC del emisor no tiene el formato del SRI (13 dígitos que terminan en 001).",
	CodeIssuerNameRequired:            "Falta la razón social del emisor.",
	CodeIssuerAddressRequired:         "Falta la dirección de la matriz del emisor.",
	CodeHeaderEnvironmentInvalid:      "El ambiente debe ser 1 (pruebas) o 2 (producción).",
	CodeHeaderEstablishmentInvalid:    "El establecimiento debe tener 3 dígitos.",
	CodeHeaderEmissionPointInvalid:    "El punto de emisión debe tener 3 dígitos.",
	CodeHeaderSequentialInvalid:       "El secuencial debe tener de 1 a 9 dígitos y ser mayor que cero.",
	CodeHeaderIssueDateRequired:       "Falta la fecha de emisión.",
	CodeAccessKeyInvalid:              "La clave de acceso no es válida (49 dígitos con su dígito verificador).",
	CodeAccessKeyMismatch:             "La clave de acceso no coincide con los datos del comprobante.",
	CodePartyIDTypeInvalid:            "El tipo de identificación no es válido.",
	CodePartyRUCInvalid:               "El RUC no tiene el formato del SRI (13 dígitos que terminan en 001).",
	CodePartyCedulaInvalid:            "La cédula no es válida.",
	CodePartyPassportInvalid:          "El pasaporte o la identificación del exterior no es válida.",
	CodePartyFinalConsumerID:          "El consumidor final se identifica con 9999999999999.",
	CodePartyNameRequired:             "Falta el nombre o la razón social.",
	CodePartyEmailInvalid:             "El correo no es válido.",
	CodeBuyerFinalConsumerLimit:       "Una factura de más de USD 50 no puede ir a consumidor final.",
	CodeLinesRequired:                 "El comprobante no tiene ítems.",
	CodeLineDescriptionRequired:       "Cada ítem necesita una descripción.",
	CodeLineNegativeAmount:            "La cantidad, el precio y el descuento no pueden ser negativos.",
	CodeLineDiscountExceeds:           "El descuento no puede ser mayor que el valor del ítem.",
	CodeLineTaxRateUnsupported:        "Solo se admite IVA 0 % o 15 %.",
	CodePaymentsRequired:              "Falta la forma de pago.",
	CodePaymentMethodInvalid:          "La forma de pago no es válida.",
	CodePaymentsTotalMismatch:         "Los pagos no suman el total del comprobante.",
	CodeAdditionalFieldsLimit:         "Se admiten hasta 15 campos adicionales.",
	CodeAdditionalFieldInvalid:        "Cada campo adicional necesita nombre y un valor de hasta 300 caracteres.",
	CodeCreditNoteModifiedInvalid:     "La nota de crédito necesita el número (001-001-000000123) y la fecha de la factura que modifica.",
	CodeCreditNoteReasonRequired:      "La nota de crédito necesita un motivo de hasta 300 caracteres.",
	CodeWithholdingPeriodRequired:     "Falta el período fiscal de la retención.",
	CodeSupportDocumentInvalid:        "El documento de sustento no es válido (tipo, número, fecha o autorización).",
	CodeSupportSustentoInvalid:        "El código de sustento no vale para ese tipo de documento.",
	CodeSupportTotalsMismatch:         "Los totales del documento de sustento no cuadran.",
	CodeWithholdingLineInvalid:        "Cada retención necesita impuesto, código, base y porcentaje válidos.",
	CodeWithholdingIVAPercentage:      "El porcentaje de retención de IVA debe ser 10, 20, 30, 50, 70 o 100.",
	CodeWithholdingAmountMismatch:     "El valor retenido debe ser la base por el porcentaje.",
	CodeTextTooLong:                   "El texto supera el largo que admite el SRI.",
	CodeIssuerWithholdingAgentInvalid: "El número de resolución de agente de retención debe tener de 1 a 8 dígitos.",
	CodeIssuerSpecialTaxpayerInvalid:  "El número de contribuyente especial debe tener de 3 a 13 letras o dígitos.",
	CodeIssuerRimpeInvalid:            "El régimen RIMPE no es válido.",
	CodeAmountTooManyDecimals:         "El valor tiene más decimales de los que admite el SRI (cantidad y precio hasta 6; montos hasta 2).",
	CodePaymentAmountInvalid:          "Cada pago necesita un valor mayor que cero y un plazo que no sea negativo.",
	CodeCreditNoteFinalConsumer:       "Una nota de crédito no puede emitirse a consumidor final.",
	CodeTotalZero:                     "El comprobante debe tener un valor mayor que cero.",
}

// validation collects problems in the order they are found.
type validation struct{ errs ValidationErrors }

func (v *validation) add(field, code string) {
	v.errs = append(v.errs, ValidationError{Field: field, Code: code, Message: validationMessages[code]})
}

// result is nil when nothing was found (an untyped nil, so err == nil holds)
// and the ValidationErrors otherwise.
func (v *validation) result() error {
	if len(v.errs) == 0 {
		return nil
	}
	return v.errs
}
