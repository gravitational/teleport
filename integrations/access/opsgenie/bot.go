/*
Copyright 2022 Gravitational, Inc.

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
	return nil
}

// Broadcast sends an access request message to a list of Recipient
func (b *Bot) Broadcast(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (data common.SentMessages, err error) {
	return nil, nil
}

// PostReviewReply posts in thread an access request review. This does nothing if the messaging service
// does not support threaded replies.
func (b *Bot) PostReviewReply(ctx context.Context, channelID string, threadID string, review types.AccessReview) error {
	return nil
}

// UpdateMessages updates access request messages that were previously sent via Broadcast
// This is used to change the access-request status and number of required approval remaining
func (b *Bot) UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, messageData common.SentMessages, reviews []types.AccessReview) error {
	return nil
}

// FetchRecipient fetches recipient data from the messaging service API. It can also be used to check and initialize
// a communication channel (e.g. MsTeams needs to install the app for the user before being able to send
// notifications)
func (b *Bot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return nil, nil
}
