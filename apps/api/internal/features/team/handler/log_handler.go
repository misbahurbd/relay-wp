package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
)

// ---------- access log ----------

func (h *Handler) accessLog(ctx context.Context, in *dto.ListLogInput) (*dto.LogOutput, error) {
	f := model.LogFilter{Action: dto.Optional(in.Action), SubjectID: dto.OptionalID(in.SubjectID), Since: in.Since, PageSize: in.PageSize}
	if in.Cursor != "" {
		t, id, ok := dto.DecodeCursor(in.Cursor)
		if !ok {
			return nil, errBadCursor
		}
		f.Before = &model.LogCursor{CreatedAt: t, ID: id}
	}
	entries, next, err := h.svc.ListAccessLog(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &dto.LogOutput{}
	out.Body.Entries = make([]dto.LogDTO, len(entries))
	for i, e := range entries {
		out.Body.Entries[i] = dto.ToLogDTO(e)
	}
	if next != nil {
		c := dto.EncodeCursor(next.CreatedAt, next.ID)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) accessLogSummary(ctx context.Context, in *dto.WorkspaceInput) (*dto.LogSummaryOutput, error) {
	sum, err := h.svc.AccessLogSummary(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.LogSummaryOutput{}
	out.Body.Events, out.Body.PermissionChanges = sum.Events, sum.PermissionChanges
	out.Body.Expiring = make([]dto.ExpiringDTO, len(sum.Expiring))
	for i, e := range sum.Expiring {
		out.Body.Expiring[i] = dto.ExpiringDTO(e)
	}
	out.Body.Alerts = make([]dto.LoginAlertDTO, len(sum.Alerts))
	for i, a := range sum.Alerts {
		out.Body.Alerts[i] = dto.LoginAlertDTO(a)
	}
	return out, nil
}
