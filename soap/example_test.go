package soap_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nelsonmarro/go_ec_sri"
	"github.com/nelsonmarro/go_ec_sri/soap"
)

// The client makes one call per method: the caller decides when to ask again.
func ExampleClient_Send() {
	client := soap.New(soap.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}))

	var signedXML []byte // from sign.Sign
	rec, err := client.Send(context.Background(), sri.EnvironmentTest, signedXML)
	if err != nil {
		fmt.Println("retry later:", soap.Retryable(err), err)
		return
	}
	if rec.Status == sri.ReceptionReturned {
		for _, m := range rec.Messages {
			fmt.Println(sri.Explain(m).Title)
		}
	}
}

func ExampleClient_Authorize() {
	client := soap.New()

	auth, err := client.Authorize(context.Background(), sri.EnvironmentTest, "2611202601179000000200110010010000001230000000112")
	if err != nil {
		fmt.Println(err)
		return
	}
	switch auth.Status {
	case sri.StatusAuthorized:
		envelope, _ := sri.AuthorizedXML(*auth) // the file to keep
		fmt.Println(len(envelope))
	case sri.StatusInProcess, sri.StatusNone:
		// Ask again later; the library never polls.
	case sri.StatusNotAuthorized:
		for _, m := range auth.Messages {
			fmt.Println(sri.ExplainFor(sri.DocInvoice, m).Advice)
		}
	}
}
