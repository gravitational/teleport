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

package opsgenie

import (
	"context"
	"net/url"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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

// CheckHealth checks if the bot can connect to its messaging service
func (b *Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.client.CheckHealth(ctx))
}

// Broadcast creates an alert for the provided recipients (schedules)
func (b *Bot) Broadcast(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (data common.SentMessages, err error) {
	schedules := []string{}
	for _, recipient := range recipients {
		schedules = append(schedules, recipient.Name)
	}
	if len(recipients) == 0 {
		schedules = append(schedules, b.client.DefaultSchedules...)
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
		ResolveAnnotations: types.Labels{
			types.TeleportNamespace + types.ReqAnnotationSchedulesLabel: schedules,
		},
	}
	opsgenieData, err := b.client.CreateAlert(ctx, reqID, opsgenieReqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data = common.SentMessages{{
		ChannelID: opsgenieData.ServiceID,
		MessageID: opsgenieData.AlertID,
	}}

	return data, nil

}

// PostReviewReply posts an alert note.
func (b *Bot) PostReviewReply(ctx context.Context, _ string, alertID string, review types.AccessReview) error {
	return trace.Wrap(b.client.PostReviewNote(ctx, alertID, review))
}

// UpdateMessages add notes to the alert containing updates to status.
// This will also resolve alerts based on the resolution tag.
func (b *Bot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, alertData common.SentMessages, reviews []types.AccessReview) error {
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
func (b *Bot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
		Kind: common.RecipientKindSchedule,
	}, nil
}
