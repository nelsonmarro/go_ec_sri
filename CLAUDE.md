# go_ec_sri — reglas del proyecto

Librería pública (MIT) de facturación electrónica del SRI de Ecuador, extraída del motor de Verith. Módulo `github.com/nelsonmarro/go_ec_sri`.

## Paquetes

- `sri` (raíz): clave de acceso, tipos de documento y ambientes, constructores de XML, validación y firma.
- Los otros tres paquetes (definidos en la spec) cubren la firma XAdES-BES, el cliente SOAP (`soap`, único con red, mediante un `*http.Client` inyectado) y el RIDE/ayudas de lectura.

## Reglas

1. **TDD siempre** (RED visto fallar por la razón correcta, GREEN, refactor) y al menos una mutación manual por comportamiento clave. Cobertura >= 80 % por paquete. Los tests que protegen códigos, tarifas o importes usan literales.
2. **Dinero, cantidades y tarifas en `decimal.Decimal`**; nunca `float64`.
3. **Español** en los mensajes al usuario (`ValidationError.Message`, explicaciones); **inglés técnico** en código y comentarios. Ningún mensaje menciona a Verith ni sus pantallas.
4. Ninguna función lee ni escribe archivos ni guarda estado; solo `soap` hace red.
5. **Sin datos reales ni secretos**: RUC, cédulas, nombres, direcciones, correos y claves de acceso son ficticios. Los RUC ficticios son válidos en estructura (13 dígitos que terminan en `001`) y las cédulas pasan el módulo 10. Sin `.p12` ni `.env` en el repo; la clave del certificado de prueba es la constante `testPassphrase = "prueba"`.
6. XML de salida validado con `xmllint` contra los XSD oficiales y con archivos golden.
7. Commits convencionales, descripción en español.

## Verificación

```
go build ./... && go vet ./... && staticcheck ./... && go test -race ./...
govulncheck ./...
```
