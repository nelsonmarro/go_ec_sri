# go_ec_sri

Librería Go para la facturación electrónica del SRI de Ecuador: clave de acceso, XML de factura, nota de crédito y retención, validación con mensajes en español, firma XAdES-BES, cliente SOAP y RIDE en PDF. Sin estado y sin archivos: tú pasas los bytes y la librería devuelve bytes.

**English.** Go library for Ecuador's SRI electronic invoicing: access keys, XML (invoice 1.1.0, credit note 1.1.0, withholding 2.0.0), validation with Spanish user-facing messages, XAdES-BES signing, SOAP client for the reception/authorization services, and PDF RIDE. It keeps no state, reads no files and retries nothing; only the `soap` package uses the network, through an injected `*http.Client`.

## Instalación

```
go get github.com/nelsonmarro/go_ec_sri@latest
```

## Paquetes

| Paquete | Qué hace |
|---|---|
| `github.com/nelsonmarro/go_ec_sri` (`sri`) | Clave de acceso, tipos, validación, XML de factura (01), nota de crédito (04) y retención (07), lectura de esos XML, sobre `<autorizacion>`, explicación de mensajes del SRI. |
| `.../sign` | Firma XAdES-BES (SHA-1, lo que acepta el SRI) desde un `.p12` en memoria; `Inspect` lo valida. |
| `.../soap` | Cliente de recepción y autorización. Una llamada por método; sin reintentos ni sondeo. |
| `.../ride` | RIDE en PDF de factura, nota de crédito y retención autorizadas. |

## Ejemplo completo

```go
import (
	"bytes"
	"context"
	"os"
	"time"

	"github.com/nelsonmarro/go_ec_sri"
	"github.com/nelsonmarro/go_ec_sri/ride"
	"github.com/nelsonmarro/go_ec_sri/sign"
	"github.com/nelsonmarro/go_ec_sri/soap"
	"github.com/shopspring/decimal"
)

func emitir(ctx context.Context, p12 []byte, password string) error {
	inv := sri.Invoice{
		Header: sri.Header{
			Environment:   sri.EnvironmentTest,
			Establishment: "001",
			EmissionPoint: "001",
			Sequential:    "123", // tú llevas el secuencial; se completa a 9 dígitos
			IssueDate:     sri.Today(), // fecha de Ecuador; ver nota abajo
			// AccessKey vacío: la librería genera la clave con crypto/rand.
		},
		Issuer: sri.Issuer{
			RUC:             "1790000002001",
			BusinessName:    "EMPRESA DE PRUEBA S.A.",
			MainAddress:     "Av. Ficticia 123 y Calle Falsa",
			KeepsAccounting: true,
		},
		Buyer: sri.Party{
			IDType: sri.IDCedula, ID: "1700000001", Name: "JUAN PEREZ",
			Email: "comprador@example.com",
		},
		Lines: []sri.Line{{
			Code: "SERV01", Description: "Servicio de consultoría",
			Quantity:  decimal.RequireFromString("2"),
			UnitPrice: decimal.RequireFromString("10.00"),
			TaxRate:   15,
		}},
		Payments: []sri.Payment{{Method: "20", Amount: decimal.RequireFromString("23.00")}},
		// El correo y el teléfono del comprador NO se imprimen solos:
		// agrégalos como campos adicionales.
		Additional: []sri.AdditionalField{{Name: "Email", Value: "comprador@example.com"}},
	}

	built, err := sri.InvoiceXML(inv) // valida y arma el XML sin firmar
	if err != nil {
		return err // sri.ValidationErrors
	}
	signed, err := sign.Sign(sri.DocInvoice, built.XML, p12, password)
	if err != nil {
		return err
	}

	client := soap.New()
	rec, err := client.Send(ctx, sri.EnvironmentTest, signed)
	if err != nil {
		return err // soap.Retryable(err) dice si vale la pena reintentar
	}
	if rec.Status != sri.ReceptionReceived {
		return nil // DEVUELTA: sri.Explain(rec.Messages[i])
	}

	// Pregunta de nuevo más tarde si el estado es EN PROCESO o vacío.
	auth, err := client.Authorize(ctx, sri.EnvironmentTest, built.AccessKey)
	if err != nil || auth.Status != sri.StatusAuthorized {
		return err
	}
	envelope, err := sri.AuthorizedXML(*auth) // el comprobante autorizado que conservas
	if err != nil {
		return err
	}
	var pdf bytes.Buffer
	if err := ride.Invoice(&pdf, envelope, ride.Options{}); err != nil {
		return err
	}
	return os.WriteFile("factura.pdf", pdf.Bytes(), 0o644)
}
```

