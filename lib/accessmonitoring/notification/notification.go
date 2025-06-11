package notification

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

// Message represents a notification message.
type Message interface {
	ID() string
}

// Notification is a notification for an access request. It keeps track of the
// state of the state of the access request and sent messages.
type Notification struct {
	// ID specifies the notification ID.
	ID string
	// AccessRequestData contains Access Request data associated with this notification.
	AccessRequestData plugindata.AccessRequestData

	// Recipients contains a list of recipients to be notified.
	Recipients []string

	// Messages maps recipient IDs to messages
	Messages map[string]Message
}

// NotificationCAS wraps the CompareAndSwap and extends the encode/decode
// functionality to support arbitrary messages.
type NotificationCAS struct {
	notifications *plugindata.CompareAndSwap[Notification]
}

// NewCAS returns a NotificationCAS. The returned CAS only supports notification
// messages that are compatible with the provided encode/decode functions.
func NewCAS(
	handlerName string,
	client plugindata.Client,
	encodeMessage plugindata.EncodeFn[Message],
	decodeMessage plugindata.DecodeFn[Message],
) (*NotificationCAS, error) {
	notifications := plugindata.NewCAS(
		client,
		handlerName,
		types.KindAccessRequest,
		newEncoder(encodeMessage),
		newDecoder(decodeMessage),
	)

	return &NotificationCAS{
		notifications: notifications,
	}, nil
}

// Create tries to perform compare-and-swap update of a plugin data assuming that it does not exist
//
// fn callback function receives current plugin data value and returns modified value and
// error.
//
// Please note that fn might be called several times due to CAS backoff, hence, you must be careful
// with things like I/O ops and channels.
func (s *NotificationCAS) Create(
	ctx context.Context,
	resource string,
	newData Notification,
) (Notification, error) {
	return s.notifications.Create(ctx, resource, newData)
}

// Update tries to perform compare-and-swap update of a plugin data assuming that it exist
//
// modifyT will receive existing plugin data and should return a modified version of the data.
//
// If existing plugin data does not match expected data, then a trace.CompareFailed error should
// be returned to backoff and try again.
//
// To abort the update, modifyT should return an error other, than trace.CompareFailed, which
// will be propagated back to the caller of `Update`.
func (s *NotificationCAS) Update(
	ctx context.Context,
	resource string,
	modifyT func(Notification) (Notification, error),
) (Notification, error) {
	return s.notifications.Update(ctx, resource, modifyT)
}

