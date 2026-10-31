package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

func (h *Handler) listSegments(ctx context.Context, in *dto.WorkspaceInput) (*dto.SegmentsOutput, error) {
	segs, err := h.svc.Segments(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.SegmentsOutput{}
	out.Body.Items = make([]dto.SegmentDTO, len(segs))
	for i, s := range segs {
		out.Body.Items[i] = dto.ToSegmentDTO(s)
	}
	return out, nil
}

func (h *Handler) getSegment(ctx context.Context, in *dto.SegmentPath) (*dto.SegmentOutput, error) {
	return segmentOutput(h.svc.Segment(ctx, in.WorkspaceID, in.SegmentID))
}

func (h *Handler) createSegment(ctx context.Context, in *dto.CreateSegmentInput) (*dto.SegmentOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return segmentOutput(h.svc.CreateSegment(ctx, in.WorkspaceID, userID, in.Body.Input()))
}

func (h *Handler) updateSegment(ctx context.Context, in *dto.UpdateSegmentInput) (*dto.SegmentOutput, error) {
	return segmentOutput(h.svc.UpdateSegment(ctx, in.WorkspaceID, in.SegmentID, in.Body.Input()))
}

func (h *Handler) deleteSegment(ctx context.Context, in *dto.SegmentPath) (*struct{}, error) {
	return nil, h.svc.DeleteSegment(ctx, in.WorkspaceID, in.SegmentID)
}

func (h *Handler) previewSegment(ctx context.Context, in *dto.PreviewSegmentInput) (*dto.SegmentPreviewOutput, error) {
	p, err := h.svc.PreviewSegment(ctx, in.WorkspaceID, in.Body.Rules.Rules())
	if err != nil {
		return nil, err
	}
	return dto.ToSegmentPreviewOutput(p), nil
}

func segmentOutput(s model.Segment, err error) (*dto.SegmentOutput, error) {
	if err != nil {
		return nil, err
	}
	return &dto.SegmentOutput{Body: dto.ToSegmentDTO(s)}, nil
}
