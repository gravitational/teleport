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
	"reflect"

	"github.com/gravitational/trace"
)

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
	case *UserTokenCreate:
		out.Event = &OneOf_UserTokenCreate{
			UserTokenCreate: e,
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
	case *AppCreate:
		out.Event = &OneOf_AppCreate{
			AppCreate: e,
		}
	case *AppUpdate:
		out.Event = &OneOf_AppUpdate{
			AppUpdate: e,
		}
	case *AppDelete:
		out.Event = &OneOf_AppDelete{
			AppDelete: e,
		}
	case *DatabaseCreate:
		out.Event = &OneOf_DatabaseCreate{
			DatabaseCreate: e,
		}
	case *DatabaseUpdate:
		out.Event = &OneOf_DatabaseUpdate{
			DatabaseUpdate: e,
		}
	case *DatabaseDelete:
		out.Event = &OneOf_DatabaseDelete{
			DatabaseDelete: e,
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
	case *PostgresParse:
		out.Event = &OneOf_PostgresParse{
			PostgresParse: e,
		}
	case *PostgresBind:
		out.Event = &OneOf_PostgresBind{
			PostgresBind: e,
		}
	case *PostgresExecute:
		out.Event = &OneOf_PostgresExecute{
			PostgresExecute: e,
		}
	case *PostgresClose:
		out.Event = &OneOf_PostgresClose{
			PostgresClose: e,
		}
	case *PostgresFunctionCall:
		out.Event = &OneOf_PostgresFunctionCall{
			PostgresFunctionCall: e,
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
	case *BillingCardCreate:
		out.Event = &OneOf_BillingCardCreate{
			BillingCardCreate: e,
		}
	case *BillingCardDelete:
		out.Event = &OneOf_BillingCardDelete{
			BillingCardDelete: e,
		}
	case *LockCreate:
		out.Event = &OneOf_LockCreate{
			LockCreate: e,
		}
	case *LockDelete:
		out.Event = &OneOf_LockDelete{
			LockDelete: e,
		}
	case *BillingInformationUpdate:
		out.Event = &OneOf_BillingInformationUpdate{
			BillingInformationUpdate: e,
		}
	case *RecoveryCodeGenerate:
		out.Event = &OneOf_RecoveryCodeGenerate{
			RecoveryCodeGenerate: e,
		}
	case *RecoveryCodeUsed:
		out.Event = &OneOf_RecoveryCodeUsed{
			RecoveryCodeUsed: e,
		}
	case *WindowsDesktopSessionStart:
		out.Event = &OneOf_WindowsDesktopSessionStart{
			WindowsDesktopSessionStart: e,
		}
	case *WindowsDesktopSessionEnd:
		out.Event = &OneOf_WindowsDesktopSessionEnd{
			WindowsDesktopSessionEnd: e,
		}
	case *SessionConnect:
		out.Event = &OneOf_SessionConnect{
			SessionConnect: e,
		}
	case *AccessRequestDelete:
		out.Event = &OneOf_AccessRequestDelete{
			AccessRequestDelete: e,
		}
	case *CertificateCreate:
		out.Event = &OneOf_CertificateCreate{
			CertificateCreate: e,
		}
	default:
		return nil, trace.BadParameter("event type %T is not supported", in)
	}
	return &out, nil
}

// FromOneOf converts audit event from one of wrapper to interface
func FromOneOf(in OneOf) (AuditEvent, error) {
	e := in.GetEvent()
	if e == nil {
		return nil, trace.BadParameter("failed to parse event, session record is corrupted")
	}

	// We go from e (isOneOf_Event) -> reflect.Value (*OneOf_SomeStruct) -> reflect.Value(OneOf_SomeStruct).
	elem := reflect.ValueOf(in.GetEvent()).Elem()

	// OneOfs only have one inner field, verify and then read it.
	if elem.NumField() != 1 {
		// This should never happen for proto one-ofs.
		return nil, trace.BadParameter("unexpect number in value %v: %v != 1", elem.Kind(), elem.NumField())
	}

	auditEvent, ok := elem.Field(0).Interface().(AuditEvent)
	if !ok || reflect.ValueOf(auditEvent).IsNil() {
		return nil, trace.BadParameter("received unsupported event %T", in.Event)
	}
	return auditEvent, nil
}
