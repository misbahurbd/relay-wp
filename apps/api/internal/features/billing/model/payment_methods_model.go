package model

// Card is the saved card the provider charges next. It is read from the
// provider on each request and never stored.
type Card struct {
	Brand    string
	Last4    string
	ExpMonth int
	ExpYear  int
}

// PaymentMethod is what the billing page shows about how the workspace pays.
type PaymentMethod struct {
	// Provider is the payment provider the card is with; empty when nothing was bought yet.
	Provider string
	// Card is nil with no customer, when the provider shares no card details, or when it didn't answer.
	Card *Card
	// CardDetails is false when the provider never shares card details (Creem).
	CardDetails bool
	// CanManage: the provider has a portal where the customer manages cards.
	CanManage bool
}
