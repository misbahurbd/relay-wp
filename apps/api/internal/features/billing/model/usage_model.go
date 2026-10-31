package model

import "time"

// PeriodStart is the first instant of t's calendar month (UTC): the month a
// monthly resource is counted in (ADR 0008).
func PeriodStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// MonthlyUsage is how much of one monthly resource a workspace used this month.
type MonthlyUsage struct {
	Resource string
	Used     int
	// Limit is the plan's cap: Unlimited, or NotIncluded when the plan leaves it out.
	Limit int
}

// Usage is a workspace's monthly resources for the current month.
type Usage struct {
	PeriodStart, PeriodEnd time.Time
	Resources              []MonthlyUsage
}
