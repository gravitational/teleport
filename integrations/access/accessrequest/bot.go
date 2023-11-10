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

package accessrequest

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

type MessagingBot interface {
	common.MessagingBot

	// BroadcastAccessRequestMessage sends an access request message to a list of Recipient
	BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (data SentMessages, err error)
	// PostReviewReply posts in thread an access request review. This does nothing if the messaging service
	// does not support threaded replies.
	PostReviewReply(ctx context.Context, channelID string, threadID string, review types.AccessReview) error
	// UpdateMessages updates access request messages that were previously sent via Broadcast
	// This is used to change the access-request status and number of required approval remaining
	UpdateMessages(ctx context.Context, reqID string, data pd.AccessRequestData, messageData SentMessages, reviews []types.AccessReview) error
}
