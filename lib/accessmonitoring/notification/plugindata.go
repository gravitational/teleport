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
	// Recipients contains the list of recipients to notify.
	Recipients []common.Recipient
	// AccessRequestData contains Access Request data associated with this notification.
	AccessRequestData plugindata.AccessRequestData
	// SentMessages contains a map of messages and reviews.
	SentMessages map[MessageID]Message
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

	messages := data["messages"]
	if err := json.Unmarshal([]byte(messages), &notification.SentMessages); err != nil {
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

	bytes, err := json.Marshal(notification.SentMessages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result["messages"] = string(bytes)

	return result, nil
}
