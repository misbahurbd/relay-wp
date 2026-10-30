package dto

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type PaymentWebhookInput struct {
	Provider string `path:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	RawBody  []byte
	// Header is every request header: each provider signs with its own (Stripe-Signature, …).
	Header http.Header `json:"-"`
}

// Resolve copies the request headers, which the gateway needs to verify the signature.
func (in *PaymentWebhookInput) Resolve(ctx huma.Context) []error {
	in.Header = http.Header{}
	ctx.EachHeader(func(name, value string) { in.Header.Add(name, value) })
	return nil
}
