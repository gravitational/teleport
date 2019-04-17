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
	// Severity is the event severity (info, warning, error).
	Severity string
	// Message contains the default event message template.
	Message string
}

var (
	// UserLocalLogin is emitted when a local user successfully logs in.
	UserLocalLogin = Event{
		Name:     UserLoginEvent,
		Code:     UserLocalLoginCode,
		Severity: SeverityInfo,
		Message:  "Local user {{.user}} successfully logged in",
	}
	// UserLocalLoginFailure is emitted when a local user login attempt fails.
	UserLocalLoginFailure = Event{
		Name:     UserLoginEvent,
		Code:     UserLocalLoginFailureCode,
		Severity: SeverityWarning,
		Message:  "Local user {{.user}} login failed: {{.error}}",
	}
	// UserSSOLogin is emitted when an SSO user successfully logs in.
	UserSSOLogin = Event{
		Name:     UserLoginEvent,
		Code:     UserSSOLoginCode,
		Severity: SeverityInfo,
		Message:  "SSO user {{.user}} successfully logged in",
	}
	// UserSSOLoginFailure is emitted when an SSO user login attempt fails.
	UserSSOLoginFailure = Event{
		Name:     UserLoginEvent,
		Code:     UserSSOLoginFailureCode,
		Severity: SeverityWarning,
		Message:  "SSO user login failed: {{.error}}",
	}
	// UserUpdate is emitted when a user is upserted.
	UserUpdate = Event{
		Name:     UserUpdatedEvent,
		Code:     UserUpdateCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} information has been updated",
	}
	// UserDelete is emitted when a user is deleted.
	UserDelete = Event{
		Name:     UserDeleteEvent,
		Code:     UserDeleteCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} has been deleted",
	}
	// SessionStart is emitted when a user starts a new session.
	SessionStart = Event{
		Name:     SessionStartEvent,
		Code:     SessionStartCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} has started a session",
	}
	// SessionJoin is emitted when a user joins the session.
	SessionJoin = Event{
		Name:     SessionJoinEvent,
		Code:     SessionJoinCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} has joined the session",
	}
	// TerminalResize is emitted when a user resizes the terminal.
	TerminalResize = Event{
		Name:     ResizeEvent,
		Code:     TerminalResizeCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} resized the terminal",
	}
	// SessionLeave is emitted when a user leaves the session.
	SessionLeave = Event{
		Name:     SessionLeaveEvent,
		Code:     SessionLeaveCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} has left the session",
	}
	// SessionEnd is emitted when a user ends the session.
	SessionEnd = Event{
		Name:     SessionEndEvent,
		Code:     SessionEndCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} has ended the session",
	}
	// SessionUpload is emitted after a session recording has been uploaded.
	SessionUpload = Event{
		Name:     SessionUploadEvent,
		Code:     SessionUploadCode,
		Severity: SeverityInfo,
		Message:  "Recorded session has been uploaded",
	}
	// SessionData is emitted to report session data usage.
	SessionData = Event{
		Name:     SessionDataEvent,
		Code:     SessionDataCode,
		Severity: SeverityInfo,
		Message:  "Session transmitted {{.tx}} bytes and received {{.rx}} bytes",
	}
	// Subsystem is emitted when a user requests a new subsystem.
	Subsystem = Event{
		Name:     SubsystemEvent,
		Code:     SubsystemCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} requested subsystem {{.name}}",
	}
	// SubsystemFailure is emitted when a user subsystem request fails.
	SubsystemFailure = Event{
		Name:     SubsystemEvent,
		Code:     SubsystemFailureCode,
		Severity: SeverityError,
		Message:  "User {{.user}} subsystem {{.name}} request failed: {{.exitError}}",
	}
	// Exec is emitted when a user executes a command on a node.
	Exec = Event{
		Name:     ExecEvent,
		Code:     ExecCode,
		Severity: SeverityInfo,
		Message:  `User {{.user}} executed a command on node {{index . "addr.remote"}}`,
	}
	// ExecFailure is emitted when a user command execution fails.
	ExecFailure = Event{
		Name:     ExecEvent,
		Code:     ExecFailureCode,
		Severity: SeverityError,
		Message:  `User {{.user}} command execution on node {{index . "addr.remote"}} failed: {{.exitError}}`,
	}
	// PortForward is emitted when a user requests port forwarding.
	PortForward = Event{
		Name:     PortForwardEvent,
		Code:     PortForwardCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} started port forwarding",
	}
	// PortForwardFailure is emitted when a port forward request fails.
	PortForwardFailure = Event{
		Name:     PortForwardEvent,
		Code:     PortForwardFailureCode,
		Severity: SeverityError,
		Message:  "User {{.user}} port forwarding request failed: {{.error}}",
	}
	// SCPDownload is emitted when a user downloads a file.
	SCPDownload = Event{
		Name:     SCPEvent,
		Code:     SCPDownloadCode,
		Severity: SeverityInfo,
		Message:  `User {{.user}} downloaded a file from node {{index . "addr.remote"}}`,
	}
	// SCPDownloadFailure is emitted when a file download fails.
	SCPDownloadFailure = Event{
		Name:     SCPEvent,
		Code:     SCPDownloadFailureCode,
		Severity: SeverityError,
		Message:  `User {{.user}} file download attempt from node {{index . "addr.remote"}} failed: {{.exitError}}`,
	}
	// SCPUpload is emitted when a user uploads a file.
	SCPUpload = Event{
		Name:     SCPEvent,
		Code:     SCPUploadCode,
		Severity: SeverityInfo,
		Message:  `User {{.user}} uploaded a file to node {{index . "addr.remote"}}`,
	}
	// SCPUploadFailure is emitted when a file upload fails.
	SCPUploadFailure = Event{
		Name:     SCPEvent,
		Code:     SCPUploadFailureCode,
		Severity: SeverityError,
		Message:  `User {{.user}} file upload attempt to node {{index . "addr.remote"}} failed: {{.exitError}}`,
	}
	// ClientDisconnect is emitted when a user session is disconnected.
	ClientDisconnect = Event{
		Name:     ClientDisconnectEvent,
		Code:     ClientDisconnectCode,
		Severity: SeverityInfo,
		Message:  "User {{.user}} has been disconnected: {{.reason}}",
	}
	// AuthAttemptFailure is emitted upon a failed authentication attempt.
	AuthAttemptFailure = Event{
		Name:     AuthAttemptEvent,
		Code:     AuthAttemptFailureCode,
		Severity: SeverityWarning,
		Message:  "User {{.user}} failed auth attempt: {{.error}}",
	}
)

var (
	// UserLocalLoginCode is the successful local user login event code.
	UserLocalLoginCode = "T1000I"
	// UserLocalLoginFailureCode is the unsuccessful local user login event code.
	UserLocalLoginFailureCode = "T1000W"
	// UserSSOLoginCode is the successful SSO user login event code.
	UserSSOLoginCode = "T1001I"
	// UserSSOLoginFailureCode is the unsuccessful SSO user login event code.
	UserSSOLoginFailureCode = "T1001W"
	// UserUpdateCode is the user update event code.
	UserUpdateCode = "T1002I"
	// UserDeleteCode is the user delete event code.
	UserDeleteCode = "T1003I"
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
)
