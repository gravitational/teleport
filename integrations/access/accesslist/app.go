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

	alclient "github.com/gravitational/teleport/api/client/accesslist"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/recipient"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const (
	// oneWeek is the number of hours in a week.
	oneWeek = 24 * time.Hour * 7
)

type App[M MessagingBot] struct {
	pluginName  string
	apiClient   teleport.Client
	accessLists services.AccessListsGetter
	pluginData  *pd.CompareAndSwap[pd.AccessListNotificationData]
	bot         M
	job         lib.ServiceJob
	clock       clockwork.Clock
}

func NewApp[M MessagingBot]() *App[M] {
	app := &App[M]{}
	app.job = lib.NewServiceJob(app.run)
	return app
}

func (a *App[M]) Init(baseApp *common.BaseApp[M]) error {
	a.pluginName = baseApp.PluginName
	a.apiClient = baseApp.APIClient
	a.accessLists = accessListClient(a.apiClient)
	if a.accessLists == nil {
		return trace.BadParameter("api client does not contain an access list client")
	}
	a.pluginData = pd.NewCAS(
		a.apiClient,
		a.pluginName,
		types.KindAccessList,
		pd.EncodeAccessListNotificationData,
		pd.DecodeAccessListNotificationData,
	)
	a.bot = baseApp.Bot

	if a.clock == nil {
		a.clock = clockwork.NewRealClock()
	}

	return nil
}

func (a *App[_]) Start(ctx context.Context, process *lib.Process) (err error) {
	process.SpawnCriticalJob(a.job)

	return nil
}

func (a *App[_]) WaitReady(ctx context.Context) (bool, error) {
	return a.job.WaitReady(ctx)
}

func (a *App[_]) WaitForDone() {
	<-a.job.Done()
}

func (a *App[_]) Err() error {
	return nil
}

// run will monitor access lists and post review reminders.
func (a *App[_]) run(ctx context.Context) error {
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
				accessLists, nextToken, err = a.accessLists.ListAccessLists(ctx, 0 /* default page size */, nextToken)
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
func (a *App[_]) notifyForAccessListReviews(ctx context.Context, accessList *accesslist.AccessList) error {
	log := logger.Get(ctx)
	allRecipients := make(map[string]recipient.Recipient, len(accessList.Spec.Owners))

	now := a.clock.Now()
	// Find the current notification window.
	notificationStart := accessList.Spec.Audit.NextAuditDate.Add(-accessList.Spec.Audit.Notifications.Start)

	// If the current time before the notification start time, skip notifications.
	if now.Before(notificationStart) {
		log.Infof("Access list %s is not ready for notifications, notifications start at %s", accessList.GetName(), notificationStart.Format(time.RFC3339))
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

	// Calculate weeks from start.
	weeksFromStart := now.Sub(notificationStart) / oneWeek
	windowStart := notificationStart.Add(weeksFromStart * oneWeek)

	recipients := []recipient.Recipient{}
	_, err = a.pluginData.Update(ctx, accessList.GetName(), func(data pd.AccessListNotificationData) (pd.AccessListNotificationData, error) {
		userNotifications := map[string]time.Time{}
		for _, recipient := range allRecipients {
			lastNotification := data.UserNotifications[recipient.Name]

			// If the notification window is before the last notification date, then this user doesn't need a notification.
			if !windowStart.After(lastNotification) {
				log.Infof("User %s has already been notified", recipient.Name)
				userNotifications[recipient.Name] = lastNotification
				continue
			}

			recipients = append(recipients, recipient)
			userNotifications[recipient.Name] = now
		}
		return pd.AccessListNotificationData{UserNotifications: userNotifications}, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(recipients) == 0 {
		log.Infof("Nobody to notify for access list %s", accessList.GetName())
		return nil
	}

	return trace.Wrap(a.bot.SendReviewReminders(ctx, recipients, accessList))
}

// accessListClient will return an access list client for the plugin manager.
func accessListClient(client teleport.Client) services.AccessListsGetter {
	type accessListClient[T services.AccessListsGetter] interface {
		AccessListClient() T
	}

	switch client := client.(type) {
	case accessListClient[services.AccessLists]:
		return client.AccessListClient()
	case accessListClient[*alclient.Client]:
		return client.AccessListClient()
	}

	return nil
}
