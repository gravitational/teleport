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
// It's responsible for formatting and ServiceNow alerts when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type Bot struct {
	client      *Client
	clusterName string
	webProxyURL *url.URL
}

// CheckHealth checks if the bot can connect to its messaging service
func (b *Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.client.CheckHealth(ctx))
}

// Broadcast creates an alert for the provided rotas
func (b *Bot) Broadcast(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (data common.SentMessages, err error) {
	rotaIDs := make([]string, 0, len(recipients))
	for _, rota := range recipients {
		rotaIDs = append(rotaIDs, rota.ID)
	}
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

// PostReviewReply posts an alert note.
func (b *Bot) PostReviewReply(ctx context.Context, _ string, incidentID string, review types.AccessReview) error {
	return trace.Wrap(b.client.PostReviewNote(ctx, incidentID, review))
}

// UpdateMessages add notes to the incident containing updates to status.
// This will also resolve incidents based on the resolution tag.
func (b *Bot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, incidentData common.SentMessages, reviews []types.AccessReview) error {
	var errs []error

	var closeCode string
	var state string
	if data.ResolutionTag == pd.ResolvedApproved {
		values, ok := data.SystemAnnotations[types.TeleportNamespace+types.ReqAnnotationApprovedCloseCode]
		if !ok || len(values) < 1 {
			return trace.BadParameter("close code annotation missing form serviceNow configuration")
		}
		closeCode = values[0]
		state = ResolutionStateResolved
	}
	if data.ResolutionTag == pd.ResolvedDenied {
		values, ok := data.SystemAnnotations[types.TeleportNamespace+types.ReqAnnotationDeniedCloseCode]
		if !ok || len(values) < 1 {
			return trace.BadParameter("close code annotation missing form serviceNow configuration")
		}
		closeCode = values[0]
		state = ResolutionStateClosed
	}

	resolution := Resolution{
		CloseCode: closeCode,
		State:     state,
		Reason:    data.ResolutionReason,
	}
	for _, incident := range incidentData {
		if err := b.client.ResolveIncident(ctx, incident.MessageID, resolution); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// FetchRecipient returns the recipient for the given raw recipient.
func (b *Bot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
		Kind: common.RecipientKindRota,
	}, nil
}
