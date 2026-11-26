package sri

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationMessages(t *testing.T) {
	want := map[string]string{
		"issuer_ruc_invalid":               "El RUC del emisor no tiene el formato del SRI (13 dígitos que terminan en 001).",
		"issuer_name_required":             "Falta la razón social del emisor.",
		"issuer_address_required":          "Falta la dirección de la matriz del emisor.",
		"header_environment_invalid":       "El ambiente debe ser 1 (pruebas) o 2 (producción).",
		"header_establishment_invalid":     "El establecimiento debe tener 3 dígitos.",
		"header_emission_point_invalid":    "El punto de emisión debe tener 3 dígitos.",
		"header_sequential_invalid":        "El secuencial debe tener de 1 a 9 dígitos y ser mayor que cero.",
		"header_issue_date_required":       "Falta la fecha de emisión.",
		"access_key_invalid":               "La clave de acceso no es válida (49 dígitos con su dígito verificador).",
		"access_key_mismatch":              "La clave de acceso no coincide con los datos del comprobante.",
		"party_id_type_invalid":            "El tipo de identificación no es válido.",
		"party_ruc_invalid":                "El RUC no tiene el formato del SRI (13 dígitos que terminan en 001).",
		"party_cedula_invalid":             "La cédula no es válida.",
		"party_passport_invalid":           "El pasaporte o la identificación del exterior no es válida.",
		"party_final_consumer_id":          "El consumidor final se identifica con 9999999999999.",
		"party_name_required":              "Falta el nombre o la razón social.",
		"party_email_invalid":              "El correo no es válido.",
		"buyer_final_consumer_limit":       "Una factura de más de USD 50 no puede ir a consumidor final.",
		"lines_required":                   "El comprobante no tiene ítems.",
		"line_description_required":        "Cada ítem necesita una descripción.",
		"line_negative_amount":             "La cantidad, el precio y el descuento no pueden ser negativos.",
		"line_discount_exceeds":            "El descuento no puede ser mayor que el valor del ítem.",
		"line_tax_rate_unsupported":        "Solo se admite IVA 0 % o 15 %.",
		"payments_required":                "Falta la forma de pago.",
		"payment_method_invalid":           "La forma de pago no es válida.",
		"payments_total_mismatch":          "Los pagos no suman el total del comprobante.",
		"additional_fields_limit":          "Se admiten hasta 15 campos adicionales.",
		"additional_field_invalid":         "Cada campo adicional necesita nombre y un valor de hasta 300 caracteres.",
		"credit_note_modified_invalid":     "La nota de crédito necesita el número (001-001-000000123) y la fecha de la factura que modifica.",
		"credit_note_reason_required":      "La nota de crédito necesita un motivo de hasta 300 caracteres.",
		"withholding_period_required":      "Falta el período fiscal de la retención.",
		"support_document_invalid":         "El documento de sustento no es válido (tipo, número, fecha o autorización).",
		"support_sustento_invalid":         "El código de sustento no vale para ese tipo de documento.",
		"support_totals_mismatch":          "Los totales del documento de sustento no cuadran.",
		"withholding_line_invalid":         "Cada retención necesita impuesto, código, base y porcentaje válidos.",
		"withholding_iva_percentage":       "El porcentaje de retención de IVA debe ser 10, 20, 30, 50, 70 o 100.",
		"withholding_amount_mismatch":      "El valor retenido debe ser la base por el porcentaje.",
		"text_too_long":                    "El texto supera el largo que admite el SRI.",
		"issuer_withholding_agent_invalid": "El número de resolución de agente de retención debe tener de 1 a 8 dígitos.",
		"issuer_special_taxpayer_invalid":  "El número de contribuyente especial debe tener de 3 a 13 letras o dígitos.",
		"issuer_rimpe_invalid":             "El régimen RIMPE no es válido.",
		"amount_too_many_decimals":         "El valor tiene más decimales de los que admite el SRI (cantidad y precio hasta 6; montos hasta 2).",
		"payment_amount_invalid":           "Cada pago necesita un valor mayor que cero y un plazo que no sea negativo.",
		"credit_note_final_consumer":       "Una nota de crédito no puede emitirse a consumidor final.",
		"total_zero":                       "El comprobante debe tener un valor mayor que cero.",
	}
	assert.Equal(t, want, validationMessages)
}

