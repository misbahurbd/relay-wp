package service

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Track T3: billing profiles. Personal data: never log a profile.

var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

// Profile returns the workspace's billing details. model.ErrProfileNotFound if
// none are saved yet.
func (s *Service) Profile(ctx context.Context, workspaceID uuid.UUID) (model.Profile, error) {
	row, err := repository.NewRepository(s.pool).GetBillingProfile(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.Profile{}, model.ErrProfileNotFound
	}
	if err != nil {
		return model.Profile{}, fmt.Errorf("get billing profile: %w", err)
	}
	return model.Profile{
		LegalName: row.LegalName, Email: row.Email, Phone: row.Phone, AddressLine1: row.AddressLine1,
		AddressLine2: row.AddressLine2, City: row.City, State: row.State, PostalCode: row.PostalCode,
		Country: row.Country, TaxIDType: row.TaxIDType, TaxID: row.TaxID, UpdatedAt: row.UpdatedAt,
	}, nil
}

// UpdateProfile replaces the workspace's billing details. Past invoices keep
// the bill_to they were issued with.
func (s *Service) UpdateProfile(ctx context.Context, workspaceID uuid.UUID, in model.ProfileInput) (model.Profile, error) {
	params, err := profileParams(workspaceID, in)
	if err != nil {
		return model.Profile{}, err
	}
	row, err := repository.NewRepository(s.pool).UpsertBillingProfile(ctx, params)
	if err != nil {
		return model.Profile{}, fmt.Errorf("save billing profile: %w", err)
	}
	return model.Profile{
		LegalName: row.LegalName, Email: row.Email, Phone: row.Phone, AddressLine1: row.AddressLine1,
		AddressLine2: row.AddressLine2, City: row.City, State: row.State, PostalCode: row.PostalCode,
		Country: row.Country, TaxIDType: row.TaxIDType, TaxID: row.TaxID, UpdatedAt: row.UpdatedAt,
	}, nil
}

// profileParams trims and checks a profile. Blank optional fields are stored as null.
func profileParams(workspaceID uuid.UUID, in model.ProfileInput) (gen.UpsertBillingProfileParams, error) {
	var details []apperr.Detail
	p := gen.UpsertBillingProfileParams{
		WorkspaceID: workspaceID, LegalName: strings.TrimSpace(in.LegalName), Email: strings.TrimSpace(in.Email),
		Phone: blankToNil(in.Phone), AddressLine1: blankToNil(in.AddressLine1), AddressLine2: blankToNil(in.AddressLine2),
		City: blankToNil(in.City), State: blankToNil(in.State), PostalCode: blankToNil(in.PostalCode),
		Country: blankToNil(in.Country), TaxIDType: blankToNil(in.TaxIDType), TaxID: blankToNil(in.TaxID),
	}
	if p.LegalName == "" {
		details = append(details, apperr.Detail{Field: "legal_name", Message: "Enter the name to bill"})
	}
	if a, err := mail.ParseAddress(p.Email); err != nil || a.Address != p.Email {
		details = append(details, apperr.Detail{Field: "email", Message: "Enter a valid email address"})
	}
	if p.Country != nil {
		c := strings.ToUpper(*p.Country)
		p.Country = &c
		if !countryCode.MatchString(c) {
			details = append(details, apperr.Detail{Field: "country", Message: "Use a two-letter country code"})
		}
	}
	if (p.TaxID == nil) != (p.TaxIDType == nil) {
		details = append(details, apperr.Detail{Field: "tax_id", Message: "Give both the tax ID and its type, or neither"})
	}
	if len(details) > 0 {
		return gen.UpsertBillingProfileParams{}, model.ErrInvalidInput.WithDetails(details...)
	}
	return p, nil
}

// BillTo copies the workspace's billing details for a new invoice's bill_to,
// in tx. Every track that issues an invoice uses it.
func (s *Service) BillTo(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.BillTo, error) {
	row, err := repository.NewRepository(tx).GetBillingProfile(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.BillTo{}, model.ErrProfileNotFound
	}
	if err != nil {
		return model.BillTo{}, fmt.Errorf("get billing profile: %w", err)
	}
	return model.BillTo{
		LegalName: row.LegalName, Email: row.Email, Phone: row.Phone, AddressLine1: row.AddressLine1,
		AddressLine2: row.AddressLine2, City: row.City, State: row.State, PostalCode: row.PostalCode,
		Country: row.Country, TaxIDType: row.TaxIDType, TaxID: row.TaxID,
	}, nil
}

// blankToNil trims s and returns nil when nothing is left.
func blankToNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
