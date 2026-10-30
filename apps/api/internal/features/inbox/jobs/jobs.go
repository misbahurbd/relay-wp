// Package jobs holds the inbox's River job workers (docs/PHASE-7-INBOX.md AD-5, AD-11).
package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
)

// SendWorker works SendArgs.
type SendWorker struct {
	river.WorkerDefaults[model.SendArgs]
	svc *service.Service
}

// NewSendWorker builds the worker.
func NewSendWorker(svc *service.Service) *SendWorker { return &SendWorker{svc: svc} }

// Work sends the message. The channel's permanent refusals fail it at once; other
// errors retry with River's backoff, and the last attempt fails the message.
func (w *SendWorker) Work(ctx context.Context, job *river.Job[model.SendArgs]) error {
	return w.svc.Deliver(ctx, job.Args.WorkspaceID, job.Args.MessageID, job.Attempt >= job.MaxAttempts)
}

// Timeout covers uploading a 100 MB document to the channel.
func (w *SendWorker) Timeout(*river.Job[model.SendArgs]) time.Duration { return 5 * time.Minute }

// ReactWorker works ReactArgs.
type ReactWorker struct {
	river.WorkerDefaults[model.ReactArgs]
	svc *service.Service
}

// NewReactWorker builds the worker.
func NewReactWorker(svc *service.Service) *ReactWorker { return &ReactWorker{svc: svc} }

// Work sends the reaction; on the last attempt a failure is dropped.
func (w *ReactWorker) Work(ctx context.Context, job *river.Job[model.ReactArgs]) error {
	return w.svc.DeliverReaction(ctx, job.Args, job.Attempt >= job.MaxAttempts)
}

// FetchMediaWorker works FetchMediaArgs.
type FetchMediaWorker struct {
	river.WorkerDefaults[model.FetchMediaArgs]
	svc *service.Service
}

// NewFetchMediaWorker builds the worker.
func NewFetchMediaWorker(svc *service.Service) *FetchMediaWorker { return &FetchMediaWorker{svc: svc} }

// Work downloads the file into storage; an error retries.
func (w *FetchMediaWorker) Work(ctx context.Context, job *river.Job[model.FetchMediaArgs]) error {
	return w.svc.FetchMedia(ctx, job.Args.WorkspaceID, job.Args.MessageID)
}

// Timeout covers streaming a 100 MB file.
func (w *FetchMediaWorker) Timeout(*river.Job[model.FetchMediaArgs]) time.Duration {
	return 5 * time.Minute
}

// MarkReadWorker works MarkReadArgs.
type MarkReadWorker struct {
	river.WorkerDefaults[model.MarkReadArgs]
	svc *service.Service
}

// NewMarkReadWorker builds the worker.
func NewMarkReadWorker(svc *service.Service) *MarkReadWorker { return &MarkReadWorker{svc: svc} }

// Work sends the read receipt; a failure is logged and dropped.
func (w *MarkReadWorker) Work(ctx context.Context, job *river.Job[model.MarkReadArgs]) error {
	w.svc.SendReadReceipt(ctx, job.Args)
	return nil
}

// UnsnoozePeriodicJob schedules UnsnoozeArgs every minute.
func UnsnoozePeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return model.UnsnoozeArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

// UnsnoozeWorker works UnsnoozeArgs.
type UnsnoozeWorker struct {
	river.WorkerDefaults[model.UnsnoozeArgs]
	svc *service.Service
}

// NewUnsnoozeWorker builds the worker.
func NewUnsnoozeWorker(svc *service.Service) *UnsnoozeWorker { return &UnsnoozeWorker{svc: svc} }

// Work reopens every chat whose snooze has ended.
func (w *UnsnoozeWorker) Work(ctx context.Context, _ *river.Job[model.UnsnoozeArgs]) error {
	_, err := w.svc.Unsnooze(ctx)
	return err
}
