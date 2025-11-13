/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package internal

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type CertRequest struct {
	// SSHPublicKey is a public key in SSH authorized_keys format. If set it
	// will be used as the subject public key for the returned SSH certificate.
	SSHPublicKey []byte
	// TLSPublicKey is a PEM-encoded public key in PKCS#1 or PKIX ASN.1 DER
	// form. If set it will be used as the subject public key for the returned
	// TLS certificate.
	TLSPublicKey []byte
	// SSHPublicKeyAttestationStatement is an attestation statement associated with sshPublicKey.
	SSHPublicKeyAttestationStatement *hardwarekey.AttestationStatement
	// TLSPublicKeyAttestationStatement is an attestation statement associated with tlsPublicKey.
	TLSPublicKeyAttestationStatement *hardwarekey.AttestationStatement

	// User is a User to generate certificate for
	User services.UserState
	// Impersonator is a user who generates the certificate,
	// is set when different from the user in the certificate
	Impersonator string

	// Checker is an access Checker that may either be scoped or unscoped. used to generate various
	// certificate parameters, some of which differ depending on whether the cert being generated
	// is scoped or not.
	Checker *services.SplitAccessChecker

	// TTL is Duration of the certificate
	TTL time.Duration
	// Compatibility is Compatibility mode
	Compatibility string
	// OverrideRoleTTL is used for requests when the requested TTL should not be
	// adjusted based off the role of the user. This is used by tctl to allow
	// creating long lived user certs.
	OverrideRoleTTL bool
	// Usage is a list of acceptable usages to be encoded in X509 certificate,
	// is used to limit ways the certificate can be used, for example
	// the cert can be only used against kubernetes endpoint, and not auth endpoint,
	// no Usage means unrestricted (to keep backwards compatibility)
	Usage []string
	// RouteToCluster is an optional teleport cluster name to route the
	// certificate requests to, this teleport cluster name will be used to
	// route the requests to in case of kubernetes
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
	// MFAVerified is the UUID of an MFA device when this certRequest was
	// created immediately after an MFA check.
	MFAVerified string
	// PreviousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpires time.Time
	// LoginIP is an IP of the client requesting the certificate.
	LoginIP string
	// PinIP flags that client's login IP should be pinned in the certificate
	PinIP bool
	// DisallowReissue flags that a cert should not be allowed to issue future
	// certificates.
	DisallowReissue bool
	// Renewable indicates that the certificate can be renewed,
	// having its TTL increased
	Renewable bool
	// IncludeHostCA indicates that host CA certs should be included in the
	// returned certs
	IncludeHostCA bool
	// Generation indicates the number of times this certificate has been
	// renewed.
	Generation uint64
	// ConnectionDiagnosticID contains the ID of the ConnectionDiagnostic.
	// The Node/Agent will append connection traces to this instance.
	ConnectionDiagnosticID string
	// DeviceExtensions holds device-aware user certificate extensions.
	DeviceExtensions tlsca.DeviceExtensions
	// BotName is the name of the bot requesting this cert, if any
	BotName string
	// BotInstanceID is the unique identifier of the bot instance associated
	// with this cert, if any
	BotInstanceID string
	// JoinToken is the name of the join token used to join, set only for bot
	// identities. It is unset for token-joined bots, whose token names are
	// secret values.
	JoinToken string
	// JoinAttributes holds attributes derived from attested metadata from the
	// join process, should any exist.
	JoinAttributes *workloadidentityv1pb.JoinAttrs
	// RequesterName is the name of the service that sent the request.
	RequesterName proto.UserCertsRequest_Requester
}

// Check verifies the cert request is valid.
func (r *CertRequest) Check() error {
	if r.User == nil {
		return trace.BadParameter("missing parameter user")
	}
	if r.Checker == nil {
		return trace.BadParameter("missing parameter checker")
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

// NewWebSessionRequest defines a request to create a new user
// web session
type NewWebSessionRequest struct {
	// User specifies the user this session is bound to
	User string
	// LoginIP is an observed IP of the client, it will be embedded into certificates.
	LoginIP string
	// LoginUserAgent is the user agent of the client's browser, as captured by
	// the Proxy.
	LoginUserAgent string
	// Roles optionally lists additional user roles
	Roles []string
	// Traits optionally lists role traits
	Traits map[string][]string
	// SessionTTL optionally specifies the session time-to-live.
	// If left unspecified, the default certificate duration is used.
	SessionTTL time.Duration
	// LoginTime is the time that this user recently logged in.
	LoginTime time.Time
	// AccessRequests contains the UUIDs of the access requests currently in use.
	AccessRequests []string
	// RequestedResourceIDs optionally lists requested resources
	RequestedResourceIDs []types.ResourceID
	// AttestWebSession optionally attests the web session to meet private key policy requirements.
	// This should only be set to true for web sessions that are purely in the purview of the Proxy
	// and Auth services. Users should never have direct access to attested web sessions.
	AttestWebSession bool
	// SSHPrivateKey is a specific private key to use when generating the web
	// sessions' SSH certificates.
	// This should be provided when extending an attested web session in order
	// to maintain the session attested status.
	SSHPrivateKey *keys.PrivateKey
	// TLSPrivateKey is a specific private key to use when generating the web
	// sessions' SSH certificates.
	// This should be provided when extending an attested web session in order
	// to maintain the session attested status.
	TLSPrivateKey *keys.PrivateKey
	// CreateDeviceWebToken informs Auth to issue a DeviceWebToken when creating
	// this session.
	// A DeviceWebToken must only be issued for users that have been authenticated
	// in the same RPC.
	// May only be set internally by Auth (and Auth-related logic), not allowed
	// for external requests.
	CreateDeviceWebToken bool
}

// CheckAndSetDefaults validates the request and sets defaults.
func (r *NewWebSessionRequest) CheckAndSetDefaults() error {
	if r.User == "" {
		return trace.BadParameter("user name required")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("roles required")
	}
	if len(r.Traits) == 0 {
		return trace.BadParameter("traits required")
	}
	if r.SessionTTL == 0 {
		r.SessionTTL = apidefaults.CertDuration
	}
	return nil
}

// NewAppSessionRequest defines a request to create a new user app session.
type NewAppSessionRequest struct {
	NewWebSessionRequest

	// PublicAddr is the public address the application.
	PublicAddr string
	// ClusterName is cluster within which the application is running.
	ClusterName string
	// AWSRoleARN is AWS role the user wants to assume.
	AWSRoleARN string
	// AzureIdentity is Azure identity the user wants to assume.
	AzureIdentity string
	// GCPServiceAccount is the GCP service account the user wants to assume.
	GCPServiceAccount string
	// MFAVerified is the UUID of an MFA device used to verify this request.
	MFAVerified string
	// DeviceExtensions holds device-aware user certificate extensions.
	DeviceExtensions tlsca.DeviceExtensions
	// AppName is the name of the app.
	AppName string
	// AppURI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	AppURI string
	// AppTargetPort signifies that the session is made to a specific port of a multi-port TCP app.
	AppTargetPort int
	// Identity is the identity of the user.
	Identity tlsca.Identity
	// ClientAddr is a client (user's) address.
	ClientAddr string

	// BotName is the name of the bot that is creating this session.
	// Empty if not a bot.
	BotName string
	// BotInstanceID is the ID of the bot instance that is creating this session.
	// Empty if not a bot.
	BotInstanceID string
}
