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

package servicenow

import (
	"context"
	"net/url"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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

// CheckHealth checks if the bot can connect to its messaging service
func (b *Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.client.CheckHealth(ctx))
}

// Broadcast creates a ServiceNow incident.
func (b *Bot) Broadcast(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (data common.SentMessages, err error) {
	serviceNowReqData := RequestData{
		User:          reqData.User,
		Roles:         reqData.Roles,
		Created:       time.Now().UTC(),
		RequestReason: reqData.RequestReason,
		ReviewsCount:  reqData.ReviewsCount,
	}
	serviceNowData, err := b.client.CreateIncident(ctx, reqID, serviceNowReqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data = common.SentMessages{{
		MessageID: serviceNowData.IncidentID,
	}}

	return data, nil
}

// PostReviewReply posts an incident work note.
func (b *Bot) PostReviewReply(ctx context.Context, _ string, incidentID string, review types.AccessReview) error {
	return trace.Wrap(b.client.PostReviewNote(ctx, incidentID, review))
}

// UpdateMessages add notes to the incident containing updates to status.
// This will also resolve incidents based on the resolution tag.
func (b *Bot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, incidentData common.SentMessages, reviews []types.AccessReview) error {
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
