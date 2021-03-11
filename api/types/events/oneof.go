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

import "github.com/gravitational/trace"

// MustToOneOf converts audit event to OneOf
// or panics, used in tests
func MustToOneOf(in AuditEvent) *OneOf {
	out, err := ToOneOf(in)
	if err != nil {
		panic(err)
	}
	return out
}

// ToOneOf converts audit event to union type of the events
func ToOneOf(in AuditEvent) (*OneOf, error) {
	out := OneOf{}

	switch e := in.(type) {
	case *UserLogin:
		out.Event = &OneOf_UserLogin{
			UserLogin: e,
		}
	case *UserCreate:
		out.Event = &OneOf_UserCreate{
			UserCreate: e,
		}
	case *UserDelete:
		out.Event = &OneOf_UserDelete{
			UserDelete: e,
		}
	case *UserPasswordChange:
		out.Event = &OneOf_UserPasswordChange{
			UserPasswordChange: e,
		}
	case *SessionStart:
		out.Event = &OneOf_SessionStart{
			SessionStart: e,
		}
	case *SessionJoin:
		out.Event = &OneOf_SessionJoin{
			SessionJoin: e,
		}
	case *SessionPrint:
		out.Event = &OneOf_SessionPrint{
			SessionPrint: e,
		}
	case *SessionReject:
		out.Event = &OneOf_SessionReject{
			SessionReject: e,
		}
	case *Resize:
		out.Event = &OneOf_Resize{
			Resize: e,
		}
	case *SessionEnd:
		out.Event = &OneOf_SessionEnd{
			SessionEnd: e,
		}
	case *SessionCommand:
		out.Event = &OneOf_SessionCommand{
			SessionCommand: e,
		}
	case *SessionDisk:
		out.Event = &OneOf_SessionDisk{
			SessionDisk: e,
		}
	case *SessionNetwork:
		out.Event = &OneOf_SessionNetwork{
			SessionNetwork: e,
		}
	case *SessionData:
		out.Event = &OneOf_SessionData{
			SessionData: e,
		}
	case *SessionLeave:
		out.Event = &OneOf_SessionLeave{
			SessionLeave: e,
		}
	case *PortForward:
		out.Event = &OneOf_PortForward{
			PortForward: e,
		}
	case *X11Forward:
		out.Event = &OneOf_X11Forward{
			X11Forward: e,
		}
	case *Subsystem:
		out.Event = &OneOf_Subsystem{
			Subsystem: e,
		}
	case *SCP:
		out.Event = &OneOf_SCP{
			SCP: e,
		}
	case *Exec:
		out.Event = &OneOf_Exec{
			Exec: e,
		}
	case *ClientDisconnect:
		out.Event = &OneOf_ClientDisconnect{
			ClientDisconnect: e,
		}
	case *AuthAttempt:
		out.Event = &OneOf_AuthAttempt{
			AuthAttempt: e,
		}
	case *AccessRequestCreate:
		out.Event = &OneOf_AccessRequestCreate{
			AccessRequestCreate: e,
		}
	case *RoleCreate:
		out.Event = &OneOf_RoleCreate{
			RoleCreate: e,
		}
	case *RoleDelete:
		out.Event = &OneOf_RoleDelete{
			RoleDelete: e,
		}
	case *ResetPasswordTokenCreate:
		out.Event = &OneOf_ResetPasswordTokenCreate{
			ResetPasswordTokenCreate: e,
		}
	case *TrustedClusterCreate:
		out.Event = &OneOf_TrustedClusterCreate{
			TrustedClusterCreate: e,
		}
	case *TrustedClusterDelete:
		out.Event = &OneOf_TrustedClusterDelete{
			TrustedClusterDelete: e,
		}
	case *TrustedClusterTokenCreate:
		out.Event = &OneOf_TrustedClusterTokenCreate{
			TrustedClusterTokenCreate: e,
		}
	case *GithubConnectorCreate:
		out.Event = &OneOf_GithubConnectorCreate{
			GithubConnectorCreate: e,
		}
	case *GithubConnectorDelete:
		out.Event = &OneOf_GithubConnectorDelete{
			GithubConnectorDelete: e,
		}
	case *OIDCConnectorCreate:
		out.Event = &OneOf_OIDCConnectorCreate{
			OIDCConnectorCreate: e,
		}
	case *OIDCConnectorDelete:
		out.Event = &OneOf_OIDCConnectorDelete{
			OIDCConnectorDelete: e,
		}
	case *SAMLConnectorCreate:
		out.Event = &OneOf_SAMLConnectorCreate{
			SAMLConnectorCreate: e,
		}
	case *SAMLConnectorDelete:
		out.Event = &OneOf_SAMLConnectorDelete{
			SAMLConnectorDelete: e,
		}
	case *KubeRequest:
		out.Event = &OneOf_KubeRequest{
			KubeRequest: e,
		}
	case *AppSessionStart:
		out.Event = &OneOf_AppSessionStart{
			AppSessionStart: e,
		}
	case *AppSessionChunk:
		out.Event = &OneOf_AppSessionChunk{
			AppSessionChunk: e,
		}
	case *AppSessionRequest:
		out.Event = &OneOf_AppSessionRequest{
			AppSessionRequest: e,
		}
	case *DatabaseSessionStart:
		out.Event = &OneOf_DatabaseSessionStart{
			DatabaseSessionStart: e,
		}
	case *DatabaseSessionEnd:
		out.Event = &OneOf_DatabaseSessionEnd{
			DatabaseSessionEnd: e,
		}
	case *DatabaseSessionQuery:
		out.Event = &OneOf_DatabaseSessionQuery{
			DatabaseSessionQuery: e,
		}
	case *SessionUpload:
		out.Event = &OneOf_SessionUpload{
			SessionUpload: e,
		}
	case *MFADeviceAdd:
		out.Event = &OneOf_MFADeviceAdd{
			MFADeviceAdd: e,
		}
	case *MFADeviceDelete:
		out.Event = &OneOf_MFADeviceDelete{
			MFADeviceDelete: e,
		}
	default:
		return nil, trace.BadParameter("event type %T is not supported", in)
	}
	return &out, nil
}

