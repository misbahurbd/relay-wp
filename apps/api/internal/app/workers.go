package app

import (
	"github.com/riverqueue/river"

	authjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/jobs"
	automationsjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/jobs"
	billingjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/jobs"
	contactsjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/jobs"
	inboxjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/jobs"
	integrationsjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/jobs"
	onboardingjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/jobs"
	teamjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/team/jobs"
	whatsappjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/jobs"
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
	river.AddWorker(w, onboardingjobs.NewSetupReminderWorker(a.onboarding))
	river.AddWorker(w, onboardingjobs.NewTrialReminderWorker(a.onboarding))
	river.AddWorker(w, whatsappjobs.NewSetupWorker(a.whatsapp))
	river.AddWorker(w, whatsappjobs.NewWebhookWorker(a.whatsapp))
	river.AddWorker(w, authjobs.NewCleanupWorker(a.db))
	river.AddWorker(w, teamjobs.NewMaintenanceWorker(a.team))
	river.AddWorker(w, billingjobs.NewProcessPaymentEventWorker(a.billing))
	river.AddWorker(w, billingjobs.NewInstallmentsDueWorker(a.billing))
	river.AddWorker(w, billingjobs.NewExpireTrialsWorker(a.billing))
	river.AddWorker(w, billingjobs.NewExpireCheckoutsWorker(a.billing))
	river.AddWorker(w, billingjobs.NewPurgeEventPayloadsWorker(a.billing))
	river.AddWorker(w, billingjobs.NewPlanCacheDriftWorker(a.billing))
	river.AddWorker(w, billingjobs.NewPastDueGraceWorker(a.billing))
	river.AddWorker(w, billingjobs.NewStuckPaymentEventsWorker(a.billing))
	river.AddWorker(w, contactsjobs.NewImportWorker(a.contacts))
	river.AddWorker(w, integrationsjobs.NewDeliverWorker(a.integrations))
	river.AddWorker(w, integrationsjobs.NewCleanupWorker(a.integrations))
	river.AddWorker(w, automationsjobs.NewInboundWorker(a.automations))
	river.AddWorker(w, automationsjobs.NewProcessRunWorker(a.automations))
	river.AddWorker(w, automationsjobs.NewTickWorker(a.automations))
	river.AddWorker(w, inboxjobs.NewSendWorker(a.inbox))
	river.AddWorker(w, inboxjobs.NewReactWorker(a.inbox))
	river.AddWorker(w, inboxjobs.NewFetchMediaWorker(a.inbox))
	river.AddWorker(w, inboxjobs.NewMarkReadWorker(a.inbox))
	river.AddWorker(w, inboxjobs.NewUnsnoozeWorker(a.inbox))
	// Periodic jobs run on the leader only, so several workers don't repeat them.
	a.queue.PeriodicJobs().Add(authjobs.CleanupPeriodicJob())
	a.queue.PeriodicJobs().Add(teamjobs.MaintenancePeriodicJob())
	a.queue.PeriodicJobs().AddMany(billingjobs.PeriodicJobs())
	a.queue.PeriodicJobs().Add(integrationsjobs.CleanupPeriodicJob())
	a.queue.PeriodicJobs().Add(inboxjobs.UnsnoozePeriodicJob())
	a.queue.PeriodicJobs().Add(automationsjobs.TickPeriodicJob())
}