`go doc` y los `Example…` de cada paquete traen más casos (generar clave, validar, explicar un rechazo, leer un XML).

## Errores de validación

`InvoiceXML`, `CreditNoteXML` y `WithholdingXML` (y los métodos `Validate`) devuelven `sri.ValidationErrors` con **todos** los problemas, no solo el primero. Cada `ValidationError` trae `Field` (`lines[0].quantity`), `Code` estable en inglés (constantes `sri.Code…`) y `Message` en español apto para el usuario final:

```go
var verrs sri.ValidationErrors
if errors.As(err, &verrs) {
	for _, e := range verrs {
		fmt.Println(e.Field, e.Code, e.Message)
	}
}
```

Para un rechazo del SRI usa `sri.ExplainFor(doc, mensaje)`: título y consejo en español, dónde corregirlo (`FixIn`) y el texto original. `sri.ReissuePlan(mensajes)` dice si hay que cambiar secuencial o clave al reenviar.

## Documentos y versiones

| Documento | Código | Esquema | Emite | Lee |
|---|---|---|---|---|
| Factura | 01 | 1.1.0 | sí | 1.0.0 y 1.1.0 |
| Nota de crédito | 04 | 1.1.0 | sí | 1.0.0 y 1.1.0 |
| Comprobante de retención | 07 | 2.0.0 | sí | 2.0.0 y 1.0.0 |

El XML de salida se valida contra los XSD oficiales en las pruebas.

## Qué no hace

La librería es un motor de documentos, no una aplicación. **No** hace:

- **Secuenciales**: tú los llevas y los entregas en `Header.Sequential`.
- **Colas, reintentos ni sondeo**: `soap` hace una llamada por método; tú decides cuándo volver a preguntar (`soap.Retryable` ayuda).
- **Almacenamiento**: no lee ni escribe archivos, certificados ni comprobantes.
- **Correo**: no envía el comprobante al cliente.
- **Plazos de negocio**: por ejemplo el límite de 12 meses de una nota de crédito lo decide quien llama.
- **Catálogo de conceptos de retención**: `WithholdingLine.Code` se valida en forma, no contra el catálogo del ATS.

### Límites de la v0.1.0

- `Header.IssueDate`: se imprime su fecha de calendario (año, mes y día tal como vienen, en la zona del valor) en el XML y en la clave de acceso. Debe ser la fecha de Ecuador: usa `sri.Today()` o una columna `DATE`; un instante tomado en un servidor en UTC imprimiría el día siguiente después de las 19:00 de Ecuador.
- Los textos libres de más de 300 caracteres (nombres, direcciones, descripciones de ítems) se cortan sin avisar; el motivo de una nota de crédito y los campos adicionales de más de 300 son un error de validación.
- Una factura de otro sistema con propina: `importeTotal` la incluye, pero los totales calculados y el RIDE no tienen una fila de propina.

- Solo IVA 0 % y 15 %. Al leer, un comprobante de otro sistema con ICE, IRBPNR u otra tarifa se rechaza.
- La retención admite **un** documento de sustento, al emitir y al leer (uno con varios sustentos se rechaza).
- Sin proveedores del exterior (08) en la retención.
- La leyenda RIMPE es la única del XSD: emprendedor y negocio popular se imprimen igual y al leer vuelve como emprendedor.
- El correo y el teléfono del comprador no se imprimen solos; agrégalos en `Additional` (ver el ejemplo).
- Si la respuesta AUTORIZADO trae una fecha ilegible, `Authorization.Date` queda en cero; quien llama decide qué hacer.
- La fecha de una factura leída vuelve a medianoche de Ecuador y los importes conservan la escala escrita en el XML.

## Prueba contra el SRI real

Las pruebas normales no usan la red. La de integración llama al ambiente de pruebas del SRI:

```
go test -tags sriintegration ./soap/
```

## Licencia

MIT. Ver [LICENSE](LICENSE).
