package model

import (
	"github.com/google/uuid"
)

// Team groups members for routing and chat visibility.
type Team struct {
	ID            uuid.UUID
	Name          string
	Description   string
	Color         string
	NumberID      *uuid.UUID
	RoutingMethod string
	MemberIDs     []uuid.UUID
}

// Details are a team's editable fields, for creating or editing it.
type Details struct {
	Name          string
	Description   string
	Color         string
	NumberID      *uuid.UUID
	RoutingMethod string
}
