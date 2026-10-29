package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type BillToDTO struct {
	LegalName    string  `json:"legal_name"`
	Email        string  `json:"email"`
	Phone        *string `json:"phone"`
	AddressLine1 *string `json:"address_line1"`
	AddressLine2 *string `json:"address_line2"`
	City         *string `json:"city"`
	State        *string `json:"state"`
	PostalCode   *string `json:"postal_code"`
	Country      *string `json:"country" doc:"ISO 3166-1 alpha-2"`
	TaxIDType    *string `json:"tax_id_type"`
	TaxID        *string `json:"tax_id"`
}

type ProfileOutput struct {
	Body struct {
		BillToDTO
		UpdatedAt time.Time `json:"updated_at"`
	}
}

func ToProfileOutput(p model.Profile) *ProfileOutput {
	out := &ProfileOutput{}
	out.Body.BillToDTO = BillToDTO{
		LegalName: p.LegalName, Email: p.Email, Phone: p.Phone, AddressLine1: p.AddressLine1,
		AddressLine2: p.AddressLine2, City: p.City, State: p.State, PostalCode: p.PostalCode, Country: p.Country,
		TaxIDType: p.TaxIDType, TaxID: p.TaxID,
	}
	out.Body.UpdatedAt = p.UpdatedAt
	return out
}

type UpdateProfileInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		LegalName    string  `json:"legal_name" minLength:"1" maxLength:"200" doc:"Person or company invoices are made out to"`
		Email        string  `json:"email" format:"email" maxLength:"254" doc:"Where invoices and payment emails go"`
		Phone        *string `json:"phone,omitempty" maxLength:"32"`
		AddressLine1 *string `json:"address_line1,omitempty" maxLength:"200"`
		AddressLine2 *string `json:"address_line2,omitempty" maxLength:"200"`
		City         *string `json:"city,omitempty" maxLength:"100"`
		State        *string `json:"state,omitempty" maxLength:"100"`
		PostalCode   *string `json:"postal_code,omitempty" maxLength:"20"`
		Country      *string `json:"country,omitempty" pattern:"^[A-Z]{2}$" doc:"ISO 3166-1 alpha-2"`
		TaxIDType    *string `json:"tax_id_type,omitempty" maxLength:"32" doc:"e.g. eu_vat, bd_bin; set with tax_id"`
		TaxID        *string `json:"tax_id,omitempty" maxLength:"64"`
	}
}

func (in *UpdateProfileInput) ToModel() model.ProfileInput {
	b := in.Body
	return model.ProfileInput{
		LegalName: b.LegalName, Email: b.Email, Phone: b.Phone, AddressLine1: b.AddressLine1,
		AddressLine2: b.AddressLine2, City: b.City, State: b.State, PostalCode: b.PostalCode, Country: b.Country,
		TaxIDType: b.TaxIDType, TaxID: b.TaxID,
	}
}
