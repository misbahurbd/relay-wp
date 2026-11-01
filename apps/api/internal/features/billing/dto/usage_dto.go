package dto

import (
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type MonthlyUsageDTO struct {
	Resource string `json:"resource" example:"broadcasts" doc:"A monthly resource key from GET /v1/billing/resources"`
	Used     int    `json:"used"`
	Limit    int    `json:"limit" doc:"-1 unlimited, 0 not included"`
}

type UsageOutput struct {
	Body struct {
		PeriodStart time.Time         `json:"period_start" doc:"First instant of this calendar month (UTC)"`
		PeriodEnd   time.Time         `json:"period_end" doc:"When the counts start again from 0"`
		Resources   []MonthlyUsageDTO `json:"resources" doc:"Every monthly resource, in catalog order"`
	}
}

func ToUsageOutput(u model.Usage) *UsageOutput {
	out := &UsageOutput{}
	out.Body.PeriodStart, out.Body.PeriodEnd = u.PeriodStart, u.PeriodEnd
	out.Body.Resources = make([]MonthlyUsageDTO, 0, len(u.Resources))
	for _, r := range u.Resources {
		out.Body.Resources = append(out.Body.Resources, MonthlyUsageDTO{Resource: r.Resource, Used: r.Used, Limit: r.Limit})
	}
	return out
}
