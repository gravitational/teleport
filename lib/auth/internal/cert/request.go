/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cert

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Request contains the parameters used to issue user and OpenSSH certificates.
type Request struct {
	// SSHPublicKey is a public key in SSH authorized_keys format. If set it
	// will be used as the subject public key for the returned SSH certificate.
	SSHPublicKey []byte
	// TLSPublicKey is a PEM-encoded public key in PKCS#1 or PKIX ASN.1 DER
	// form. If set it will be used as the subject public key for the returned
	// TLS certificate.
	TLSPublicKey []byte
	// SSHPublicKeyAttestationStatement is an attestation statement associated with SSHPublicKey.
	SSHPublicKeyAttestationStatement *hardwarekey.AttestationStatement
	// TLSPublicKeyAttestationStatement is an attestation statement associated with TLSPublicKey.
	TLSPublicKeyAttestationStatement *hardwarekey.AttestationStatement

	// User is a user to generate certificate for.
	User services.UserState
	// Impersonator is a user who generates the certificate,
	// is set when different from the user in the certificate.
	Impersonator string

	// CheckerContext is an access checker context that may either be scoped or
	// unscoped. For unscoped identities, this wraps a classic access checker and
	// cert parameters are determined by role policies. For scoped identities,
	// this wraps a scoped access checker context and cert parameters must be
	// determined by configuration/defaults since we cannot know which role will
	// apply without knowing the target resource.
	CheckerContext *services.ScopedAccessCheckerContext

	// TTL is duration of the certificate.
	TTL time.Duration
	// Compatibility is compatibility mode.
	Compatibility string
	// OverrideRoleTTL is used for requests when the requested TTL should not be
	// adjusted based off the role of the user. This is used by tctl to allow
	// creating long lived user certs.
	OverrideRoleTTL bool
	// Usage is a list of acceptable usages to be encoded in X509 certificate,
	// is used to limit ways the certificate can be used, for example
	// the cert can be only used against kubernetes endpoint, and not auth endpoint,
	// no Usage means unrestricted (to keep backwards compatibility).
	Usage []string
	// RouteToCluster is an optional teleport cluster name to route the
	// certificate requests to, this teleport cluster name will be used to
	// route the requests to in case of kubernetes.
	RouteToCluster string
	// KubernetesCluster specifies the target kubernetes cluster for TLS
	// identities. This can be empty on older Teleport clients.
	KubernetesCluster string
	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits
	// ActiveRequests tracks privilege escalation requests applied
	// during the construction of the certificate.
	ActiveRequests []string
	// AppSessionID is the session ID of the application session.
	AppSessionID string
	// AppPublicAddr is the public address of the application.
	AppPublicAddr string
	// AppClusterName is the name of the cluster this application is in.
	AppClusterName string
	// AppName is the name of the application to generate cert for.
	AppName string
	// AppURI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	AppURI string
	// AppTargetPort signifies that the cert should grant access to a specific port in a multi-port
	// TCP app, as long as the port is defined in the app spec. Used only for routing, should not be
	// used in other contexts (e.g., access requests).
	AppTargetPort int
	// AWSRoleARN is the role ARN to generate certificate for.
	AWSRoleARN string
	// AzureIdentity is the Azure identity to generate certificate for.
	AzureIdentity string
	// GCPServiceAccount is the GCP service account to generate certificate for.
	GCPServiceAccount string
	// DBService identifies the name of the database service requests will
	// be routed to.
	DBService string
	// DBProtocol specifies the protocol of the database a certificate will
	// be issued for.
	DBProtocol string
	// DBUser is the optional database user which, if provided, will be used
	// as a default username.
	DBUser string
	// DBName is the optional database name which, if provided, will be used
	// as a default database.
	DBName string
	// DBRoles is the optional list of database roles which, if provided, will
	// be used instead of all database roles granted for the target database.
	DBRoles []string
	// MFAVerified is the UUID of an MFA device when this Request was
	// created immediately after an MFA check.
	MFAVerified string
	// PreviousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpires time.Time
	// LoginIP is an IP of the client requesting the certificate.
	LoginIP string
	// PinIP flags that client's login IP should be pinned in the certificate.
	PinIP bool
	// DisallowReissue flags that a cert should not be allowed to issue future
	// certificates.
	DisallowReissue bool
	// Renewable indicates that the certificate can be renewed,
	// having its TTL increased.
	Renewable bool
	// IncludeHostCA indicates that host CA certs should be included in the
	// returned certs.
	IncludeHostCA bool
	// Generation indicates the number of times this certificate has been
	// renewed.
	Generation uint64
	// ConnectionDiagnosticID contains the ID of the ConnectionDiagnostic.
	// The Node/Agent will append connection traces to this instance.
	ConnectionDiagnosticID string
	// DeviceExtensions holds device-aware user certificate extensions.
	DeviceExtensions tlsca.DeviceExtensions
	// BotName is the name of the bot requesting this cert, if any.
	BotName string
	// BotInstanceID is the unique identifier of the bot instance associated
	// with this cert, if any.
	BotInstanceID string
	// BotInternal is a flag that indicates an identity is specifically a bot
	// internal identity, rather than output certificates intended for direct
	// consumption by users or user-facing bot services.
	BotInternal bool
	// JoinToken is the name of the join token used to join, set only for bot
	// identities. It is unset for token-joined bots, whose token names are
	// secret values.
	JoinToken string
	// JoinAttributes holds attributes derived from attested metadata from the
	// join process, should any exist.
	JoinAttributes *workloadidentityv1pb.JoinAttrs
	// RequesterName is the name of the service that sent the request.
	RequesterName proto.UserCertsRequest_Requester
	// WebSessionID is the session ID of the web session.
	// When the certificate is generated for access graph usage, we store the
	// web session ID in the cert request to be able to link the certificate to
	// a valid web session so that we can properly report access graph usage
	// and reuse the same handlers.
	WebSessionID string
}

// Check verifies the cert request is valid.
func (r *Request) Check() error {
	if r.User == nil {
		return trace.BadParameter("missing parameter user")
	}
	if r.CheckerContext == nil {
		return trace.BadParameter("missing parameter checkerContext")
	}

	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if r.DBProtocol == defaults.ProtocolMongoDB {
		if r.DBUser == "" {
			return trace.BadParameter("must provide database user name to generate certificate for database %q", r.DBService)
		}
	}

	if r.SSHPublicKey == nil && r.TLSPublicKey == nil {
		return trace.BadParameter("must provide a public key")
	}

	return nil
}