// FromOneOf converts audit event from one of wrapper to interface
func FromOneOf(in OneOf) (AuditEvent, error) {
	if e := in.GetUserLogin(); e != nil {
		return e, nil
	} else if e := in.GetUserCreate(); e != nil {
		return e, nil
	} else if e := in.GetUserDelete(); e != nil {
		return e, nil
	} else if e := in.GetUserPasswordChange(); e != nil {
		return e, nil
	} else if e := in.GetSessionStart(); e != nil {
		return e, nil
	} else if e := in.GetSessionJoin(); e != nil {
		return e, nil
	} else if e := in.GetSessionPrint(); e != nil {
		return e, nil
	} else if e := in.GetSessionReject(); e != nil {
		return e, nil
	} else if e := in.GetResize(); e != nil {
		return e, nil
	} else if e := in.GetSessionEnd(); e != nil {
		return e, nil
	} else if e := in.GetSessionCommand(); e != nil {
		return e, nil
	} else if e := in.GetSessionDisk(); e != nil {
		return e, nil
	} else if e := in.GetSessionNetwork(); e != nil {
		return e, nil
	} else if e := in.GetSessionData(); e != nil {
		return e, nil
	} else if e := in.GetSessionLeave(); e != nil {
		return e, nil
	} else if e := in.GetPortForward(); e != nil {
		return e, nil
	} else if e := in.GetX11Forward(); e != nil {
		return e, nil
	} else if e := in.GetSCP(); e != nil {
		return e, nil
	} else if e := in.GetExec(); e != nil {
		return e, nil
	} else if e := in.GetSubsystem(); e != nil {
		return e, nil
	} else if e := in.GetClientDisconnect(); e != nil {
		return e, nil
	} else if e := in.GetAuthAttempt(); e != nil {
		return e, nil
	} else if e := in.GetAccessRequestCreate(); e != nil {
		return e, nil
	} else if e := in.GetResetPasswordTokenCreate(); e != nil {
		return e, nil
	} else if e := in.GetRoleCreate(); e != nil {
		return e, nil
	} else if e := in.GetRoleDelete(); e != nil {
		return e, nil
	} else if e := in.GetTrustedClusterCreate(); e != nil {
		return e, nil
	} else if e := in.GetTrustedClusterDelete(); e != nil {
		return e, nil
	} else if e := in.GetTrustedClusterTokenCreate(); e != nil {
		return e, nil
	} else if e := in.GetGithubConnectorCreate(); e != nil {
		return e, nil
	} else if e := in.GetGithubConnectorDelete(); e != nil {
		return e, nil
	} else if e := in.GetOIDCConnectorCreate(); e != nil {
		return e, nil
	} else if e := in.GetOIDCConnectorDelete(); e != nil {
		return e, nil
	} else if e := in.GetSAMLConnectorCreate(); e != nil {
		return e, nil
	} else if e := in.GetSAMLConnectorDelete(); e != nil {
		return e, nil
	} else if e := in.GetKubeRequest(); e != nil {
		return e, nil
	} else if e := in.GetAppSessionStart(); e != nil {
		return e, nil
	} else if e := in.GetAppSessionChunk(); e != nil {
		return e, nil
	} else if e := in.GetAppSessionRequest(); e != nil {
		return e, nil
	} else if e := in.GetDatabaseSessionStart(); e != nil {
		return e, nil
	} else if e := in.GetDatabaseSessionEnd(); e != nil {
		return e, nil
	} else if e := in.GetDatabaseSessionQuery(); e != nil {
		return e, nil
	} else if e := in.GetSessionUpload(); e != nil {
		return e, nil
	} else if e := in.GetMFADeviceAdd(); e != nil {
		return e, nil
	} else if e := in.GetMFADeviceDelete(); e != nil {
		return e, nil
	} else {
		if in.Event == nil {
			return nil, trace.BadParameter("failed to parse event, session record is corrupted")
		}
		return nil, trace.BadParameter("received unsupported event %T", in.Event)
	}
}
