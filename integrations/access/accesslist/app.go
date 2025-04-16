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
	logutils "github.com/gravitational/teleport/lib/utils/log"
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

	log.InfoContext(ctx, "Access list monitor is running")

	a.job.SetReady(true)

	jitter := retryutils.SeventhJitter
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
			log.InfoContext(ctx, "Access list monitor is finished")
			return nil
		}
	}
}

// remindIfNecessary will create and send reminders if necessary. The only error this returns is
// notImplemented, which will cease looking for reminders if the auth server does not support
// access lists.
func (a *App) remindIfNecessary(ctx context.Context) error {
	log := logger.Get(ctx)

	log.InfoContext(ctx, "Looking for Access List Review reminders")

	var nextToken string
	var err error
	remindersLookup := make(map[common.Recipient][]*accesslist.AccessList)
	for {
		var accessLists []*accesslist.AccessList
		accessLists, nextToken, err = a.apiClient.ListAccessLists(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			if trace.IsNotImplemented(err) {
				log.ErrorContext(ctx, "access list endpoint is not implemented on this auth server, so the access list app is ceasing to run")
				return trace.Wrap(err)
			} else if trace.IsAccessDenied(err) {
				const msg = "Slack bot does not have permissions to list access lists. Please add access_list read and list permissions " +
					"to the role associated with the Slack bot."
				log.WarnContext(ctx, msg)
			} else {
				log.ErrorContext(ctx, "error listing access lists", "error", err)
			}
			break
		}

		for _, accessList := range accessLists {
			recipients, err := a.getRecipientsRequiringReminders(ctx, accessList)
			if err != nil {
				log.WarnContext(ctx, "Error getting recipients to notify for review due for access list",
					"error", err,
					"access_list", accessList.Spec.Title,
				)
				continue
			}

			// Store all recipients and the accesslist needing review
			// for later processing.
			for _, recipient := range recipients {
				remindersLookup[recipient] = append(remindersLookup[recipient], accessList)
			}
		}

		if nextToken == "" {
			break
		}
	}

	// Send reminders for each collected recipients.
	var errs []error
	for recipient, accessLists := range remindersLookup {
		if err := a.bot.SendReviewReminders(ctx, recipient, accessLists); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		log.WarnContext(ctx, "Error notifying for access list reviews", "error", trace.NewAggregate(errs...))
	}

	return nil
}

// getRecipientsRequiringReminders will return recipients that require reminders only
// if the access list review dates are getting close. At the moment, this
// only supports notifying owners.
func (a *App) getRecipientsRequiringReminders(ctx context.Context, accessList *accesslist.AccessList) ([]common.Recipient, error) {
	log := logger.Get(ctx)

	// Find the current notification window.
	now := a.clock.Now()
	notificationStart := accessList.Spec.Audit.NextAuditDate.Add(-accessList.Spec.Audit.Notifications.Start)

	// If the current time before the notification start time, skip notifications.
	if now.Before(notificationStart) {
		log.DebugContext(ctx, "Access list is not ready for notifications",
			"access_list", accessList.GetName(),
			"notification_start_time", notificationStart.Format(time.RFC3339),
		)
		return nil, nil
	}

	allRecipients := a.fetchRecipients(ctx, accessList, now, notificationStart)
	if len(allRecipients) == 0 {
		return nil, trace.NotFound("no recipients could be fetched for access list %s", accessList.GetName())
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
		return nil, trace.Wrap(err, "during create")
	}

	recipients, err := a.updatePluginDataAndGetRecipientsRequiringReminders(ctx, accessList, allRecipients, now, notificationStart)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return recipients, nil
}

// fetchRecipients will return all recipients.
func (a *App) fetchRecipients(ctx context.Context, accessList *accesslist.AccessList, now, notificationStart time.Time) map[string]common.Recipient {
	log := logger.Get(ctx)

	var allOwners []*accesslist.Owner

	allOwners, err := a.apiClient.GetAccessListOwners(ctx, accessList.GetName())
	if err != nil {
		// TODO(kiosion): Remove in v18; protecting against server not having `GetAccessListOwners` func.
		if trace.IsNotImplemented(err) {
			log.WarnContext(ctx, "Error getting nested owners for access list, continuing with only explicit owners",
				"error", err,
				"access_list", accessList.GetName(),
			)
			for _, owner := range accessList.Spec.Owners {
				allOwners = append(allOwners, &owner)
			}
		} else {
			log.ErrorContext(ctx, "Error getting owners for access list",
				"error", err,
				"access_list", accessList.GetName())
		}
	}

	allRecipients := make(map[string]common.Recipient, len(allOwners))

	// Get the owners from the bot as recipients.
	for _, owner := range allOwners {
		recipient, err := a.bot.FetchRecipient(ctx, owner.Name)
		if err != nil {
			log.DebugContext(ctx, "error getting recipient", "recipient", owner.Name)
			continue
		}
		allRecipients[owner.Name] = *recipient
	}

	return allRecipients
}

// updatePluginDataAndGetRecipientsRequiringReminders will return recipients requiring reminders
// and update the plugin data about when the recipient got notified.
func (a *App) updatePluginDataAndGetRecipientsRequiringReminders(ctx context.Context, accessList *accesslist.AccessList, allRecipients map[string]common.Recipient, now, notificationStart time.Time) ([]common.Recipient, error) {
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
		log.InfoContext(ctx, "calculating window start",
			"window_start", logutils.StringerAttr(windowStart),
			"now", logutils.StringerAttr(now),
		)
	}

	recipients := []common.Recipient{}
	_, err := a.pluginData.Update(ctx, accessList.GetName(), func(data pd.AccessListNotificationData) (pd.AccessListNotificationData, error) {
		userNotifications := map[string]time.Time{}
		for _, recipient := range allRecipients {
			lastNotification := data.UserNotifications[recipient.Name]

			// If the notification window is before the last notification date, then this user doesn't need a notification.
			if !windowStart.After(lastNotification) {
				log.DebugContext(ctx, "User has already been notified for access list",
					"user", recipient.Name,
					"access_list", accessList.GetName(),
				)
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
		return nil, trace.Wrap(err)
	}

	return recipients, nil
}
