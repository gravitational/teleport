/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

func init() {
	common.RegisterAppCreator("accesslist", NewApp)
}

const (
	// oneWeek is the number of hours in a week.
	oneWeek = 24 * time.Hour * 7
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
func NewApp(bot common.MessagingBot) (common.App, error) {
	if _, ok := bot.(MessagingBot); !ok {
		return nil, trace.BadParameter("bot does not support this app")
	}
	app := &App{}
	app.job = lib.NewServiceJob(app.run)
	return app, nil
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

	remindInterval := interval.New(interval.Config{
		Duration:      time.Hour * 3,
		FirstDuration: utils.FullJitter(time.Second * 30),
		Jitter:        retryutils.NewSeventhJitter(),
		Clock:         a.clock,
	})
	defer remindInterval.Stop()
	log := logger.Get(ctx)

	log.Info("Access list monitor is running")

	a.job.SetReady(true)
	for {
		select {
		case <-remindInterval.Next():
			log.Info("Looking for Access List Review reminders")

			var nextToken string
			var err error
			for {
				var accessLists []*accesslist.AccessList
				accessLists, nextToken, err = a.apiClient.AccessListClient().ListAccessLists(ctx, 0 /* default page size */, nextToken)
				if err != nil {
					log.Errorf("error listing access lists: %v", err)
					continue
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
		case <-ctx.Done():
			log.Info("Access list monitor is finished")
			return nil
		}
	}
}

// notifyForAccessListReviews will notify if access list review dates are getting close. At the moment, this
// only supports notifying owners.
func (a *App) notifyForAccessListReviews(ctx context.Context, accessList *accesslist.AccessList) error {
	// Find the current notification window.
	now := a.clock.Now()
	notificationStart := accessList.Spec.Audit.NextAuditDate.Add(-accessList.Spec.Audit.Notifications.Start)

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

	// If the current time before the notification start time, skip notifications.
	if now.Before(notificationStart) {
		log.Debugf("Access list %s is not ready for notifications, notifications start at %s", accessList.GetName(), notificationStart.Format(time.RFC3339))
		return nil
	}

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

	// Calculate weeks from start.
	weeksFromStart := now.Sub(notificationStart) / oneWeek
	windowStart := notificationStart.Add(weeksFromStart * oneWeek)

	recipients := []common.Recipient{}
	_, err := a.pluginData.Update(ctx, accessList.GetName(), func(data pd.AccessListNotificationData) (pd.AccessListNotificationData, error) {
		userNotifications := map[string]time.Time{}
		for _, recipient := range allRecipients {
			lastNotification := data.UserNotifications[recipient.Name]

			// If the notification window is before the last notification date, then this user doesn't need a notification.
			if !windowStart.After(lastNotification) {
				log.Debugf("User %s has already been notified", recipient.Name)
				userNotifications[recipient.Name] = lastNotification
				continue
			}

			recipients = append(recipients, recipient)
			userNotifications[recipient.Name] = now
		}
		if len(recipients) == 0 {
			return pd.AccessListNotificationData{}, trace.NotFound("nobody to notify for access list %s", accessList.GetName())
		}

		if err := a.bot.SendReviewReminders(ctx, recipients, accessList); err != nil {
			return pd.AccessListNotificationData{}, trace.Wrap(err)
		}

		return pd.AccessListNotificationData{UserNotifications: userNotifications}, nil
	})
	return trace.Wrap(err)
}
