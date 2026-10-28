package app

import (
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

// workers returns the registry with the platform workers, which need no features.
func (a *App) workers() *river.Workers {
	w := river.NewWorkers()
	river.AddWorker(w, mailer.NewSendWorker(a.mailer))
	return w
}

// addFeatureWorkers registers each feature's workers once the features are built.
func (a *App) addFeatureWorkers(w *river.Workers) {
	river.AddWorker(w, onboarding.NewSetupReminderWorker(a.onboarding))
	river.AddWorker(w, onboarding.NewTrialReminderWorker(a.onboarding))
	river.AddWorker(w, whatsapp.NewSetupWorker(a.whatsapp))
	river.AddWorker(w, whatsapp.NewWebhookWorker(a.whatsapp))
	river.AddWorker(w, auth.NewCleanupWorker(a.db))
	// Periodic jobs run on the leader only, so several workers don't repeat them.
	a.queue.PeriodicJobs().Add(auth.CleanupPeriodicJob())
}
