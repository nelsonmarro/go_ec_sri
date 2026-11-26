package sri_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nelsonmarro/go_ec_sri"
	"github.com/shopspring/decimal"
)

var exampleDate = time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

func exampleInvoice() sri.Invoice {
	return sri.Invoice{
		Header: sri.Header{
			Environment:   sri.EnvironmentTest,
			Establishment: "001",
			EmissionPoint: "001",
			Sequential:    "123",
			IssueDate:     exampleDate,
			// Fixed key so the example is deterministic; leave it empty to
			// have the library generate one.
			AccessKey: "2611202601179000000200110010010000001230000000112",
		},
		Issuer: sri.Issuer{
			RUC:             "1790000002001",
			BusinessName:    "EMPRESA DE PRUEBA S.A.",
			MainAddress:     "Av. Ficticia 123 y Calle Falsa",
			KeepsAccounting: true,
		},
		Buyer: sri.Party{
			IDType: sri.IDCedula,
			ID:     "1700000001",
			Name:   "JUAN PEREZ",
			Email:  "comprador@example.com",
			Phone:  "0999999999",
		},
		Lines: []sri.Line{{
			Code:        "SERV01",
			Description: "Servicio de consultoría",
			Quantity:    decimal.RequireFromString("2"),
			UnitPrice:   decimal.RequireFromString("10.00"),
			TaxRate:     15,
		}},
		Payments: []sri.Payment{{Method: "20", Amount: decimal.RequireFromString("23.00")}},
		// The buyer's email and phone are not printed automatically:
		// add them as additional fields.
		Additional: []sri.AdditionalField{
			{Name: "Email", Value: "comprador@example.com"},
			{Name: "Teléfono", Value: "0999999999"},
		},
	}
}

func ExampleGenerateAccessKey() {
	// The last argument is the source of the 8-digit numeric code:
	// crypto/rand.Reader in production, a fixed reader in tests.
	key, err := sri.GenerateAccessKey(exampleDate, sri.DocInvoice, "1790000002001",
		sri.EnvironmentTest, "001", "001", "000000123", bytes.NewReader([]byte{0, 0, 0, 1}))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(key, sri.ValidAccessKey(key))
	// Output:
	// 2611202601179000000200110010010000001230000000112 true
}

func ExampleInvoiceXML() {
	built, err := sri.InvoiceXML(exampleInvoice())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(built.AccessKey)
	fmt.Println(strings.Contains(string(built.XML), "<importeTotal>23.00</importeTotal>"))
	// Output:
	// 2611202601179000000200110010010000001230000000112
	// true
}

func ExampleInvoice_Validate() {
	inv := exampleInvoice()
	inv.Lines = nil

	err := inv.Validate()

	var verrs sri.ValidationErrors
	if errors.As(err, &verrs) {
		for _, e := range verrs {
			fmt.Printf("%s | %s | %s\n", e.Field, e.Code, e.Message)
		}
	}
	// Output:
	// lines | lines_required | El comprobante no tiene ítems.
	// payments | payments_total_mismatch | Los pagos no suman el total del comprobante.
}

func ExampleExplainFor() {
	e := sri.ExplainFor(sri.DocInvoice, sri.Message{
		Identifier: "45",
		Text:       "ERROR SECUENCIAL REGISTRADO",
		Type:       sri.MessageError,
	})
	fmt.Println(e.Code, e.FixIn)
	fmt.Println(e.Title)
	// Output:
	// 45 emission_point
	// Ese número de comprobante ya fue usado
}

func ExampleParseInvoice() {
	built, _ := sri.InvoiceXML(exampleInvoice())
	parsed, err := sri.ParseInvoice(built.XML)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(parsed.Invoice.Buyer.Name, parsed.Totals.Total, parsed.Authorization == nil)
	// Output:
	// JUAN PEREZ 23 true
}
