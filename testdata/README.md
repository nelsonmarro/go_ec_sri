# testdata

Todos los datos de este directorio son ficticios: RUC y cédulas estructuralmente válidos pero inexistentes, nombres genéricos y direcciones inventadas. Nunca se agregan datos reales, certificados `.p12` ni secretos.

All data here is fictitious: structurally valid but non-existent RUC and cédulas, generic names and made-up addresses. Never add real data, `.p12` files or secrets.

- `golden/`: XML sin firmar que producen los constructores.
- `authorized/`: los mismos comprobantes dentro del sobre `<autorizacion>`, generados con `AuthorizedXML` (`go test -run TestAuthorizedSamples -update`).
- `read/`: comprobantes escritos por otros sistemas para probar la lectura: una retención 1.0.0 en ISO-8859-1 (la codificación es intencional; no la vuelvas a guardar como UTF-8), una retención 2.0.0 indentada en su sobre y una nota de crédito con el comprobante como texto escapado.
