/*
Copyright 2020 Gravitational, Inc.

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

// Package constants defines Teleport-specific constants
package constants

const (
	// DefaultRole is the name of the default admin role for all local users if
	// another role is not explicitly assigned (Enterprise only).
	AdminRoleName = "admin"

	// DefaultImplicitRole is implicit role that gets added to all service.RoleSet
	// objects.
	DefaultImplicitRole = "default-implicit-role"

	// APIDomain is a default domain name for Auth server API
	APIDomain = "teleport.cluster.local"
)

const (
	// EnhancedRecordingMinKernel is the minimum kernel version for the enhanced
	// recording feature.
	EnhancedRecordingMinKernel = "4.18.0"

	// EnhancedRecordingCommand is a role option that implies command events are
	// captured.
	EnhancedRecordingCommand = "command"

	// EnhancedRecordingDisk is a role option that implies disk events are captured.
	EnhancedRecordingDisk = "disk"

	// EnhancedRecordingNetwork is a role option that implies network events
	// are captured.
	EnhancedRecordingNetwork = "network"
)

const (
	// ConnectorOIDC means connector type OIDC
	ConnectorOIDC = "oidc"

	// ConnectorSAML means connector type SAML
	ConnectorSAML = "saml"

	// ConnectorGithub means connector type Github
	ConnectorGithub = "github"

	// OTP means One-time Password Algorithm for Two-Factor Authentication.
	OTP = "otp"

	// U2F means Universal 2nd Factor.for Two-Factor Authentication.
	U2F = "u2f"

	// OFF means no second factor.for Two-Factor Authentication.
	OFF = "off"

	// Local means authentication will happen locally within the Teleport cluster.
	Local = "local"

	// OIDC means authentication will happen remotely using an OIDC connector.
	OIDC = ConnectorOIDC

	// SAML means authentication will happen remotely using a SAML connector.
	SAML = ConnectorSAML

	// Github means authentication will happen remotely using a Github connector.
	Github = ConnectorGithub

	// JSON means JSON serialization format
	JSON = "json"

	// SchemeFile is a local disk file storage
	SchemeFile = "file"

	// HumanDateFormatSeconds is a human readable date formatting with seconds
	HumanDateFormatSeconds = "Jan _2 15:04:05 UTC"
)

const (
	// MaxLeases serves as an identifying error string indicating that the
	// semaphore system is rejecting an acquisition attempt due to max
	// leases having already been reached.
	MaxLeases = "err-max-leases"
)

const (
	// ADFS is Microsoft Active Directory Federation Services
	ADFS = "adfs"
)

const (
	// CertificateFormatStandard is used for normal Teleport operation without any
	// compatibility modes.
	CertificateFormatStandard = "standard"
)

const (
	// DurationNever is human friendly shortcut that is interpreted as a Duration of 0
	DurationNever = "never"
)

const (
	// OIDCPromptSelectAccount instructs the Authorization Server to
	// prompt the End-User to select a user account.
	OIDCPromptSelectAccount = "select_account"
)

const (
	// KeepAliveNode is the keep alive type for SSH servers.
	KeepAliveNode = "node"
	// KeepAliveApp is the keep alive type for application server.
	KeepAliveApp = "app"
	// KeepAliveDatabase is the keep alive type for database server.
	KeepAliveDatabase = "db"
)

// RSAKeySize is the size of the RSA key.
const RSAKeySize = 2048
