package accesslist

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib/launcher"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/services/common"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"time"
)

const (
	// oneDay is the number of hours in a day.
	oneDay = 24 * time.Hour
	// oneWeek is the number of days in a week.
	oneWeek = oneDay * 7
	// reminderInterval is the interval for sending access list reminders.
	reminderInterval = 3 * time.Hour
)

type accessListReminderService struct {
	// provided on creation
	notifier Notifier
	clock    clockwork.Clock

	// provided at runtime
	pluginName     string
	teleportClient teleport.Client

	// initialized but the service itself
	pluginData *pd.CompareAndSwap[pd.AccessListNotificationData]
}

func NewAccessListReminderService(notifier Notifier) launcher.Service {
	return &accessListReminderService{
		notifier: notifier,
		clock:    clockwork.NewRealClock(),
	}
}

func (a *accessListReminderService) CheckHealth(ctx context.Context) error {
	// TODO: use the notifier to ping the remote service instead
	return nil
}

func (a *accessListReminderService) Run(ctx context.Context, clt teleport.Client, name string) error {
	a.teleportClient = clt
	a.pluginName = name

	a.pluginData = pd.NewCAS(
		a.teleportClient,
		a.pluginName,
		types.KindAccessList,
		pd.EncodeAccessListNotificationData,
		pd.DecodeAccessListNotificationData,
	)

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
			return ctx.Err()
		}
	}

}

// remindIfNecessary will create and send reminders if necessary. The only error this returns is
// notImplemented, which will cease looking for reminders if the auth server does not support
// access lists.
func (a *accessListReminderService) remindIfNecessary(ctx context.Context) error {
	log := logger.Get(ctx)

	log.Info("Looking for Access List Review reminders")

	var nextToken string
	var err error
	remindersLookup := make(map[common.Recipient][]*accesslist.AccessList)
	for {
		var accessLists []*accesslist.AccessList
		accessLists, nextToken, err = a.teleportClient.ListAccessLists(ctx, 0 /* default page size */, nextToken)
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
			recipients, err := a.getRecipientsRequiringReminders(ctx, accessList)
			if err != nil {
				log.WithError(err).Warnf("Error getting recipients to notify for review due for access list %q", accessList.Spec.Title)
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
		if err := a.notifier.SendReviewReminders(ctx, recipient, accessLists); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		log.WithError(trace.NewAggregate(errs...)).Warn("Error notifying for access list reviews")
	}

	return nil
}

// getRecipientsRequiringReminders will return recipients that require reminders only
// if the access list review dates are getting close. At the moment, this
// only supports notifying owners.
func (a *accessListReminderService) getRecipientsRequiringReminders(ctx context.Context, accessList *accesslist.AccessList) ([]common.Recipient, error) {
	log := logger.Get(ctx)

	// Find the current notification window.
	now := a.clock.Now()
	notificationStart := accessList.Spec.Audit.NextAuditDate.Add(-accessList.Spec.Audit.Notifications.Start)

	// If the current time before the notification start time, skip notifications.
	if now.Before(notificationStart) {
		log.Debugf("Access list %s is not ready for notifications, notifications start at %s", accessList.GetName(), notificationStart.Format(time.RFC3339))
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
func (a *accessListReminderService) fetchRecipients(ctx context.Context, accessList *accesslist.AccessList, now, notificationStart time.Time) map[string]common.Recipient {
	log := logger.Get(ctx)

	allRecipients := make(map[string]common.Recipient, len(accessList.Spec.Owners))

	// Get the owners from the bot as recipients.
	for _, owner := range accessList.Spec.Owners {
		recipient, err := a.notifier.FetchRecipient(ctx, owner.Name)
		if err != nil {
			log.Debugf("error getting recipient %s", owner.Name)
			continue
		}
		allRecipients[owner.Name] = *recipient
	}

	return allRecipients
}

// updatePluginDataAndGetRecipientsRequiringReminders will return recipients requiring reminders
// and update the plugin data about when the recipient got notified.
func (a *accessListReminderService) updatePluginDataAndGetRecipientsRequiringReminders(ctx context.Context, accessList *accesslist.AccessList, allRecipients map[string]common.Recipient, now, notificationStart time.Time) ([]common.Recipient, error) {
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
		return nil, trace.Wrap(err)
	}

	return recipients, nil
}
