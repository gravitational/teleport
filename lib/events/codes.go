/*
Copyright 2019 Gravitational, Inc.

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

// Event describes an audit log event.
type Event struct {
	// Name is the event name.
	Name string
	// Code is the unique event code.
	Code string
}

var (
	// UserLocalLoginE is emitted when a local user successfully logs in.
	UserLocalLoginE = Event{
		Name: UserLoginEvent,
		Code: UserLocalLoginCode,
	}
	// UserLocalLoginFailureE is emitted when a local user login attempt fails.
	UserLocalLoginFailureE = Event{
		Name: UserLoginEvent,
		Code: UserLocalLoginFailureCode,
	}
	// UserSSOLoginE is emitted when an SSO user successfully logs in.
	UserSSOLoginE = Event{
		Name: UserLoginEvent,
		Code: UserSSOLoginCode,
	}
	// UserSSOLoginFailureE is emitted when an SSO user login attempt fails.
	UserSSOLoginFailureE = Event{
		Name: UserLoginEvent,
		Code: UserSSOLoginFailureCode,
	}
	// UserUpdateE is emitted when a user is updated.
	UserUpdateE = Event{
		Name: UserUpdatedEvent,
		Code: UserUpdateCode,
	}
	// UserDeleteE is emitted when a user is deleted.
	UserDeleteE = Event{
		Name: UserDeleteEvent,
		Code: UserDeleteCode,
	}
	// UserCreateE is emitted when a user is created.
	UserCreateE = Event{
		Name: UserCreateEvent,
		Code: UserCreateCode,
	}
	// UserPasswordChangeE is emitted when a user changes their own password.
	UserPasswordChangeE = Event{
		Name: UserPasswordChangeEvent,
		Code: UserPasswordChangeCode,
	}
	// SessionStartE is emitted when a user starts a new session.
	SessionStartE = Event{
		Name: SessionStartEvent,
		Code: SessionStartCode,
	}
	// SessionJoinE is emitted when a user joins the session.
	SessionJoinE = Event{
		Name: SessionJoinEvent,
		Code: SessionJoinCode,
	}
	// TerminalResizeE is emitted when a user resizes the terminal.
	TerminalResizeE = Event{
		Name: ResizeEvent,
		Code: TerminalResizeCode,
	}
	// SessionLeaveE is emitted when a user leaves the session.
	SessionLeaveE = Event{
		Name: SessionLeaveEvent,
		Code: SessionLeaveCode,
	}
	// SessionEndE is emitted when a user ends the session.
	SessionEndE = Event{
		Name: SessionEndEvent,
		Code: SessionEndCode,
	}
	// SessionUploadE is emitted after a session recording has been uploaded.
	SessionUploadE = Event{
		Name: SessionUploadEvent,
		Code: SessionUploadCode,
	}
	// SessionDataE is emitted to report session data usage.
	SessionDataE = Event{
		Name: SessionDataEvent,
		Code: SessionDataCode,
	}
	// SubsystemE is emitted when a user requests a new subsystem.
	SubsystemE = Event{
		Name: SubsystemEvent,
		Code: SubsystemCode,
	}
	// SubsystemFailureE is emitted when a user subsystem request fails.
	SubsystemFailureE = Event{
		Name: SubsystemEvent,
		Code: SubsystemFailureCode,
	}
	// ExecE is emitted when a user executes a command on a node.
	ExecE = Event{
		Name: ExecEvent,
		Code: ExecCode,
	}
	// ExecFailureE is emitted when a user command execution fails.
	ExecFailureE = Event{
		Name: ExecEvent,
		Code: ExecFailureCode,
	}
	// X11ForwardE is emitted when a user requests X11 forwarding.
	X11ForwardE = Event{
		Name: X11ForwardEvent,
		Code: X11ForwardCode,
	}
	// X11ForwardFailureE is emitted when an X11 forwarding request fails.
	X11ForwardFailureE = Event{
		Name: X11ForwardEvent,
		Code: X11ForwardFailureCode,
	}
	// PortForwardE is emitted when a user requests port forwarding.
	PortForwardE = Event{
		Name: PortForwardEvent,
		Code: PortForwardCode,
	}
	// PortForwardFailureE is emitted when a port forward request fails.
	PortForwardFailureE = Event{
		Name: PortForwardEvent,
		Code: PortForwardFailureCode,
	}
	// SCPDownloadE is emitted when a user downloads a file.
	SCPDownloadE = Event{
		Name: SCPEvent,
		Code: SCPDownloadCode,
	}
	// SCPDownloadFailureE is emitted when a file download fails.
	SCPDownloadFailureE = Event{
		Name: SCPEvent,
		Code: SCPDownloadFailureCode,
	}
	// SCPUploadE is emitted when a user uploads a file.
	SCPUploadE = Event{
		Name: SCPEvent,
		Code: SCPUploadCode,
	}
	// SCPUploadFailureE is emitted when a file upload fails.
	SCPUploadFailureE = Event{
		Name: SCPEvent,
		Code: SCPUploadFailureCode,
	}
	// ClientDisconnectE is emitted when a user session is disconnected.
	ClientDisconnectE = Event{
		Name: ClientDisconnectEvent,
		Code: ClientDisconnectCode,
	}
	// AuthAttemptFailureE is emitted upon a failed authentication attempt.
	AuthAttemptFailureE = Event{
		Name: AuthAttemptEvent,
		Code: AuthAttemptFailureCode,
	}
	// AccessRequestCreatedE is emitted when an access request is created.
	AccessRequestCreatedE = Event{
		Name: AccessRequestCreateEvent,
		Code: AccessRequestCreateCode,
	}
	AccessRequestUpdatedE = Event{
		Name: AccessRequestUpdateEvent,
		Code: AccessRequestUpdateCode,
	}
	// SessionCommandE is emitted upon execution of a command when using enhanced
	// session recording.
	SessionCommandE = Event{
		Name: SessionCommandEvent,
		Code: SessionCommandCode,
	}
	// SessionDiskE is emitted upon open of a file when using enhanced session recording.
	SessionDiskE = Event{
		Name: SessionDiskEvent,
		Code: SessionDiskCode,
	}
	// SessionNetworkE is emitted when a network request is issued when
	// using enhanced session recording.
	SessionNetworkE = Event{
		Name: SessionNetworkEvent,
		Code: SessionNetworkCode,
	}
	// ResetPasswordTokenCreatedE is emitted when a password reset token is created.
	ResetPasswordTokenCreatedE = Event{
		Name: ResetPasswordTokenCreateEvent,
		Code: ResetPasswordTokenCreateCode,
	}
	// RoleCreatedE is emitted when a role is created/updated.
	RoleCreatedE = Event{
		Name: RoleCreatedEvent,
		Code: RoleCreatedCode,
	}
	// RoleDeletedE is emitted when a role is deleted.
	RoleDeletedE = Event{
		Name: RoleDeletedEvent,
		Code: RoleDeletedCode,
	}
	// TrustedClusterCreateE is emitted when a trusted cluster relationship is created.
	TrustedClusterCreateE = Event{
		Name: TrustedClusterCreateEvent,
		Code: TrustedClusterCreateCode,
	}
	// TrustedClusterDeleteE is emitted when a trusted cluster is removed from the root cluster.
	TrustedClusterDeleteE = Event{
		Name: TrustedClusterDeleteEvent,
		Code: TrustedClusterDeleteCode,
	}
	// TrustedClusterTokenCreateE is emitted when a new join
	// token for trusted cluster is created.
	TrustedClusterTokenCreateE = Event{
		Name: TrustedClusterTokenCreateEvent,
		Code: TrustedClusterTokenCreateCode,
	}
	// GithubConnectorCreatedE is emitted when a Github connector is created/updated.
	GithubConnectorCreatedE = Event{
		Name: GithubConnectorCreatedEvent,
		Code: GithubConnectorCreatedCode,
	}
	// GithubConnectorDeletedE is emitted when a Github connector is deleted.
	GithubConnectorDeletedE = Event{
		Name: GithubConnectorDeletedEvent,
		Code: GithubConnectorDeletedCode,
	}
	// OIDCConnectorCreatedE is emitted when an OIDC connector is created/updated.
	OIDCConnectorCreatedE = Event{
		Name: OIDCConnectorCreatedEvent,
		Code: OIDCConnectorCreatedCode,
	}
	// OIDCConnectorDeletedE is emitted when an OIDC connector is deleted.
	OIDCConnectorDeletedE = Event{
		Name: OIDCConnectorDeletedEvent,
		Code: OIDCConnectorDeletedCode,
	}
	// SAMLConnectorCreatedE is emitted when a SAML connector is created/updated.
	SAMLConnectorCreatedE = Event{
		Name: SAMLConnectorCreatedEvent,
		Code: SAMLConnectorCreatedCode,
	}
	// SAMLConnectorDeletedE is emitted when a SAML connector is deleted.
	SAMLConnectorDeletedE = Event{
		Name: SAMLConnectorDeletedEvent,
		Code: SAMLConnectorDeletedCode,
	}
	// SessionRejectedE is emitted when a user hits `max_connections`.
	SessionRejectedE = Event{
		Name: SessionRejectedEvent,
		Code: SessionRejectedCode,
	}
)

// There is no strict algorithm for picking an event code, however existing
// event codes are currently loosely categorized as follows:
//
//  * Teleport event codes start with "T" and belong in this const block.
//
//  * Related events are grouped starting with the same number.
//		eg: All user related events are grouped under 1xxx.
//
//  * Suffix code with one of these letters: I (info), W (warn), E (error).
const (
	// UserLocalLoginCode is the successful local user login event code.
	UserLocalLoginCode = "T1000I"
	// UserLocalLoginFailureCode is the unsuccessful local user login event code.
	UserLocalLoginFailureCode = "T1000W"
	// UserSSOLoginCode is the successful SSO user login event code.
	UserSSOLoginCode = "T1001I"
	// UserSSOLoginFailureCode is the unsuccessful SSO user login event code.
	UserSSOLoginFailureCode = "T1001W"
	// UserCreateCode is the user create event code.
	UserCreateCode = "T1002I"
	// UserUpdateCode is the user update event code.
	UserUpdateCode = "T1003I"
	// UserDeleteCode is the user delete event code.
	UserDeleteCode = "T1004I"
	// UserPasswordChangeCode is an event code for when user changes their own password.
	UserPasswordChangeCode = "T1005I"
	// MFADeviceAddEventCode is an event code for users adding MFA devices.
	MFADeviceAddEventCode = "T1006I"
	// MFADeviceDeleteEventCode is an event code for users deleting MFA devices.
	MFADeviceDeleteEventCode = "T1007I"
	// RecoveryCodesGenerateCode is an event code for generation of recovery codes.
	RecoveryCodesGenerateCode = "T1008I"
	// RecoveryCodeUseSuccessCode is an event code for when a
	// recovery code was used successfully.
	RecoveryCodeUseSuccessCode = "T1009I"
	// RecoveryCodeUseFailureCode is an event code for when a
	// recovery code was not used successfully.
	RecoveryCodeUseFailureCode = "T1009W"

	// BillingCardCreateCode is an event code for when a user creates a new credit card.
	BillingCardCreateCode = "TBL00I"
	// BillingCardDeleteCode is an event code for when a user deletes a credit card.
	BillingCardDeleteCode = "TBL01I"
	// BillingCardUpdateCode is an event code for when a user updates an existing credit card.
	BillingCardUpdateCode = "TBL02I"
	// BillingInformationUpdateCode is an event code for when a user updates their billing info.
	BillingInformationUpdateCode = "TBL03I"

	// SessionRejectedCode is an event code for when a user's attempt to create an
	// session/connection has been rejected.
	SessionRejectedCode = "T1006W"

	// SessionStartCode is the session start event code.
	SessionStartCode = "T2000I"
	// SessionJoinCode is the session join event code.
	SessionJoinCode = "T2001I"
	// TerminalResizeCode is the terminal resize event code.
	TerminalResizeCode = "T2002I"
	// SessionLeaveCode is the session leave event code.
	SessionLeaveCode = "T2003I"
	// SessionEndCode is the session end event code.
	SessionEndCode = "T2004I"
	// SessionUploadCode is the session upload event code.
	SessionUploadCode = "T2005I"
	// SessionDataCode is the session data event code.
	SessionDataCode = "T2006I"

	// AppCreateCode is the app.create event code.
	AppCreateCode = "TAP03I"
	// AppUpdateCode is the app.update event code.
	AppUpdateCode = "TAP04I"
	// AppDeleteCode is the app.delete event code.
	AppDeleteCode = "TAP05I"

	// AppSessionStartCode is the application session start code.
	AppSessionStartCode = "T2007I"
	// AppSessionChunkCode is the application session chunk create code.
	AppSessionChunkCode = "T2008I"
	// AppSessionRequestCode is the application request/response code.
	AppSessionRequestCode = "T2009I"

	// DatabaseSessioStartCode is the database session start event code.
	DatabaseSessionStartCode = "TDB00I"
	// DatabaseSessionStartFailureCode is the database session start failure event code.
	DatabaseSessionStartFailureCode = "TDB00W"
	// DatabaseSessionEndCode is the database session end event code.
	DatabaseSessionEndCode = "TDB01I"
	// DatabaseSessionQueryCode is the database query event code.
	DatabaseSessionQueryCode = "TDB02I"
	// DatabaseSessionQueryFailedCode is the database query failure event code.
	DatabaseSessionQueryFailedCode = "TDB02W"

	// DatabaseCreateCode is the db.create event code.
	DatabaseCreateCode = "TDB03I"
	// DatabaseUpdateCode is the db.update event code.
	DatabaseUpdateCode = "TDB04I"
	// DatabaseDeleteCode is the db.delete event code.
	DatabaseDeleteCode = "TDB05I"

	// SubsystemCode is the subsystem event code.
	SubsystemCode = "T3001I"
	// SubsystemFailureCode is the subsystem failure event code.
	SubsystemFailureCode = "T3001E"
	// ExecCode is the exec event code.
	ExecCode = "T3002I"
	// ExecFailureCode is the exec failure event code.
	ExecFailureCode = "T3002E"
	// PortForwardCode is the port forward event code.
	PortForwardCode = "T3003I"
	// PortForwardFailureCode is the port forward failure event code.
	PortForwardFailureCode = "T3003E"
	// SCPDownloadCode is the file download event code.
	SCPDownloadCode = "T3004I"
	// SCPDownloadFailureCode is the file download event failure code.
	SCPDownloadFailureCode = "T3004E"
	// SCPUploadCode is the file upload event code.
	SCPUploadCode = "T3005I"
	// SCPUploadFailureCode is the file upload failure event code.
	SCPUploadFailureCode = "T3005E"
	// ClientDisconnectCode is the client disconnect event code.
	ClientDisconnectCode = "T3006I"
	// AuthAttemptFailureCode is the auth attempt failure event code.
	AuthAttemptFailureCode = "T3007W"
	// X11ForwardCode is the x11 forward event code.
	X11ForwardCode = "T3008I"
	// X11ForwardFailureCode is the x11 forward failure event code.
	X11ForwardFailureCode = "T3008W"
	// KubeRequestCode is an event code for a generic kubernetes request.
	//
	// Note: some requests (like exec into a pod) use other codes (like
	// ExecCode).
	KubeRequestCode = "T3009I"

	// SessionCommandCode is a session command code.
	SessionCommandCode = "T4000I"
	// SessionDiskCode is a session disk code.
	SessionDiskCode = "T4001I"
	// SessionNetworkCode is a session network code.
	SessionNetworkCode = "T4002I"

	// AccessRequestCreateCode is the the access request creation code.
	AccessRequestCreateCode = "T5000I"
	// AccessRequestUpdateCode is the access request state update code.
	AccessRequestUpdateCode = "T5001I"
	// AccessRequestReviewCode is the access review application code.
	AccessRequestReviewCode = "T5002I"

	// ResetPasswordTokenCreateCode is the token create event code.
	ResetPasswordTokenCreateCode = "T6000I"
	// RecoveryTokenCreateCode is the recovery token create event code.
	RecoveryTokenCreateCode = "T6001I"
	// PrivilegeTokenCreateCode is the recovery token create event code.
	PrivilegeTokenCreateCode = "T6001I"

	// TrustedClusterCreateCode is the event code for creating a trusted cluster.
	TrustedClusterCreateCode = "T7000I"
	// TrustedClusterDeleteCode is the event code for removing a trusted cluster.
	TrustedClusterDeleteCode = "T7001I"
	// TrustedClusterTokenCreateCode is the event code for
	// creating new join token for a trusted cluster.
	TrustedClusterTokenCreateCode = "T7002I"

	// GithubConnectorCreatedCode is the Github connector created event code.
	GithubConnectorCreatedCode = "T8000I"
	// GithubConnectorDeletedCode is the Github connector deleted event code.
	GithubConnectorDeletedCode = "T8001I"

	// OIDCConnectorCreatedCode is the OIDC connector created event code.
	OIDCConnectorCreatedCode = "T8100I"
	// OIDCConnectorDeletedCode is the OIDC connector deleted event code.
	OIDCConnectorDeletedCode = "T8101I"

	// SAMLConnectorCreatedCode is the SAML connector created event code.
	SAMLConnectorCreatedCode = "T8200I"
	// SAMLConnectorDeletedCode is the SAML connector deleted event code.
	SAMLConnectorDeletedCode = "T8201I"

	// RoleCreatedCode is the role created event code.
	RoleCreatedCode = "T9000I"
	// RoleDeletedCode is the role deleted event code.
	RoleDeletedCode = "T9001I"

	// LockCreatedCode is the lock created event code.
	LockCreatedCode = "TLK00I"
	// LockDeletedCode is the lock deleted event code.
	LockDeletedCode = "TLK01I"
)