func newEncoder(encodeMessage plugindata.EncodeFn[Message]) plugindata.EncodeFn[Notification] {
	return func(notification Notification) (map[string]string, error) {
		result, err := plugindata.EncodeAccessRequestData(notification.AccessRequestData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["id"] = notification.ID

		if len(notification.Recipients) > 0 {
			bytes, err := json.Marshal(notification.Recipients)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result["recipients"] = string(bytes)
		}

		if len(notification.Messages) > 0 {
			encodedMessages := make(map[string]map[string]string)
			for recipient, message := range notification.Messages {
				encodedMessage, err := encodeMessage(message)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				encodedMessages[recipient] = encodedMessage
			}

			bytes, err := json.Marshal(encodedMessages)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result["messages"] = string(bytes)
		}

		return result, nil
	}
}

func newDecoder(decodeMessage plugindata.DecodeFn[Message]) plugindata.DecodeFn[Notification] {
	return func(data map[string]string) (Notification, error) {
		var notification Notification
		var err error

		notification.AccessRequestData, err = plugindata.DecodeAccessRequestData(data)
		if err != nil {
			return notification, trace.Wrap(err)
		}
		notification.ID = data["id"]

		if recipients := data["recipients"]; recipients != "" {
			if err := json.Unmarshal([]byte(recipients), &notification.Recipients); err != nil {
				return Notification{}, trace.Wrap(err)
			}
		}

		if messages := data["messages"]; messages != "" {
			var messageData map[string]map[string]string
			if err := json.Unmarshal([]byte(messages), &messageData); err != nil {
				return Notification{}, trace.Wrap(err)
			}

			if len(messageData) != 0 {
				notification.Messages = make(map[string]Message)
				for recipient, message := range messageData {
					decoded, err := decodeMessage(message)
					if err != nil {
						return Notification{}, trace.Wrap(err)
					}
					notification.Messages[recipient] = decoded
				}
			}
		}

		return notification, nil
	}
}

// func (s *NotificationCAS) NewNotification(ctx context.Context, req types.AccessRequest) (Notification, error) {
// 	// recipients, err := handler.getRecipients(ctx, req)
// 	// if err != nil {
// 	// 	return Notification{}, trace.Wrap(err)
// 	// }
// 	resourceNames, err := s.getResourceNames(ctx, req)
// 	if err != nil {
// 		return Notification{}, trace.Wrap(err)
// 	}

// 	loginsByRole, err := s.getLoginsByRole(ctx, req)
// 	if trace.IsAccessDenied(err) {
// 		handler.Logger.WarnContext(ctx, "Missing permissions to get logins by role, please add role.read to the associated role", "error", err)
// 	} else if err != nil {
// 		return Notification{}, trace.Wrap(err)
// 	}

// 	return Notification{
// 		ID: req.GetName(),
// 		AccessRequestData: pd.AccessRequestData{
// 			User:              req.GetUser(),
// 			Roles:             req.GetRoles(),
// 			RequestReason:     req.GetRequestReason(),
// 			SystemAnnotations: req.GetSystemAnnotations(),
// 			Reviews:           req.GetReviews(),
// 			Resources:         resourceNames,
// 			LoginsByRole:      loginsByRole,
// 		},
// 		Recipients: recipients,
// 	}, nil

// }

// func (s *NotificationCAS) getLoginsByRole(ctx context.Context, req types.AccessRequest) (map[string][]string, error) {
// 	loginsByRole := make(map[string][]string, len(req.GetRoles()))

// 	user, err := handler.Client.GetUser(ctx, req.GetUser(), false)
// 	if err != nil {
// 		handler.Logger.WarnContext(ctx, "Missing permissions to apply user traits to login roles, please add user.read to the associated role", "error", err)
// 		for _, role := range req.GetRoles() {
// 			currentRole, err := handler.Client.GetRole(ctx, role)
// 			if err != nil {
// 				return nil, trace.Wrap(err)
// 			}
// 			loginsByRole[role] = currentRole.GetLogins(types.Allow)
// 		}
// 		return loginsByRole, nil
// 	}
// 	for _, role := range req.GetRoles() {
// 		currentRole, err := handler.Client.GetRole(ctx, role)
// 		if err != nil {
// 			return nil, trace.Wrap(err)
// 		}
// 		currentRole, err = services.ApplyTraits(currentRole, user.GetTraits())
// 		if err != nil {
// 			return nil, trace.Wrap(err)
// 		}
// 		logins := currentRole.GetLogins(types.Allow)
// 		if logins == nil {
// 			logins = []string{}
// 		}
// 		loginsByRole[role] = logins
// 	}
// 	return loginsByRole, nil
// }

// func (s *NotificationCAS) getResourceNames(ctx context.Context, req types.AccessRequest) ([]string, error) {
// 	resourceNames := make([]string, 0, len(req.GetRequestedResourceIDs()))
// 	resourcesByCluster := accessrequest.GetResourceIDsByCluster(req)

// 	for cluster, resources := range resourcesByCluster {
// 		resourceDetails, err := accessrequest.GetResourceDetails(ctx, cluster, handler.Client, resources)
// 		if err != nil {
// 			return nil, trace.Wrap(err)
// 		}

// 		for _, resource := range resources {
// 			resourceName := types.ResourceIDToString(resource)
// 			if details, ok := resourceDetails[resourceName]; ok && details.FriendlyName != "" {
// 				resourceName = fmt.Sprintf("%s/%s", resource.Kind, details.FriendlyName)
// 			}
// 			resourceNames = append(resourceNames, resourceName)
// 		}
// 	}
// 	return resourceNames, nil
// }
