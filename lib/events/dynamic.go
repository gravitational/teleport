/*
Copyright 2021 Gravitational, Inc.

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

package events

import (
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"encoding/json"
)

// FromEventFields converts from the typed dynamic representation
// to the new typed interface-style representation.
//
// This is mainly used to convert from the backend format used by
// our various event backends.
func FromEventFields(fields EventFields) (AuditEvent, error) {
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	eventType := fields.GetString(EventType)

	switch eventType {
	case SessionPrintEvent:
		var e events.SessionPrint
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionStartEvent:
		var e events.SessionStart
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionEndEvent:
		var e events.SessionEnd
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionUploadEvent:
		var e events.SessionUpload
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionJoinEvent:
		var e events.SessionJoin
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionLeaveEvent:
		var e events.SessionLeave
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionDataEvent:
		var e events.SessionData
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case ClientDisconnectEvent:
		var e events.ClientDisconnect
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case UserLoginEvent:
		var e events.UserLogin
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case UserDeleteEvent:
		var e events.UserDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case UserCreateEvent:
		var e events.UserCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case UserUpdatedEvent:
		// note: user.update is a custom code applied on top of the same data as the user.create event
		//       and they are thus functionally identical. There exists no direct gRPC version of user.update.
		var e events.UserCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case UserPasswordChangeEvent:
		var e events.UserPasswordChange
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case AccessRequestCreateEvent:
		var e events.AccessRequestCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case AccessRequestUpdateEvent:
		var e events.AccessRequestCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case BillingCardCreateEvent:
		var e events.BillingCardCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case BillingCardUpdateEvent:
		var e events.BillingCardCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case BillingCardDeleteEvent:
		var e events.BillingCardDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case BillingInformationUpdateEvent:
		var e events.BillingInformationUpdate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case ResetPasswordTokenCreateEvent:
		var e events.ResetPasswordTokenCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case ExecEvent:
		var e events.Exec
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SubsystemEvent:
		var e events.Subsystem
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case X11ForwardEvent:
		var e events.X11Forward
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case PortForwardEvent:
		var e events.PortForward
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case AuthAttemptEvent:
		var e events.AuthAttempt
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SCPEvent:
		var e events.SCP
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case ResizeEvent:
		var e events.Resize
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionCommandEvent:
		var e events.SessionCommand
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionDiskEvent:
		var e events.SessionDisk
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionNetworkEvent:
		var e events.SessionNetwork
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case RoleCreatedEvent:
		var e events.RoleCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case RoleDeletedEvent:
		var e events.RoleDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case TrustedClusterCreateEvent:
		var e events.TrustedClusterCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case TrustedClusterDeleteEvent:
		var e events.TrustedClusterDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case TrustedClusterTokenCreateEvent:
		var e events.TrustedClusterTokenCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case GithubConnectorCreatedEvent:
		var e events.GithubConnectorCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case GithubConnectorDeletedEvent:
		var e events.GithubConnectorDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case OIDCConnectorCreatedEvent:
		var e events.OIDCConnectorCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case OIDCConnectorDeletedEvent:
		var e events.OIDCConnectorDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SAMLConnectorCreatedEvent:
		var e events.SAMLConnectorCreate
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SAMLConnectorDeletedEvent:
		var e events.SAMLConnectorDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case SessionRejectedEvent:
		var e events.SessionReject
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case AppSessionStartEvent:
		var e events.AppSessionStart
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case AppSessionChunkEvent:
		var e events.AppSessionChunk
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case AppSessionRequestEvent:
		var e events.AppSessionRequest
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case DatabaseSessionStartEvent:
		var e events.DatabaseSessionStart
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case DatabaseSessionEndEvent:
		var e events.DatabaseSessionEnd
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case DatabaseSessionQueryEvent:
		var e events.DatabaseSessionQuery
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case KubeRequestEvent:
		var e events.KubeRequest
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case MFADeviceAddEvent:
		var e events.MFADeviceAdd
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	case MFADeviceDeleteEvent:
		var e events.MFADeviceDelete
		if err := utils.FastUnmarshal(data, &e); err != nil {
			return nil, trace.Wrap(err)
		}
		return &e, nil
	default:
		return nil, trace.BadParameter("unknown event type: %q", eventType)
	}
}

// GetSessionID pulls the session ID from the events that have a
// SessionMetadata. For other events an empty string is returned.
func GetSessionID(event AuditEvent) string {
	var sessionID string

	if g, ok := event.(SessionMetadataGetter); ok {
		sessionID = g.GetSessionID()
	}

	return sessionID
}

// ToEventFields converts from the typed interface-style event representation
// to the old dynamic map style representation in order to provide outer compatibility
// with existing public API routes when the backend is updated with the typed events.
func ToEventFields(event AuditEvent) (EventFields, error) {
	var fields EventFields
	if err := utils.ObjectToStruct(event, &fields); err != nil {
		return nil, trace.Wrap(err)
	}

	return fields, nil
}
