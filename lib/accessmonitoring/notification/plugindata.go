package notification

import (
	"encoding/json"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/trace"
)

type SentMessage interface {
	ID() MessageID
}

type SentReview interface {
	ID() ReviewID
}

type Message[M SentMessage, R SentReview] struct {
	SentMessage M
	Reviews     map[string]R
}

type Notification[M SentMessage, R SentReview] struct {
	// ID specifies the notification ID.
	ID string
	// AccessRequestData contains Access Request data associated with this notification.
	AccessRequestData plugindata.AccessRequestData

	// RequesterRecipient specifies the requester recipient.
	RequesterRecipient *common.Recipient
	// ReviewerRecipients contains the list of recipients to notify.
	ReviewerRecipients []common.Recipient

	// RequesterMessage specifies the requester message and reviews.
	RequesterMessage *Message[M, R]
	// SentMessages contains a map of messages and reviews.
	ReviewerMessages map[MessageID]Message[M, R]
}

type MessageID string
type ReviewID string

// DecodeNotification deserializes a string map to Notification struct.
func DecodeNotification[M SentMessage, R SentReview](data map[string]string) (notification Notification[M, R], err error) {
	notification.AccessRequestData, err = plugindata.DecodeAccessRequestData(data)
	if err != nil {
		return notification, trace.Wrap(err)
	}

	notification.ID = data["id"]

	if data["requester_recipient"] != "" {
		if err := json.Unmarshal([]byte(data["requester_recipient"]), &notification.RequesterRecipient); err != nil {
			return Notification[M, R]{}, trace.Wrap(err)
		}
	}
	if data["reviewer_recipients"] != "" {
		if err := json.Unmarshal([]byte(data["reviewer_recipients"]), &notification.ReviewerRecipients); err != nil {
			return Notification[M, R]{}, trace.Wrap(err)
		}
	}
	if data["requester_message"] != "" {
		if err := json.Unmarshal([]byte(data["requester_message"]), &notification.RequesterMessage); err != nil {
			return Notification[M, R]{}, trace.Wrap(err)
		}
	}
	if data["reviewer_messages"] != "" {
		if err := json.Unmarshal([]byte(data["reviewer_messages"]), &notification.ReviewerMessages); err != nil {
			return Notification[M, R]{}, trace.Wrap(err)
		}
	}

	return notification, nil
}

func EncodeNotification[M SentMessage, R SentReview](notification Notification[M, R]) (map[string]string, error) {
	result, err := plugindata.EncodeAccessRequestData(notification.AccessRequestData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["id"] = notification.ID

	if notification.RequesterRecipient != nil {
		bytes, err := json.Marshal(notification.RequesterRecipient)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["requester_recipient"] = string(bytes)
	}

	if len(notification.ReviewerRecipients) > 0 {
		bytes, err := json.Marshal(notification.ReviewerRecipients)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["reviewer_recipients"] = string(bytes)
	}

	if notification.RequesterMessage != nil {
		bytes, err := json.Marshal(notification.RequesterMessage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["requester_message"] = string(bytes)
	}

	if len(notification.ReviewerMessages) > 0 {
		bytes, err := json.Marshal(notification.ReviewerMessages)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["reviewer_messages"] = string(bytes)
	}

	return result, nil
}
