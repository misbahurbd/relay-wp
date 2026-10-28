package app

import (
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

// workers registers every River worker. Features add theirs here.
func (a *App) workers() *river.Workers {
	w := river.NewWorkers()
	river.AddWorker(w, mailer.NewSendWorker(a.mailer))
	return w
}
