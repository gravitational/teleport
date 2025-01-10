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

package opsgenie

import (
	"context"
	"net/url"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

// Bot is a opsgenie client that works with AccessRequest.
// It's responsible for formatting and Opsgenie alerts when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type Bot struct {
	client      *Client
	clusterName string
	webProxyURL *url.URL
}

// SupportedApps are the apps supported by this bot.
func (b *Bot) SupportedApps() []common.App {
	return []common.App{
		accessrequest.NewApp(b),
	}
}

// CheckHealth checks if the bot can connect to its messaging service
func (b *Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.client.CheckHealth(ctx))
}

// SendReviewReminders will send a review reminder that an access list needs to be reviewed.
func (b Bot) SendReviewReminders(ctx context.Context, recipients []common.Recipient, accessLists []*accesslist.AccessList) error {
	return trace.NotImplemented("access list review reminder is not yet implemented")
}

// BroadcastAccessRequestMessage creates an alert for the provided recipients (schedules)
func (b *Bot) BroadcastAccessRequestMessage(ctx context.Context, recipientSchedules []common.Recipient, reqID string, reqData pd.AccessRequestData) (data accessrequest.SentMessages, err error) {
	notificationSchedules := make([]string, 0, len(recipientSchedules))
	for _, notifySchedule := range recipientSchedules {
		notificationSchedules = append(notificationSchedules, notifySchedule.Name)
	}
	autoApprovalSchedules := []string{}
	if annotationAutoApprovalSchedules, ok := reqData.SystemAnnotations[types.TeleportNamespace+types.ReqAnnotationApproveSchedulesLabel]; ok {
		autoApprovalSchedules = annotationAutoApprovalSchedules
	}
	if len(autoApprovalSchedules) == 0 {
		autoApprovalSchedules = append(autoApprovalSchedules, b.client.DefaultSchedules...)
	}
	opsgenieReqData := RequestData{
		User:          reqData.User,
		Roles:         reqData.Roles,
		Created:       time.Now(),
		RequestReason: reqData.RequestReason,
		ReviewsCount:  reqData.ReviewsCount,
		Resolution: Resolution{
			Tag:    ResolutionTag(reqData.ResolutionTag),
			Reason: reqData.ResolutionReason,
		},
		SystemAnnotations: types.Labels{
			types.TeleportNamespace + types.ReqAnnotationApproveSchedulesLabel: autoApprovalSchedules,
			types.TeleportNamespace + types.ReqAnnotationNotifySchedulesLabel:  notificationSchedules,
		},
	}
	opsgenieData, err := b.client.CreateAlert(ctx, reqID, opsgenieReqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data = accessrequest.SentMessages{{
		ChannelID: opsgenieData.ServiceID,
		MessageID: opsgenieData.AlertID,
	}}

	return data, nil

}

// PostReviewReply posts an alert note.
func (b *Bot) PostReviewReply(ctx context.Context, _ string, alertID string, review types.AccessReview) error {
	return trace.Wrap(b.client.PostReviewNote(ctx, alertID, review))
}

// NotifyUser will send users a direct message with the access request status
func (b Bot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	return trace.NotImplemented("notify user not implemented for plugin")
}

// UpdateMessages add notes to the alert containing updates to status.
// This will also resolve alerts based on the resolution tag.
func (b *Bot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, alertData accessrequest.SentMessages, reviews []types.AccessReview) error {
	var errs []error
	for _, alert := range alertData {
		resolution := Resolution{
			Tag:    ResolutionTag(data.ResolutionTag),
			Reason: data.ResolutionReason,
		}
		err := b.client.ResolveAlert(ctx, alert.MessageID, resolution)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// FetchRecipient returns the recipient for the given raw recipient.
func (b *Bot) FetchRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	return createScheduleRecipient(ctx, name)
}

func createScheduleRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	return &common.Recipient{
		Name: name,
		ID:   name,
		Kind: common.RecipientKindSchedule,
	}, nil
}

// FetchOncallUsers fetches on-call users filtered by the provided annotations.
func (b Bot) FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error) {
	return nil, trace.NotImplemented("fetch oncall users not implemented for plugin")
}
