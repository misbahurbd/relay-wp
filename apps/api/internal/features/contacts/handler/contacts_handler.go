package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

func (h *Handler) listContacts(ctx context.Context, in *dto.ListContactsInput) (*dto.ContactsOutput, error) {
	f := in.ToModel()
	f.Limit = in.Limit
	if in.Cursor != "" {
		id, ok := dto.DecodeCursor(in.Cursor)
		if !ok {
			return nil, model.ErrInvalidCursor
		}
		f.After = &id
	}
	page, err := h.svc.List(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &dto.ContactsOutput{}
	out.Body.Total = page.Total
	out.Body.Items = make([]dto.ContactDTO, len(page.Items))
	for i, c := range page.Items {
		out.Body.Items[i] = dto.ToContactDTO(c)
	}
	if page.Next != nil {
		c := dto.EncodeCursor(*page.Next)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) summary(ctx context.Context, in *dto.WorkspaceInput) (*dto.SummaryOutput, error) {
	s, err := h.svc.Summary(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.SummaryOutput{}
	out.Body.Total, out.Body.OptedIn = s.Total, s.OptedIn
	return out, nil
}

func (h *Handler) getContact(ctx context.Context, in *dto.ContactPath) (*dto.ContactOutput, error) {
	c, err := h.svc.Get(ctx, in.WorkspaceID, in.ContactID)
	if err != nil {
		return nil, err
	}
	return &dto.ContactOutput{Body: dto.ToContactDetailDTO(c)}, nil
}

func (h *Handler) createContact(ctx context.Context, in *dto.CreateContactInput) (*dto.ContactOutput, error) {
	c, err := h.svc.Create(ctx, in.WorkspaceID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return &dto.ContactOutput{Body: dto.ToContactDetailDTO(c)}, nil
}

func (h *Handler) updateContact(ctx context.Context, in *dto.UpdateContactInput) (*dto.ContactOutput, error) {
	c, err := h.svc.Update(ctx, in.WorkspaceID, in.ContactID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return &dto.ContactOutput{Body: dto.ToContactDetailDTO(c)}, nil
}

func (h *Handler) deleteContact(ctx context.Context, in *dto.ContactPath) (*struct{}, error) {
	return nil, h.svc.Delete(ctx, in.WorkspaceID, in.ContactID)
}

func (h *Handler) bulk(ctx context.Context, in *dto.BulkInput) (*dto.BulkOutput, error) {
	n, err := h.svc.Bulk(ctx, actor(ctx), in.ToModel())
	if err != nil {
		return nil, err
	}
	out := &dto.BulkOutput{}
	out.Body.Affected = n
	return out, nil
}

func (h *Handler) export(ctx context.Context, in *dto.ExportContactsInput) (*huma.StreamResponse, error) {
	write, err := h.svc.Export(ctx, in.WorkspaceID, in.ToModel())
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("contacts-%s.csv", time.Now().UTC().Format(time.DateOnly))
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		hctx.SetHeader("Content-Type", "text/csv; charset=utf-8")
		hctx.SetHeader("Content-Disposition", `attachment; filename="`+name+`"`)
		hctx.SetHeader("Cache-Control", "no-store")
		hctx.SetStatus(http.StatusOK)
		if err := write(hctx.BodyWriter()); err != nil {
			// The status is already sent; the client gets a cut-off file.
			logger.FromContext(ctx).Error("contacts export failed", "err", err)
		}
	}}, nil
}

// streamFile sends a stored CSV as a download.
func streamFile(ctx context.Context, f io.ReadCloser, name string) *huma.StreamResponse {
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		defer func() { _ = f.Close() }()
		hctx.SetHeader("Content-Type", "text/csv; charset=utf-8")
		hctx.SetHeader("Content-Disposition", `attachment; filename="`+name+`"`)
		hctx.SetHeader("Cache-Control", "no-store")
		hctx.SetStatus(http.StatusOK)
		if _, err := io.Copy(hctx.BodyWriter(), f); err != nil {
			logger.FromContext(ctx).Error("file download failed", "file", name, "err", err)
		}
	}}
}
