/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package notifications

import (
	"context"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
)

// App is the access list application for plugins. This will notify access list owners
// when they need to review an access list.
type App struct {
	pluginName string
	apiClient  teleport.Client
	bot        MessagingBot
	job        lib.ServiceJob
	clock      clockwork.Clock
}

// NewApp will create a new access list application.
func NewApp(bot MessagingBot) common.App {
	app := &App{}
	app.job = lib.NewServiceJob(app.run)
	return app
}

// Init will initialize the application.
func (a *App) Init(baseApp *common.BaseApp) error {
	a.pluginName = baseApp.PluginName
	a.apiClient = baseApp.APIClient

	var ok bool
	a.bot, ok = baseApp.Bot.(MessagingBot)
	if !ok {
		return trace.BadParameter("bot does not implement access list bot methods")
	}

	a.clock = baseApp.Clock

	return nil
}

// Start will start the application.
func (a *App) Start(process *lib.Process) {
	process.SpawnCriticalJob(a.job)
}

// WaitReady will block until the job is ready.
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	return a.job.WaitReady(ctx)
}

// WaitForDone will wait until the job has completed.
func (a *App) WaitForDone() {
	<-a.job.Done()
}

// Err will return the error associated with the underlying job.
func (a *App) Err() error {
	return a.job.Err()
}

func (a *App) run(ctx context.Context) error {
	process := lib.MustGetProcess(ctx)

	job, err := watcherjob.NewJob(
		a.apiClient,
		watcherjob.Config{
			Watch:            types.Watch{Kinds: []types.WatchKind{{Kind: types.KindPluginNotification}}},
			EventFuncTimeout: handlerTimeout,
		},
		a.onWatcherEvent,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Info("Starting notification watcher")
	process.SpawnCriticalJob(job)

	ok, err := job.WaitReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Info("Notification watcher started and ready")
	a.job.SetReady(ok)
	if !ok {
		return trace.BadParameter("job not ready")
	}

	<-job.Done()
	return nil
}

// onWatcherEvent is called for every cluster Event. It will filter out non-access-request events and
// call onPendingRequest, onResolvedRequest and on DeletedRequest depending on the event.
func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
	if kind := event.Resource.GetKind(); kind != types.KindPluginNotification {
		return trace.Errorf("unexpected kind %s", kind)
	}
	op := event.Type
	notificationID := event.Resource.GetName()
	ctx, _ = logger.WithField(ctx, "notification_id", notificationID)

	switch op {
	case types.OpPut:
		ctx, _ = logger.WithField(ctx, "request_op", "put")
		adapter, ok := event.Resource.(*types.Resource153ToLegacyAdapter)
		if !ok {
			return trace.Errorf("unexpected legacy resource %T", event.Resource)
		}

		res := adapter.Unwrap()
		notification, ok := res.(*notificationsv1.PluginNotification)
		if !ok {
			return trace.Errorf("unexpected resource type %T", res)
		}

		if plugin := notification.GetSpec().GetPlugin(); plugin != a.pluginName {
			log.Debugf("ignoring notification for plugin %q", plugin)
			return nil
		}

		return trace.Wrap(a.notify(ctx, notification.GetSpec().GetRecipients(), notification.GetSpec().GetNotification()))
	case types.OpDelete:
		log.Debugln("ignoring notification delete")
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

func (a *App) fetchRecipients(ctx context.Context, rawRecipients []string) (common.RecipientSet, error) {
	recipients := common.RecipientSet{}
	for _, rawRecipient := range rawRecipients {
		recipient, err := a.bot.FetchRecipient(ctx, rawRecipient)
		if err != nil {
			return recipients, trace.Wrap(err)
		}
		recipients.Add(*recipient)
	}
	return recipients, nil
}

func (a *App) notify(ctx context.Context, rawRecipients []string, notification *notificationsv1.Notification) error {
	recipients, err := a.fetchRecipients(ctx, rawRecipients)
	if err != nil {
		return trace.Wrap(err, "fetching recipients")
	}
	var errors []error
	for _, recipient := range recipients.ToSlice() {
		err = a.bot.SendNotification(ctx, recipient, notification)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}
