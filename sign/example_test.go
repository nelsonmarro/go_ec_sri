package sign_test

import (
	"fmt"
	"os"

	"github.com/nelsonmarro/go_ec_sri"
	"github.com/nelsonmarro/go_ec_sri/sign"
)

// The library never reads files: the caller loads the .p12 and its password.
func ExampleSign() {
	p12, err := os.ReadFile("firma.p12")
	if err != nil {
		fmt.Println(err)
		return
	}
	password := os.Getenv("SRI_P12_PASSWORD")

	var unsigned []byte // from sri.InvoiceXML
	signed, err := sign.Sign(sri.DocInvoice, unsigned, p12, password)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(signed))
}

func ExampleInspect() {
	p12, _ := os.ReadFile("firma.p12")

	cert, err := sign.Inspect(p12, os.Getenv("SRI_P12_PASSWORD"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(cert.Subject, cert.NotAfter.Format("2006-01-02"))
}
