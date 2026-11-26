# go_ec_sri — reglas del proyecto

Librería pública (MIT) de facturación electrónica del SRI de Ecuador, extraída del motor de Verith. Módulo `github.com/nelsonmarro/go_ec_sri`.

## Paquetes

- `sri` (raíz): clave de acceso, RUC y cédula, tipos y ambientes, constructores de XML (factura 01, nota de crédito 04, retención 07), validación con códigos estables, lectura de los tres comprobantes, sobre autorizado, `Explain`/`ExplainFor` y `ReissuePlan`. `sri.Today()` entrega la fecha de Ecuador para `Header.IssueDate`; la librería imprime la fecha de calendario tal como viene y no la convierte.
- `sign`: firma XAdES-BES desde un `.p12` en memoria e `Inspect`.
- `soap`: cliente de recepción y autorización (único con red, mediante un `*http.Client` inyectado); errores clasificados en español y `Retryable`; sin reintentos. `Authorize` rechaza una clave de otro ambiente.
- `ride`: RIDE en PDF de factura, nota de crédito y retención; exige el sobre `<autorizacion>` con número y fecha.

`FinalConsumerLimit()` y `soap.DefaultURLs()` son funciones (solo lectura); no exponer variables globales mutables.

## Reglas

1. **TDD siempre** (RED visto fallar por la razón correcta, GREEN, refactor) y al menos una mutación manual por comportamiento clave. Cobertura >= 80 % por paquete. Los tests que protegen códigos, tarifas o importes usan literales.
2. **Dinero, cantidades y tarifas en `decimal.Decimal`**; nunca `float64`.
3. **Español** en los mensajes al usuario (`ValidationError.Message`, explicaciones); **inglés técnico** en código y comentarios. Ningún mensaje menciona a Verith ni sus pantallas.
4. Ninguna función lee ni escribe archivos ni guarda estado; solo `soap` hace red.
5. **Sin datos reales ni secretos**: RUC, cédulas, nombres, direcciones, correos y claves de acceso son ficticios. Los RUC ficticios son válidos en estructura (13 dígitos que terminan en `001`) y las cédulas pasan el módulo 10. Sin `.p12` ni `.env` en el repo; la clave del certificado de prueba es la constante `testPassphrase = "prueba"`.
6. XML de salida validado con `xmllint` contra los XSD oficiales y con archivos golden; bajo CI (`CI` definida) la falta de `xmllint` hace fallar las pruebas en lugar de saltarlas.
7. Commits convencionales, descripción en español.

## Verificación

```
go build ./... && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go test -race -cover ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```
(El binario local de `staticcheck` falla con go1.27: se ejecuta con `go run`. El CI fija las versiones.)
