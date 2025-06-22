package accessrequest

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/services/common"
)

type Notifier interface {
	// CheckHealth checks if the bot can connect to its messaging service
	CheckHealth(ctx context.Context) error
	// FetchRecipient fetches recipient data from the messaging service API. It can also be used to check and initialize
	// a communication channel (e.g. MsTeams needs to install the app for the user before being able to send
	// notifications)
	FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error)

	// BroadcastAccessRequestMessage sends an access request message to a list of Recipient
	BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData plugindata.AccessRequestData) (data SentMessages, err error)
	// PostReviewReply posts in thread an access request review. This does nothing if the messaging service
	// does not support threaded replies.
	PostReviewReply(ctx context.Context, channelID string, threadID string, review types.AccessReview) error
	// UpdateMessages updates access request messages that were previously sent via Broadcast
	// This is used to change the access-request status and number of required approval remaining
	UpdateMessages(ctx context.Context, reqID string, data plugindata.AccessRequestData, messageData SentMessages, reviews []types.AccessReview) error
	// NotifyUser notifies the user if their access request status has changed
	NotifyUser(ctx context.Context, reqID string, ard plugindata.AccessRequestData) error

	// TODO: move this out
	// FetchOncallUsers fetches on-call users filtered by the provided annotations
	FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error)
}

// TODO, add an alternative interface that checks the on-call and also support choosing recipients from
// the AR annotations.
// We have 2 uses cases:
// - the messaging plugins: no auto-approval and recipients come from static config, AMR, and suggested reviewers
// - the operational plugins: auto-apprtoval and recipients come from AMR or AR annotations
// It will be better to have to constructors and two notifier interface to better support those two.
