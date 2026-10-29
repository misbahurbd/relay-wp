package handler

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/dto"
)

func (h *Handler) uploadImport(ctx context.Context, in *dto.UploadImportInput) (*dto.UploadOutput, error) {
	f := in.RawBody.Data().File
	defer func() { _ = f.Close() }()
	u, err := h.svc.Upload(ctx, actor(ctx), f.Filename, f.Size, f)
	if err != nil {
		return nil, err
	}
	return dto.ToUploadOutput(u), nil
}

func (h *Handler) setMapping(ctx context.Context, in *dto.MappingInput) (*dto.ImportOutput, error) {
	i, err := h.svc.SetMapping(ctx, in.WorkspaceID, in.ImportID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return &dto.ImportOutput{Body: dto.ToImportDTO(i)}, nil
}

func (h *Handler) previewImport(ctx context.Context, in *dto.ImportPath) (*dto.PreviewOutput, error) {
	p, err := h.svc.Preview(ctx, in.WorkspaceID, in.ImportID)
	if err != nil {
		return nil, err
	}
	return dto.ToPreviewOutput(p), nil
}

func (h *Handler) startImport(ctx context.Context, in *dto.ImportPath) (*dto.ImportOutput, error) {
	i, err := h.svc.Start(ctx, in.WorkspaceID, in.ImportID)
	if err != nil {
		return nil, err
	}
	return &dto.ImportOutput{Body: dto.ToImportDTO(i)}, nil
}

func (h *Handler) getImport(ctx context.Context, in *dto.ImportPath) (*dto.ImportOutput, error) {
	i, err := h.svc.GetImport(ctx, in.WorkspaceID, in.ImportID)
	if err != nil {
		return nil, err
	}
	return &dto.ImportOutput{Body: dto.ToImportDTO(i)}, nil
}

func (h *Handler) skippedRows(ctx context.Context, in *dto.ImportPath) (*huma.StreamResponse, error) {
	f, err := h.svc.SkippedRows(ctx, in.WorkspaceID, in.ImportID)
	if err != nil {
		return nil, err
	}
	return streamFile(ctx, f, "skipped-rows.csv"), nil
}
