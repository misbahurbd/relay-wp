package model

import "time"

// Profile is a workspace's billing details, the same for every provider.
// Personal data: never logged.
type Profile struct {
	LegalName    string
	Email        string
	Phone        *string
	AddressLine1 *string
	AddressLine2 *string
	City         *string
	State        *string
	PostalCode   *string
	Country      *string // ISO 3166-1 alpha-2
	TaxIDType    *string
	TaxID        *string
	UpdatedAt    time.Time
}

// ProfileInput replaces a workspace's billing details.
type ProfileInput struct {
	LegalName    string
	Email        string
	Phone        *string
	AddressLine1 *string
	AddressLine2 *string
	City         *string
	State        *string
	PostalCode   *string
	Country      *string
	TaxIDType    *string
	TaxID        *string
}

// BillTo is the copy of a profile taken when an invoice is issued
// (invoices.bill_to). It never changes after.
type BillTo struct {
	LegalName    string  `json:"legal_name"`
	Email        string  `json:"email"`
	Phone        *string `json:"phone,omitempty"`
	AddressLine1 *string `json:"address_line1,omitempty"`
	AddressLine2 *string `json:"address_line2,omitempty"`
	City         *string `json:"city,omitempty"`
	State        *string `json:"state,omitempty"`
	PostalCode   *string `json:"postal_code,omitempty"`
	Country      *string `json:"country,omitempty"`
	TaxIDType    *string `json:"tax_id_type,omitempty"`
	TaxID        *string `json:"tax_id,omitempty"`
}
