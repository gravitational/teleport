package accesslist

import (
	"context"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/lib/services/common"
)

type Notifier interface {
	// CheckHealth checks if the bot can connect to its messaging service
	CheckHealth(ctx context.Context) error
	// FetchRecipient fetches recipient data from the messaging service API. It can also be used to check and initialize
	// a communication channel (e.g. MsTeams needs to install the app for the user before being able to send
	// notifications)
	FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error)
	// SendReviewReminders will send a review reminder that an access list needs to be reviewed.
	SendReviewReminders(ctx context.Context, recipient common.Recipient, accessLists []*accesslist.AccessList) error
}
