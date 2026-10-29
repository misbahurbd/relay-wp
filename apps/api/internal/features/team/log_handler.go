package team

import (
	"context"
)

// ---------- access log ----------

func (h *Handler) accessLog(ctx context.Context, in *listLogInput) (*logOutput, error) {
	f := LogFilter{Action: optional(in.Action), SubjectID: optionalID(in.SubjectID), Since: in.Since, PageSize: in.PageSize}
	if in.Cursor != "" {
		t, id, ok := decodeCursor(in.Cursor)
		if !ok {
			return nil, errBadCursor
		}
		f.Before = &LogCursor{CreatedAt: t, ID: id}
	}
	entries, next, err := h.svc.ListAccessLog(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &logOutput{}
	out.Body.Entries = make([]logDTO, len(entries))
	for i, e := range entries {
		out.Body.Entries[i] = toLogDTO(e)
	}
	if next != nil {
		c := encodeCursor(next.CreatedAt, next.ID)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) accessLogSummary(ctx context.Context, in *workspaceInput) (*logSummaryOutput, error) {
	sum, err := h.svc.AccessLogSummary(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &logSummaryOutput{}
	out.Body.Events, out.Body.PermissionChanges = sum.Events, sum.PermissionChanges
	out.Body.Expiring = make([]expiringDTO, len(sum.Expiring))
	for i, e := range sum.Expiring {
		out.Body.Expiring[i] = expiringDTO(e)
	}
	out.Body.Alerts = make([]loginAlertDTO, len(sum.Alerts))
	for i, a := range sum.Alerts {
		out.Body.Alerts[i] = loginAlertDTO(a)
	}
	return out, nil
}
