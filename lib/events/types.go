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

import "github.com/gravitational/teleport/api/types/events"

// The following types, functions, and constants have been moved to /api/types, and are now imported here
// for backwards compatibility. These can be removed in a future version.
// DELETE IN 7.0.0

// Imported from api.go
type (
	ProtoMarshaler = events.ProtoMarshaler
	AuditEvent     = events.AuditEvent
	Emitter        = events.Emitter
	Stream         = events.Stream
)

// Imported from struct.go
type (
	Struct = events.Struct
)

var (
	EncodeMap        = events.EncodeMap
	EncodeMapStrings = events.EncodeMapStrings
	MustEncodeMap    = events.MustEncodeMap
)

// Imported from oneof.go
var (
	MustToOneOf = events.MustToOneOf
	ToOneOf     = events.ToOneOf
	FromOneOf   = events.FromOneOf
)

// Imported from events.pb.go
type (
	Metadata                        = events.Metadata
	SessionMetadata                 = events.SessionMetadata
	UserMetadata                    = events.UserMetadata
	ServerMetadata                  = events.ServerMetadata
	ConnectionMetadata              = events.ConnectionMetadata
	KubernetesClusterMetadata       = events.KubernetesClusterMetadata
	KubernetesPodMetadata           = events.KubernetesPodMetadata
	SessionStart                    = events.SessionStart
	SessionJoin                     = events.SessionJoin
	SessionPrint                    = events.SessionPrint
	SessionReject                   = events.SessionReject
	Resize                          = events.Resize
	SessionEnd                      = events.SessionEnd
	BPFMetadata                     = events.BPFMetadata
	Status                          = events.Status
	SessionCommand                  = events.SessionCommand
	SessionDisk                     = events.SessionDisk
	SessionNetwork                  = events.SessionNetwork
	SessionData                     = events.SessionData
	SessionLeave                    = events.SessionLeave
	UserLogin                       = events.UserLogin
	ResourceMetadata                = events.ResourceMetadata
	UserCreate                      = events.UserCreate
	UserDelete                      = events.UserDelete
	UserPasswordChange              = events.UserPasswordChange
	AccessRequestCreate             = events.AccessRequestCreate
	PortForward                     = events.PortForward
	X11Forward                      = events.X11Forward
	CommandMetadata                 = events.CommandMetadata
	Exec                            = events.Exec
	SCP                             = events.SCP
	Subsystem                       = events.Subsystem
	ClientDisconnect                = events.ClientDisconnect
	AuthAttempt                     = events.AuthAttempt
	ResetPasswordTokenCreate        = events.ResetPasswordTokenCreate
	RoleCreate                      = events.RoleCreate
	RoleDelete                      = events.RoleDelete
	TrustedClusterCreate            = events.TrustedClusterCreate
	TrustedClusterDelete            = events.TrustedClusterDelete
	TrustedClusterTokenCreate       = events.TrustedClusterTokenCreate
	GithubConnectorCreate           = events.GithubConnectorCreate
	GithubConnectorDelete           = events.GithubConnectorDelete
	OIDCConnectorCreate             = events.OIDCConnectorCreate
	OIDCConnectorDelete             = events.OIDCConnectorDelete
	SAMLConnectorCreate             = events.SAMLConnectorCreate
	SAMLConnectorDelete             = events.SAMLConnectorDelete
	KubeRequest                     = events.KubeRequest
	AppSessionStart                 = events.AppSessionStart
	AppSessionChunk                 = events.AppSessionChunk
	AppSessionRequest               = events.AppSessionRequest
	BillingInformationUpdate        = events.BillingInformationUpdate
	BillingCardCreate               = events.BillingCardCreate
	BillingCardDelete               = events.BillingCardDelete
	OneOf                           = events.OneOf
	OneOf_UserLogin                 = events.OneOf_UserLogin                 //nolint
	OneOf_UserCreate                = events.OneOf_UserCreate                //nolint
	OneOf_UserDelete                = events.OneOf_UserDelete                //nolint
	OneOf_UserPasswordChange        = events.OneOf_UserPasswordChange        //nolint
	OneOf_SessionStart              = events.OneOf_SessionStart              //nolint
	OneOf_SessionJoin               = events.OneOf_SessionJoin               //nolint
	OneOf_SessionPrint              = events.OneOf_SessionPrint              //nolint
	OneOf_SessionReject             = events.OneOf_SessionReject             //nolint
	OneOf_Resize                    = events.OneOf_Resize                    //nolint
	OneOf_SessionEnd                = events.OneOf_SessionEnd                //nolint
	OneOf_SessionCommand            = events.OneOf_SessionCommand            //nolint
	OneOf_SessionDisk               = events.OneOf_SessionDisk               //nolint
	OneOf_SessionNetwork            = events.OneOf_SessionNetwork            //nolint
	OneOf_SessionData               = events.OneOf_SessionData               //nolint
	OneOf_SessionLeave              = events.OneOf_SessionLeave              //nolint
	OneOf_PortForward               = events.OneOf_PortForward               //nolint
	OneOf_X11Forward                = events.OneOf_X11Forward                //nolint
	OneOf_SCP                       = events.OneOf_SCP                       //nolint
	OneOf_Exec                      = events.OneOf_Exec                      //nolint
	OneOf_Subsystem                 = events.OneOf_Subsystem                 //nolint
	OneOf_ClientDisconnect          = events.OneOf_ClientDisconnect          //nolint
	OneOf_AuthAttempt               = events.OneOf_AuthAttempt               //nolint
	OneOf_AccessRequestCreate       = events.OneOf_AccessRequestCreate       //nolint
	OneOf_ResetPasswordTokenCreate  = events.OneOf_ResetPasswordTokenCreate  //nolint
	OneOf_RoleCreate                = events.OneOf_RoleCreate                //nolint
	OneOf_RoleDelete                = events.OneOf_RoleDelete                //nolint
	OneOf_TrustedClusterCreate      = events.OneOf_TrustedClusterCreate      //nolint
	OneOf_TrustedClusterDelete      = events.OneOf_TrustedClusterDelete      //nolint
	OneOf_TrustedClusterTokenCreate = events.OneOf_TrustedClusterTokenCreate //nolint
	OneOf_GithubConnectorCreate     = events.OneOf_GithubConnectorCreate     //nolint
	OneOf_GithubConnectorDelete     = events.OneOf_GithubConnectorDelete     //nolint
	OneOf_OIDCConnectorCreate       = events.OneOf_OIDCConnectorCreate       //nolint
	OneOf_OIDCConnectorDelete       = events.OneOf_OIDCConnectorDelete       //nolint
	OneOf_SAMLConnectorCreate       = events.OneOf_SAMLConnectorCreate       //nolint
	OneOf_SAMLConnectorDelete       = events.OneOf_SAMLConnectorDelete       //nolint
	OneOf_KubeRequest               = events.OneOf_KubeRequest               //nolint
	OneOf_AppSessionStart           = events.OneOf_AppSessionStart           //nolint
	OneOf_AppSessionChunk           = events.OneOf_AppSessionChunk           //nolint
	OneOf_AppSessionRequest         = events.OneOf_AppSessionRequest         //nolint
	StreamStatus                    = events.StreamStatus
)
