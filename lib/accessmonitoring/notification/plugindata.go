package notification

import (
	"encoding/json"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/trace"
)

type SentMessage interface {
	ID() string
}

type SentReview interface {
	ID() string
}

type Message struct {
	SentMessage SentMessage
	Reviews     map[ReviewID]SentReview
}

type Notification struct {
	// ID specifies the notification ID.
	ID string
	// AccessRequestData contains Access Request data associated with this notification.
	AccessRequestData plugindata.AccessRequestData

	// RequesterRecipient specifies the requester recipient.
	RequesterRecipient common.Recipient
	// ReviewerRecipients contains the list of recipients to notify.
	ReviewerRecipients []common.Recipient

	// RequesterMessage specifies the requester message and reviews.
	RequesterMessage Message
	// SentMessages contains a map of messages and reviews.
	ReviewerMessages map[MessageID]Message
}

type MessageID any
type ReviewID any

// DecodeNotification deserializes a string map to Notification struct.
func DecodeNotification(data map[string]string) (notification Notification, err error) {
	notification.AccessRequestData, err = plugindata.DecodeAccessRequestData(data)
	if err != nil {
		return notification, trace.Wrap(err)
	}

	notification.ID = data["id"]

	if err := json.Unmarshal([]byte(data["requester_recipient"]), &notification.RequesterRecipient); err != nil {
		return Notification{}, trace.Wrap(err)
	}
	if err := json.Unmarshal([]byte(data["reviewer_recipients"]), &notification.ReviewerRecipients); err != nil {
		return Notification{}, trace.Wrap(err)
	}
	if err := json.Unmarshal([]byte(data["requester_message"]), &notification.RequesterMessage); err != nil {
		return Notification{}, trace.Wrap(err)
	}
	if err := json.Unmarshal([]byte(data["reviewer_messages"]), &notification.ReviewerMessages); err != nil {
		return Notification{}, trace.Wrap(err)
	}

	return notification, trace.NotImplemented("not implemented")
}

func EncodeNotification(notification Notification) (map[string]string, error) {
	result, err := plugindata.EncodeAccessRequestData(notification.AccessRequestData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["id"] = notification.ID

	bytes, err := json.Marshal(notification.RequesterRecipient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["requester_recipient"] = string(bytes)

	bytes, err = json.Marshal(notification.ReviewerRecipients)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["reviewer_recipients"] = string(bytes)

	bytes, err = json.Marshal(notification.RequesterMessage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["requester_message"] = string(bytes)

	bytes, err = json.Marshal(notification.ReviewerMessages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["reviewer_messages"] = string(bytes)

	return result, nil
}
