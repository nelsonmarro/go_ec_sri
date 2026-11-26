package ride_test

import (
	"bytes"
	"fmt"
	"os"

	"github.com/nelsonmarro/go_ec_sri/ride"
)

// The RIDE needs the <autorizacion> envelope (sri.AuthorizedXML).
func ExampleInvoice() {
	authorized, err := os.ReadFile("factura-autorizada.xml")
	if err != nil {
		fmt.Println(err)
		return
	}
	logo, _ := os.ReadFile("logo.png") // optional

	var pdf bytes.Buffer
	if err := ride.Invoice(&pdf, authorized, ride.Options{Logo: logo}); err != nil {
		fmt.Println(err)
		return
	}
	_ = os.WriteFile("factura.pdf", pdf.Bytes(), 0o644)
}
