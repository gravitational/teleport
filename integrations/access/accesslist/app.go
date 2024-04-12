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

package accesslist

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const (
	// oneDay is the number of hours in a day.
	oneDay = 24 * time.Hour
	// oneWeek is the number of days in a week.
	oneWeek = oneDay * 7
	// reminderInterval is the interval for sending access list reminders.
	reminderInterval = 3 * time.Hour
)

// App is the access list application for plugins. This will notify access list owners
// when they need to review an access list.
type App struct {
	pluginName string
	apiClient  teleport.Client
	pluginData *pd.CompareAndSwap[pd.AccessListNotificationData]
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
	a.pluginData = pd.NewCAS(
		a.apiClient,
		a.pluginName,
		types.KindAccessList,
		pd.EncodeAccessListNotificationData,
		pd.DecodeAccessListNotificationData,
	)

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

// run will monitor access lists and post review reminders.
func (a *App) run(ctx context.Context) error {
	process := lib.MustGetProcess(ctx)
	ctx, cancel := context.WithCancel(ctx)

	// On process termination, explicitly cancel the context
	process.OnTerminate(func(ctx context.Context) error {
		cancel()
		return nil
	})

	log := logger.Get(ctx)

	log.Info("Access list monitor is running")

	a.job.SetReady(true)

	jitter := retryutils.NewSeventhJitter()
	timer := a.clock.NewTimer(jitter(30 * time.Second))
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
			if err := a.remindIfNecessary(ctx); err != nil {
				return trace.Wrap(err)
			}
			timer.Reset(jitter(reminderInterval))
		case <-ctx.Done():
			log.Info("Access list monitor is finished")
			return nil
		}
	}
}

// remindIfNecessary will create and send reminders if necessary. The only error this returns is
// notImplemented, which will cease looking for reminders if the auth server does not support
// access lists.
func (a *App) remindIfNecessary(ctx context.Context) error {
	log := logger.Get(ctx)

	log.Info("Looking for Access List Review reminders")

	var nextToken string
	var err error
	for {
		var accessLists []*accesslist.AccessList
		accessLists, nextToken, err = a.apiClient.ListAccessLists(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			if trace.IsNotImplemented(err) {
				log.Errorf("access list endpoint is not implemented on this auth server, so the access list app is ceasing to run.")
				return trace.Wrap(err)
			} else if trace.IsAccessDenied(err) {
				log.Warnf("Slack bot does not have permissions to list access lists. Please add access_list read and list permissions " +
					"to the role associated with the Slack bot.")
			} else {
				log.Errorf("error listing access lists: %v", err)
			}
			break
		}

		for _, accessList := range accessLists {
			if err := a.notifyForAccessListReviews(ctx, accessList); err != nil {
				log.WithError(err).Warn("Error notifying for access list reviews")
			}
		}

		if nextToken == "" {
			break
		}
	}

	return nil
}

// notifyForAccessListReviews will notify if access list review dates are getting close. At the moment, this
// only supports notifying owners.
func (a *App) notifyForAccessListReviews(ctx context.Context, accessList *accesslist.AccessList) error {
	log := logger.Get(ctx)

	// Find the current notification window.
	now := a.clock.Now()
	notificationStart := accessList.Spec.Audit.NextAuditDate.Add(-accessList.Spec.Audit.Notifications.Start)

	// If the current time before the notification start time, skip notifications.
	if now.Before(notificationStart) {
		log.Debugf("Access list %s is not ready for notifications, notifications start at %s", accessList.GetName(), notificationStart.Format(time.RFC3339))
		return nil
	}

	allRecipients := a.fetchRecipients(ctx, accessList, now, notificationStart)
	if len(allRecipients) == 0 {
		return trace.NotFound("no recipients could be fetched for access list %s", accessList.GetName())
	}

	// Try to create base notification data with a zero notification date. If these objects already
	// exist, that's okay.
	userNotifications := map[string]time.Time{}
	for _, recipient := range allRecipients {
		userNotifications[recipient.Name] = time.Time{}
	}
	_, err := a.pluginData.Create(ctx, accessList.GetName(), pd.AccessListNotificationData{
		UserNotifications: userNotifications,
	})

	// Error is okay so long as it's already exists.
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err, "during create")
	}

	return trace.Wrap(a.sendMessages(ctx, accessList, allRecipients, now, notificationStart))
}

// fetchRecipients will return all recipients.
func (a *App) fetchRecipients(ctx context.Context, accessList *accesslist.AccessList, now, notificationStart time.Time) map[string]common.Recipient {
	log := logger.Get(ctx)

	allRecipients := make(map[string]common.Recipient, len(accessList.Spec.Owners))

	// Get the owners from the bot as recipients.
	for _, owner := range accessList.Spec.Owners {
		recipient, err := a.bot.FetchRecipient(ctx, owner.Name)
		if err != nil {
			log.Debugf("error getting recipient %s", owner.Name)
			continue
		}
		allRecipients[owner.Name] = *recipient
	}

	return allRecipients
}

// sendMessages will send review notifications to owners and update the plugin data.
func (a *App) sendMessages(ctx context.Context, accessList *accesslist.AccessList, allRecipients map[string]common.Recipient, now, notificationStart time.Time) error {
	log := logger.Get(ctx)

	var windowStart time.Time
	if !now.After(accessList.Spec.Audit.NextAuditDate) {
		// Calculate weeks from start.
		weeksFromStart := now.Sub(notificationStart) / oneWeek
		windowStart = notificationStart.Add(weeksFromStart * oneWeek)
	} else {
		// Calculate days from start.
		daysFromStart := now.Sub(notificationStart) / oneDay
		windowStart = notificationStart.Add(daysFromStart * oneDay)
		log.Infof("windowStart: %s, now: %s", windowStart.String(), now.String())
	}

	recipients := []common.Recipient{}
	_, err := a.pluginData.Update(ctx, accessList.GetName(), func(data pd.AccessListNotificationData) (pd.AccessListNotificationData, error) {
		userNotifications := map[string]time.Time{}
		for _, recipient := range allRecipients {
			lastNotification := data.UserNotifications[recipient.Name]

			// If the notification window is before the last notification date, then this user doesn't need a notification.
			if !windowStart.After(lastNotification) {
				log.Debugf("User %s has already been notified for access list %s", recipient.Name, accessList.GetName())
				userNotifications[recipient.Name] = lastNotification
				continue
			}

			recipients = append(recipients, recipient)
			userNotifications[recipient.Name] = now
		}
		if len(recipients) == 0 {
			return pd.AccessListNotificationData{}, trace.NotFound("nobody to notify for access list %s", accessList.GetName())
		}

		return pd.AccessListNotificationData{UserNotifications: userNotifications}, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var errs []error
	for _, recipient := range recipients {
		if err := a.bot.SendReviewReminders(ctx, recipient, accessList); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}
