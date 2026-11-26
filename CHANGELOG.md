# Changelog

## v0.1.0 (sin publicar)

Primera versión pública, extraída del motor de facturación de Verith.

- `sri`: clave de acceso (módulo 11), RUC y cédula, XML de factura 1.1.0, nota de crédito 1.1.0 y retención 2.0.0 validados contra el XSD oficial, validación con códigos estables y mensajes en español, lectura de los tres comprobantes (con o sin `<autorizacion>`), sobre autorizado, `Explain`/`ExplainFor` y `ReissuePlan`.
- `sign`: firma XAdES-BES desde un `.p12` en memoria e `Inspect`.
- `soap`: cliente de recepción y autorización, errores clasificados y `Retryable`; sin reintentos.
- `ride`: RIDE en PDF de factura, nota de crédito y retención.
- Límites: IVA 0 % y 15 %, un documento de sustento por retención, sin proveedores del exterior en retenciones. Ver el README.
- API: `sri.Today()` (fecha de Ecuador); `Header.IssueDate` imprime su fecha de calendario tal como viene. `FinalConsumerLimit` y `soap.DefaultURLs` son ahora funciones. `soap.Error.Error()` devuelve texto en español por tipo (la causa sigue en `Unwrap`). `soap.Client.Authorize` rechaza (`KindClient`) una clave de otro ambiente sin llamar al SRI. `ride` rechaza un comprobante sin fecha de autorización.
- Deuda de API conocida: `sri.Plan` es un nombre genérico y `Withholding.Support` admite un solo documento de sustento (cambiará de forma incompatible al admitir varios). Los textos libres de más de 300 caracteres se cortan sin avisar.