func TestValidationErrorsError(t *testing.T) {
	errs := ValidationErrors{
		{Field: "buyer.id", Code: "party_cedula_invalid", Message: "La cédula no es válida."},
		{Field: "lines", Code: "lines_required", Message: "El comprobante no tiene ítems."},
	}
	assert.Equal(t, "buyer.id: La cédula no es válida.; lines: El comprobante no tiene ítems.", errs.Error())
}

func TestValidationCollector(t *testing.T) {
	var v validation
	require.NoError(t, v.result(), "no problems is a nil error, not a typed nil")
	v.add("lines[0].quantity", "line_negative_amount")
	err := v.result()
	var errs ValidationErrors
	require.True(t, errors.As(err, &errs))
	assert.Equal(t, ValidationErrors{{
		Field: "lines[0].quantity", Code: "line_negative_amount",
		Message: "La cantidad, el precio y el descuento no pueden ser negativos.",
	}}, errs)
}

func TestCodeConstants(t *testing.T) {
	tests := []struct{ got, want string }{
		{CodeIssuerRUCInvalid, "issuer_ruc_invalid"},
		{CodeIssuerNameRequired, "issuer_name_required"},
		{CodeIssuerAddressRequired, "issuer_address_required"},
		{CodeHeaderEnvironmentInvalid, "header_environment_invalid"},
		{CodeHeaderEstablishmentInvalid, "header_establishment_invalid"},
		{CodeHeaderEmissionPointInvalid, "header_emission_point_invalid"},
		{CodeHeaderSequentialInvalid, "header_sequential_invalid"},
		{CodeHeaderIssueDateRequired, "header_issue_date_required"},
		{CodeAccessKeyInvalid, "access_key_invalid"},
		{CodeAccessKeyMismatch, "access_key_mismatch"},
		{CodePartyIDTypeInvalid, "party_id_type_invalid"},
		{CodePartyRUCInvalid, "party_ruc_invalid"},
		{CodePartyCedulaInvalid, "party_cedula_invalid"},
		{CodePartyPassportInvalid, "party_passport_invalid"},
		{CodePartyFinalConsumerID, "party_final_consumer_id"},
		{CodePartyNameRequired, "party_name_required"},
		{CodePartyEmailInvalid, "party_email_invalid"},
		{CodeBuyerFinalConsumerLimit, "buyer_final_consumer_limit"},
		{CodeLinesRequired, "lines_required"},
		{CodeLineDescriptionRequired, "line_description_required"},
		{CodeLineNegativeAmount, "line_negative_amount"},
		{CodeLineDiscountExceeds, "line_discount_exceeds"},
		{CodeLineTaxRateUnsupported, "line_tax_rate_unsupported"},
		{CodePaymentsRequired, "payments_required"},
		{CodePaymentMethodInvalid, "payment_method_invalid"},
		{CodePaymentsTotalMismatch, "payments_total_mismatch"},
		{CodeAdditionalFieldsLimit, "additional_fields_limit"},
		{CodeAdditionalFieldInvalid, "additional_field_invalid"},
		{CodeCreditNoteModifiedInvalid, "credit_note_modified_invalid"},
		{CodeCreditNoteReasonRequired, "credit_note_reason_required"},
		{CodeWithholdingPeriodRequired, "withholding_period_required"},
		{CodeSupportDocumentInvalid, "support_document_invalid"},
		{CodeSupportSustentoInvalid, "support_sustento_invalid"},
		{CodeSupportTotalsMismatch, "support_totals_mismatch"},
		{CodeWithholdingLineInvalid, "withholding_line_invalid"},
		{CodeWithholdingIVAPercentage, "withholding_iva_percentage"},
		{CodeWithholdingAmountMismatch, "withholding_amount_mismatch"},
		{CodeTextTooLong, "text_too_long"},
		{CodeIssuerWithholdingAgentInvalid, "issuer_withholding_agent_invalid"},
		{CodeIssuerSpecialTaxpayerInvalid, "issuer_special_taxpayer_invalid"},
		{CodeIssuerRimpeInvalid, "issuer_rimpe_invalid"},
		{CodeAmountTooManyDecimals, "amount_too_many_decimals"},
		{CodePaymentAmountInvalid, "payment_amount_invalid"},
		{CodeCreditNoteFinalConsumer, "credit_note_final_consumer"},
		{CodeTotalZero, "total_zero"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.got)
		_, ok := validationMessages[tt.got]
		assert.True(t, ok, "%s has a message", tt.got)
	}
	assert.Len(t, validationMessages, len(tests), "one constant per code")
}
