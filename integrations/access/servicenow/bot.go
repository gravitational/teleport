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

package servicenow

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

// Bot is a serviceNow client that works with AccessRequests.
// It's responsible for formatting and ServiceNow incidents when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type Bot struct {
	client      *Client
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
func (b Bot) SendReviewReminders(ctx context.Context, recipients []common.Recipient, accessList *accesslist.AccessList) error {
	return trace.NotImplemented("access list review reminder is not yet implemented")
}

// BroadcastAccessRequestMessage creates a ServiceNow incident.
func (b *Bot) BroadcastAccessRequestMessage(ctx context.Context, _ []common.Recipient, reqID string, reqData pd.AccessRequestData) (data accessrequest.SentMessages, err error) {
	serviceNowReqData := RequestData{
		User:               reqData.User,
		Roles:              reqData.Roles,
		Created:            time.Now().UTC(),
		RequestReason:      reqData.RequestReason,
		ReviewsCount:       reqData.ReviewsCount,
		Resources:          reqData.Resources,
		SuggestedReviewers: reqData.SuggestedReviewers,
	}
	serviceNowData, err := b.client.CreateIncident(ctx, reqID, serviceNowReqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data = accessrequest.SentMessages{{
		MessageID: serviceNowData.IncidentID,
	}}

	return data, nil
}

// PostReviewReply posts an incident work note.
func (b *Bot) PostReviewReply(ctx context.Context, _ string, incidentID string, review types.AccessReview) error {
	return trace.Wrap(b.client.PostReviewNote(ctx, incidentID, review))
}

// NotifyUser will send users a direct message with the access request status
func (b Bot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	return trace.NotImplemented("notify user not implemented for plugin")
}

// UpdateMessages add notes to the incident containing updates to status.
// This will also resolve incidents based on the resolution tag.
func (b *Bot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, incidentData accessrequest.SentMessages, reviews []types.AccessReview) error {
	var errs []error

	var state string

	switch data.ResolutionTag {
	case pd.ResolvedApproved:
		state = ResolutionStateResolved
	case pd.ResolvedDenied:
		state = ResolutionStateClosed
	}

	resolution := Resolution{
		State:  state,
		Reason: data.ResolutionReason,
	}
	for _, incident := range incidentData {
		if err := b.client.ResolveIncident(ctx, incident.MessageID, resolution); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// FetchRecipient isn't used by the ServicenoPlugin
func (b *Bot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return nil, trace.NotImplemented("ServiceNow plugin does not use recipients")
}
