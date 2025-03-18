/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package tlsca

import (
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
)

var log = logrus.WithFields(logrus.Fields{
	teleport.ComponentKey: teleport.ComponentAuthority,
})

// FromCertAndSigner returns a CertAuthority with the given raw certificate and signer.
func FromCertAndSigner(certPEM []byte, signer crypto.Signer) (*CertAuthority, error) {
	cert, err := ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CertAuthority{
		Cert:   cert,
		Signer: signer,
	}, nil
}

// FromKeys returns new CA from PEM encoded certificate and private
// key. Private Key is optional, if omitted CA won't be able to
// issue new certificates, only verify them
func FromKeys(certPEM, keyPEM []byte) (*CertAuthority, error) {
	ca := &CertAuthority{}
	var err error
	ca.Cert, err = ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(keyPEM) != 0 {
		ca.Signer, err = keys.ParsePrivateKey(keyPEM)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return ca, nil
}

// FromTLSCertificate returns a CertAuthority with the given TLS certificate.
func FromTLSCertificate(ca tls.Certificate) (*CertAuthority, error) {
	if len(ca.Certificate) == 0 {
		return nil, trace.BadParameter("invalid certificate length")
	}
	cert, err := x509.ParseCertificate(ca.Certificate[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, ok := ca.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("failed to convert private key to signer")
	}

	return &CertAuthority{
		Cert:   cert,
		Signer: signer,
	}, nil
}

// CertAuthority is X.509 certificate authority
type CertAuthority struct {
	// Cert is a CA certificate
	Cert *x509.Certificate
	// Signer is a private key based signer
	Signer crypto.Signer
}

// Identity is an identity of the user or service, e.g. Proxy or Node
// Must be kept in sync with teleport.decision.v1alpha1.TLSIdentity.
type Identity struct {
	// Username is the name of the user (for end-users/bots) or the Host ID (for
	// Teleport processes).
	Username string
	// Impersonator is a username of a user impersonating this user
	Impersonator string
	// Groups is a list of groups (Teleport roles) encoded in the identity
	Groups []string
	// SystemRoles is a list of system roles (e.g. auth, proxy, node, etc) used
	// in "multi-role" certificates. Single-role certificates encode the system role
	// in `Groups` for back-compat reasons.
	SystemRoles []string
	// Usage is a list of usage restrictions encoded in the identity
	Usage []string
	// Principals is a list of Unix logins allowed.
	Principals []string
	// KubernetesGroups is a list of Kubernetes groups allowed
	KubernetesGroups []string
	// KubernetesUsers is a list of Kubernetes users allowed
	KubernetesUsers []string
	// Expires specifies whenever the session will expire
	Expires time.Time
	// RouteToCluster specifies the target cluster
	// if present in the session
	RouteToCluster string
	// KubernetesCluster specifies the target kubernetes cluster for TLS
	// identities. This can be empty on older Teleport clients.
	KubernetesCluster string
	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits
	// RouteToApp holds routing information for applications. Routing metadata
	// allows Teleport web proxy to route HTTP requests to the appropriate
	// cluster and Teleport application proxy within the cluster.
	RouteToApp RouteToApp
	// TeleportCluster is the name of the teleport cluster that this identity
	// originated from. For TLS certs this may not be the same as cert issuer,
	// in case of multi-hop requests that originate from a remote cluster.
	TeleportCluster string
	// RouteToDatabase contains routing information for databases.
	RouteToDatabase RouteToDatabase
	// DatabaseNames is a list of allowed database names.
	DatabaseNames []string
	// DatabaseUsers is a list of allowed database users.
	DatabaseUsers []string
	// MFAVerified is the UUID of an MFA device when this Identity was
	// confirmed immediately after an MFA check.
	MFAVerified string
	// PreviousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpires time.Time
	// LoginIP is an observed IP of the client that this Identity represents.
	LoginIP string
	// PinnedIP is an IP the certificate is pinned to.
	PinnedIP string
	// AWSRoleARNs is a list of allowed AWS role ARNs user can assume.
	AWSRoleARNs []string
	// AzureIdentities is a list of allowed Azure identities user can assume.
	AzureIdentities []string
	// GCPServiceAccounts is a list of allowed GCP service accounts that the user can assume.
	GCPServiceAccounts []string
	// ActiveRequests is a list of UUIDs of active requests for this Identity.
	ActiveRequests []string
	// DisallowReissue is a flag that, if set, instructs the auth server to
	// deny any attempts to reissue new certificates while authenticated with
	// this certificate.
	DisallowReissue bool
	// Renewable indicates that this identity is allowed to renew it's
	// own credentials. This is only enabled for certificate renewal bots.
	Renewable bool
	// Generation counts the number of times this certificate has been renewed.
	Generation uint64
	// BotName indicates the name of the Machine ID bot this identity was issued
	// to, if any.
	BotName string
	// BotInstanceID is a unique identifier for Machine ID bots that is
	// persisted through renewals.
	BotInstanceID string
	// AllowedResourceIDs lists the resources the identity should be allowed to
	// access.
	AllowedResourceIDs []types.ResourceID
	// PrivateKeyPolicy is the private key policy supported by this identity.
	PrivateKeyPolicy keys.PrivateKeyPolicy

	// ConnectionDiagnosticID is used to add connection diagnostic messages when Testing a Connection.
	ConnectionDiagnosticID string

	// DeviceExtensions holds device-aware extensions for the identity.
	DeviceExtensions DeviceExtensions

	// UserType indicates if the User was created by an SSO Provider or locally.
	UserType types.UserType

	// JoinAttributes holds the attributes that resulted from the
	// Bot/Agent join process.
	JoinAttributes *workloadidentityv1pb.JoinAttrs
}

// RouteToApp holds routing information for applications.
type RouteToApp struct {
	// SessionID is a UUIDv4 used to identify application sessions created by
	// this certificate. The reason a UUID was used instead of a hash of the
	// SubjectPublicKeyInfo like the CA pin is for UX consistency. For example,
	// the SessionID is emitted in the audit log, using a UUID matches how SSH
	// sessions are identified.
	SessionID string

	// PublicAddr (and ClusterName) are used to route requests issued with this
	// certificate to the appropriate application proxy/cluster.
	PublicAddr string

	// ClusterName (and PublicAddr) are used to route requests issued with this
	// certificate to the appropriate application proxy/cluster.
	ClusterName string

	// Name is the app name.
	Name string

	// AWSRoleARN is the AWS role to assume when accessing AWS console.
	AWSRoleARN string

	// AzureIdentity is the Azure identity to assume when accessing Azure API.
	AzureIdentity string

	// GCPServiceAccount is the GCP service account to assume when accessing GCP API.
	GCPServiceAccount string

	// URI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	URI string
}

// RouteToDatabase contains routing information for databases.
type RouteToDatabase struct {
	// ServiceName is the name of the Teleport database proxy service
	// to route requests to.
	ServiceName string
	// Protocol is the database protocol.
	//
	// It is embedded in identity so clients can understand what type
	// of database this is without contacting server.
	Protocol string
	// Username is an optional database username to serve as a default
	// username to connect as.
	Username string
	// Database is an optional database name to serve as a default
	// database to connect to.
	Database string
	// Roles is an optional list of database roles to use for a database
	// session.
	// This list should be a subset of allowed database roles. If not
	// specified, Database Service will use all allowed database roles for this
	// database.
	Roles []string
}

// String returns string representation of the database routing struct.
func (r RouteToDatabase) String() string {
	return fmt.Sprintf("Database(Service=%v, Protocol=%v, Username=%v, Database=%v, Roles=%v)",
		r.ServiceName, r.Protocol, r.Username, r.Database, r.Roles)
}

// Empty returns true if RouteToDatabase is empty.
func (r RouteToDatabase) Empty() bool {
	return r.ServiceName == "" &&
		r.Protocol == "" &&
		r.Username == "" &&
		r.Database == "" &&
		len(r.Roles) == 0
}

// DeviceExtensions holds device-aware extensions for the identity.
type DeviceExtensions struct {
	// DeviceID is the trusted device identifier.
	DeviceID string
	// AssetTag is the device inventory identifier.
	AssetTag string
	// CredentialID is the identifier for the credential used by the device to
	// authenticate itself.
	CredentialID string
}

// GetRouteToApp returns application routing data. If missing, returns an error.
func (id *Identity) GetRouteToApp() (RouteToApp, error) {
	if id.RouteToApp.SessionID == "" ||
		id.RouteToApp.PublicAddr == "" ||
		id.RouteToApp.ClusterName == "" {
		return RouteToApp{}, trace.BadParameter("identity is missing application routing metadata")
	}

	return id.RouteToApp, nil
}

func (id *Identity) GetEventIdentity() events.Identity {
	// leave a nil instead of a zero struct so the field doesn't appear when
	// serialized as json
	var routeToApp *events.RouteToApp
	if id.RouteToApp != (RouteToApp{}) {
		routeToApp = &events.RouteToApp{
			Name:              id.RouteToApp.Name,
			SessionID:         id.RouteToApp.SessionID,
			PublicAddr:        id.RouteToApp.PublicAddr,
			ClusterName:       id.RouteToApp.ClusterName,
			AWSRoleARN:        id.RouteToApp.AWSRoleARN,
			AzureIdentity:     id.RouteToApp.AzureIdentity,
			GCPServiceAccount: id.RouteToApp.GCPServiceAccount,
			URI:               id.RouteToApp.URI,
		}
	}
	var routeToDatabase *events.RouteToDatabase
	if !id.RouteToDatabase.Empty() {
		routeToDatabase = &events.RouteToDatabase{
			ServiceName: id.RouteToDatabase.ServiceName,
			Protocol:    id.RouteToDatabase.Protocol,
			Username:    id.RouteToDatabase.Username,
			Database:    id.RouteToDatabase.Database,
			Roles:       id.RouteToDatabase.Roles,
		}
	}

	var devExts *events.DeviceExtensions
	if id.DeviceExtensions != (DeviceExtensions{}) {
		devExts = &events.DeviceExtensions{
			DeviceId:     id.DeviceExtensions.DeviceID,
			AssetTag:     id.DeviceExtensions.AssetTag,
			CredentialId: id.DeviceExtensions.CredentialID,
		}
	}

	return events.Identity{
		User:                    id.Username,
		Impersonator:            id.Impersonator,
		Roles:                   id.Groups,
		Usage:                   id.Usage,
		Logins:                  id.Principals,
		KubernetesGroups:        id.KubernetesGroups,
		KubernetesUsers:         id.KubernetesUsers,
		Expires:                 id.Expires,
		RouteToCluster:          id.RouteToCluster,
		KubernetesCluster:       id.KubernetesCluster,
		Traits:                  id.Traits,
		RouteToApp:              routeToApp,
		TeleportCluster:         id.TeleportCluster,
		RouteToDatabase:         routeToDatabase,
		DatabaseNames:           id.DatabaseNames,
		DatabaseUsers:           id.DatabaseUsers,
		MFADeviceUUID:           id.MFAVerified,
		PreviousIdentityExpires: id.PreviousIdentityExpires,
		ClientIP:                id.LoginIP,
		AWSRoleARNs:             id.AWSRoleARNs,
		AzureIdentities:         id.AzureIdentities,
		GCPServiceAccounts:      id.GCPServiceAccounts,
		AccessRequests:          id.ActiveRequests,
		DisallowReissue:         id.DisallowReissue,
		AllowedResourceIDs:      events.ResourceIDs(id.AllowedResourceIDs),
		PrivateKeyPolicy:        string(id.PrivateKeyPolicy),
		DeviceExtensions:        devExts,
		BotName:                 id.BotName,
		BotInstanceID:           id.BotInstanceID,
	}
}

// CheckAndSetDefaults checks and sets default values
func (id *Identity) CheckAndSetDefaults() error {
	if id.Username == "" {
		return trace.BadParameter("missing identity username")
	}
	if len(id.Groups) == 0 {
		return trace.BadParameter("missing identity groups")
	}

	return nil
}

// Custom ranges are taken from this article
//
// https://serverfault.com/questions/551477/is-there-reserved-oid-space-for-internal-enterprise-cas
//
// http://oid-info.com/get/1.3.9999
var (
	// KubeUsersASN1ExtensionOID is an extension ID used when encoding/decoding
	// license payload into certificates
	KubeUsersASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 1}

	// KubeGroupsASN1ExtensionOID is an extension ID used when encoding/decoding
	// license payload into certificates
	KubeGroupsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 2}

	// KubeClusterASN1ExtensionOID is an extension ID used when encoding/decoding
	// target kubernetes cluster name into certificates.
	KubeClusterASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 3}

	// AppSessionIDASN1ExtensionOID is an extension ID used to encode the application
	// session ID into a certificate.
	AppSessionIDASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 4}

	// AppClusterNameASN1ExtensionOID is an extension ID used to encode the application
	// cluster name into a certificate.
	AppClusterNameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 5}

	// AppPublicAddrASN1ExtensionOID is an extension ID used to encode the application
	// public address into a certificate.
	AppPublicAddrASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 6}

	// TeleportClusterASN1ExtensionOID is an extension ID used when encoding/decoding
	// origin teleport cluster name into certificates.
	TeleportClusterASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 7}

	// MFAVerifiedASN1ExtensionOID is an extension ID used when encoding/decoding
	// the MFAVerified flag into certificates.
	MFAVerifiedASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 8}

	// LoginIPASN1ExtensionOID is an extension ID used when encoding/decoding
	// the client's login IP into certificates.
	LoginIPASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 9}

	// AppNameASN1ExtensionOID is an extension ID used when encoding/decoding
	// application name into a certificate.
	AppNameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 10}

	// AppAWSRoleARNASN1ExtensionOID is an extension ID used when encoding/decoding
	// AWS role ARN into a certificate.
	AppAWSRoleARNASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 11}

	// AWSRoleARNsASN1ExtensionOID is an extension ID used when encoding/decoding
	// allowed AWS role ARNs into a certificate.
	AWSRoleARNsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 12}

	// RenewableCertificateASN1ExtensionOID is an extension ID used to indicate
	// that a certificate may be renewed by a certificate renewal bot.
	RenewableCertificateASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 13}

	// GenerationASN1ExtensionOID is an extension OID used to count the number
	// of times this certificate has been renewed.
	GenerationASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 14}

	// PrivateKeyPolicyASN1ExtensionOID is an extension ID used to determine the
	// private key policy supported by the certificate.
	PrivateKeyPolicyASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 15}

	// AppAzureIdentityASN1ExtensionOID is an extension ID used when encoding/decoding
	// Azure identity into a certificate.
	AppAzureIdentityASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 16}

	// AzureIdentityASN1ExtensionOID is an extension ID used when encoding/decoding
	// allowed Azure identity into a certificate.
	AzureIdentityASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 17}

	// AppGCPServiceAccountASN1ExtensionOID is an extension ID used when encoding/decoding
	// the chosen GCP service account into a certificate.
	AppGCPServiceAccountASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 18}

	// GCPServiceAccountsASN1ExtensionOID is an extension ID used when encoding/decoding
	// the list of allowed GCP service accounts into a certificate.
	GCPServiceAccountsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 19}

	// UserTypeASN1ExtensionOID is an extension that encodes the user type.
	// Its value is either local or sso.
	UserTypeASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 20}

	// DatabaseServiceNameASN1ExtensionOID is an extension ID used when encoding/decoding
	// database service name into certificates.
	DatabaseServiceNameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 1}

	// DatabaseProtocolASN1ExtensionOID is an extension ID used when encoding/decoding
	// database protocol into certificates.
	DatabaseProtocolASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 2}

	// DatabaseUsernameASN1ExtensionOID is an extension ID used when encoding/decoding
	// database username into certificates.
	DatabaseUsernameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 3}

	// DatabaseNameASN1ExtensionOID is an extension ID used when encoding/decoding
	// database name into certificates.
	DatabaseNameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 4}

	// DatabaseNamesASN1ExtensionOID is an extension OID used when encoding/decoding
	// allowed database names into certificates.
	DatabaseNamesASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 5}

	// DatabaseUsersASN1ExtensionOID is an extension OID used when encoding/decoding
	// allowed database users into certificates.
	DatabaseUsersASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 6}

	// ImpersonatorASN1ExtensionOID is an extension OID used when encoding/decoding
	// impersonator user
	ImpersonatorASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 7}

	// ActiveRequestsASN1ExtensionOID is an extension OID used when encoding/decoding
	// active access requests into certificates.
	ActiveRequestsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 8}

	// DisallowReissueASN1ExtensionOID is an extension OID used to flag that a
	// requests to generate new certificates using this certificate should be
	// denied.
	DisallowReissueASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 9}

	// AllowedResourcesASN1ExtensionOID is an extension OID used to list the
	// resources which the certificate should be able to grant access to
	AllowedResourcesASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 10}

	// SystemRolesASN1ExtensionOID is an extension OID used to indicate system roles
	// (auth, proxy, node, etc). Note that some certs correspond to a single specific
	// system role, and use `pkix.Name.Organization` to encode this value. This extension
	// is specifically used for "multi-role" certs.
	SystemRolesASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 11}

	// PreviousIdentityExpiresASN1ExtensionOID is the RFC3339 timestamp representing the hard
	// deadline of the session on a certificates issued after an MFA check.
	// See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpiresASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 12}

	// ConnectionDiagnosticIDASN1ExtensionOID is an extension OID used to indicate the Connection Diagnostic ID.
	// When using the Test Connection feature, there's propagation of the ConnectionDiagnosticID.
	// Each service (ex DB Agent) uses that to add checkpoints describing if it was a success or a failure.
	ConnectionDiagnosticIDASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 13}

	// LicenseOID is an extension OID signaling the license type of Teleport build.
	// It should take values "oss" or "ent" (the values returned by modules.GetModules().BuildType())
	LicenseOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 14}

	// PinnedIPASN1ExtensionOID is an extension ID used when encoding/decoding
	// the IP the certificate is pinned to.
	PinnedIPASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 15}

	// CreateWindowsUserOID is an extension OID used to indicate that the user should be created.
	CreateWindowsUserOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 16}

	// DesktopsLimitExceededOID is an extension OID used indicate if number of non-AD desktops exceeds the limit for OSS distribution.
	DesktopsLimitExceededOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 17}

	// BotASN1ExtensionOID is an extension OID used to indicate an identity is associated with a Machine ID bot.
	BotASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 18}

	// RequestedDatabaseRolesExtensionOID is an extension OID used when
	// encoding/decoding requested database roles.
	RequestedDatabaseRolesExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 19}

	// BotInstanceASN1ExtensionOID is an extension that encodes a unique bot
	// instance identifier into a certificate.
	BotInstanceASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 20}

	// JoinAttributesASN1ExtensionOID is an extension that encodes the
	// attributes that resulted from the Bot/Agent join process.
	JoinAttributesASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 21}
)

// Device Trust OIDs.
// Namespace 1.3.9999.3.x.
var (
	// DeviceIDExtensionOID is a string extension that identifies the trusted
	// device.
	DeviceIDExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 3, 1}

	// DeviceAssetTagExtensionOID is a string extension containing the device
	// inventory identifier.
	DeviceAssetTagExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 3, 2}

	// DeviceCredentialIDExtensionOID is a string extension that identifies the
	// credential used to authenticate the device.
	DeviceCredentialIDExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 3, 3}
)

// Subject converts identity to X.509 subject name
func (id *Identity) Subject() (pkix.Name, error) {
	rawTraits, err := wrappers.MarshalTraits(&id.Traits)
	if err != nil {
		return pkix.Name{}, trace.Wrap(err)
	}

	subject := pkix.Name{
		CommonName:         id.Username,
		Organization:       append([]string{}, id.Groups...),
		OrganizationalUnit: append([]string{}, id.Usage...),
		Locality:           append([]string{}, id.Principals...),

		// TODO: create ASN.1 extensions for traits and RouteToCluster
		// and move away from using StreetAddress and PostalCode
		StreetAddress: []string{id.RouteToCluster},
		PostalCode:    []string{string(rawTraits)},
	}

	for i := range id.SystemRoles {
		systemRole := id.SystemRoles[i]
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  SystemRolesASN1ExtensionOID,
				Value: systemRole,
			})
	}

	for i := range id.KubernetesUsers {
		kubeUser := id.KubernetesUsers[i]
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  KubeUsersASN1ExtensionOID,
				Value: kubeUser,
			})
	}

	for i := range id.KubernetesGroups {
		kubeGroup := id.KubernetesGroups[i]
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  KubeGroupsASN1ExtensionOID,
				Value: kubeGroup,
			})
	}

	if id.KubernetesCluster != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  KubeClusterASN1ExtensionOID,
				Value: id.KubernetesCluster,
			})
	}

	// Encode application routing metadata if provided.
	if id.RouteToApp.SessionID != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppSessionIDASN1ExtensionOID,
				Value: id.RouteToApp.SessionID,
			})
	}
	if id.RouteToApp.PublicAddr != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppPublicAddrASN1ExtensionOID,
				Value: id.RouteToApp.PublicAddr,
			})
	}
	if id.RouteToApp.ClusterName != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppClusterNameASN1ExtensionOID,
				Value: id.RouteToApp.ClusterName,
			})
	}
	if id.RouteToApp.Name != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppNameASN1ExtensionOID,
				Value: id.RouteToApp.Name,
			})
	}
	if id.RouteToApp.AWSRoleARN != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppAWSRoleARNASN1ExtensionOID,
				Value: id.RouteToApp.AWSRoleARN,
			})
	}
	for i := range id.AWSRoleARNs {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AWSRoleARNsASN1ExtensionOID,
				Value: id.AWSRoleARNs[i],
			})
	}
	if id.RouteToApp.AzureIdentity != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppAzureIdentityASN1ExtensionOID,
				Value: id.RouteToApp.AzureIdentity,
			})
	}
	for i := range id.AzureIdentities {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AzureIdentityASN1ExtensionOID,
				Value: id.AzureIdentities[i],
			})
	}
	if id.RouteToApp.GCPServiceAccount != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AppGCPServiceAccountASN1ExtensionOID,
				Value: id.RouteToApp.GCPServiceAccount,
			})
	}
	for i := range id.GCPServiceAccounts {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  GCPServiceAccountsASN1ExtensionOID,
				Value: id.GCPServiceAccounts[i],
			})
	}
	if id.Renewable {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  RenewableCertificateASN1ExtensionOID,
				Value: types.True,
			})
	}
	if id.TeleportCluster != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  TeleportClusterASN1ExtensionOID,
				Value: id.TeleportCluster,
			})
	}
	if id.MFAVerified != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  MFAVerifiedASN1ExtensionOID,
				Value: id.MFAVerified,
			})
	}
	if !id.PreviousIdentityExpires.IsZero() {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  PreviousIdentityExpiresASN1ExtensionOID,
				Value: id.PreviousIdentityExpires.Format(time.RFC3339),
			})
	}
	if id.LoginIP != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  LoginIPASN1ExtensionOID,
				Value: id.LoginIP,
			})
	}
	if id.PinnedIP != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  PinnedIPASN1ExtensionOID,
				Value: id.PinnedIP,
			})
	}

	// Encode routing metadata for databases.
	if id.RouteToDatabase.ServiceName != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DatabaseServiceNameASN1ExtensionOID,
				Value: id.RouteToDatabase.ServiceName,
			})
	}
	if id.RouteToDatabase.Protocol != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DatabaseProtocolASN1ExtensionOID,
				Value: id.RouteToDatabase.Protocol,
			})
	}
	if id.RouteToDatabase.Username != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DatabaseUsernameASN1ExtensionOID,
				Value: id.RouteToDatabase.Username,
			})
	}
	if id.RouteToDatabase.Database != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DatabaseNameASN1ExtensionOID,
				Value: id.RouteToDatabase.Database,
			})
	}
	for i := range id.RouteToDatabase.Roles {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  RequestedDatabaseRolesExtensionOID,
				Value: id.RouteToDatabase.Roles[i],
			})
	}

	// Encode allowed database names/users used when passing them
	// to remote clusters as user traits.
	for i := range id.DatabaseNames {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DatabaseNamesASN1ExtensionOID,
				Value: id.DatabaseNames[i],
			})
	}
	for i := range id.DatabaseUsers {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DatabaseUsersASN1ExtensionOID,
				Value: id.DatabaseUsers[i],
			})
	}

	if id.Impersonator != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  ImpersonatorASN1ExtensionOID,
				Value: id.Impersonator,
			})
	}

	for _, activeRequest := range id.ActiveRequests {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  ActiveRequestsASN1ExtensionOID,
				Value: activeRequest,
			})
	}

	if id.DisallowReissue {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  DisallowReissueASN1ExtensionOID,
				Value: types.True,
			},
		)
	}

	if id.Generation > 0 {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  GenerationASN1ExtensionOID,
				Value: fmt.Sprint(id.Generation),
			},
		)
	}

	if id.BotName != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  BotASN1ExtensionOID,
				Value: id.BotName,
			})
	}

	if id.BotInstanceID != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  BotInstanceASN1ExtensionOID,
				Value: id.BotInstanceID,
			})
	}

	if id.UserType != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  UserTypeASN1ExtensionOID,
				Value: string(id.UserType),
			},
		)
	}

	if len(id.AllowedResourceIDs) > 0 {
		allowedResourcesStr, err := types.ResourceIDsToString(id.AllowedResourceIDs)
		if err != nil {
			return pkix.Name{}, trace.Wrap(err)
		}
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  AllowedResourcesASN1ExtensionOID,
				Value: allowedResourcesStr,
			},
		)
	}

	if id.PrivateKeyPolicy != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  PrivateKeyPolicyASN1ExtensionOID,
				Value: id.PrivateKeyPolicy,
			},
		)
	}

	if id.ConnectionDiagnosticID != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  ConnectionDiagnosticIDASN1ExtensionOID,
				Value: id.ConnectionDiagnosticID,
			},
		)
	}

	if id.JoinAttributes != nil && shouldPersistJoinAttrs() {
		encoded, err := protojson.MarshalOptions{
			// Use the proto field names as this is what we use in the
			// templating engine and this being consistent for any user who
			// inspects the cert is kind.
			UseProtoNames: true,
		}.Marshal(id.JoinAttributes)
		if err != nil {
			return pkix.Name{}, trace.Wrap(err, "encoding join attributes as protojson")
		}
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  JoinAttributesASN1ExtensionOID,
				Value: string(encoded),
			},
		)
	}

	// Device extensions.
	if devID := id.DeviceExtensions.DeviceID; devID != "" {
		subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
			Type:  DeviceIDExtensionOID,
			Value: devID,
		})
	}
	if devTag := id.DeviceExtensions.AssetTag; devTag != "" {
		subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
			Type:  DeviceAssetTagExtensionOID,
			Value: devTag,
		})
	}
	if devCred := id.DeviceExtensions.CredentialID; devCred != "" {
		subject.ExtraNames = append(subject.ExtraNames, pkix.AttributeTypeAndValue{
			Type:  DeviceCredentialIDExtensionOID,
			Value: devCred,
		})
	}

	return subject, nil
}

// FromSubject returns identity from subject name
func FromSubject(subject pkix.Name, expires time.Time) (*Identity, error) {
	id := &Identity{
		Username:   subject.CommonName,
		Groups:     subject.Organization,
		Usage:      subject.OrganizationalUnit,
		Principals: subject.Locality,
		Expires:    expires,
	}
	if len(subject.StreetAddress) > 0 {
		id.RouteToCluster = subject.StreetAddress[0]
	}
	if len(subject.PostalCode) > 0 {
		err := wrappers.UnmarshalTraits([]byte(subject.PostalCode[0]), &id.Traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	for _, attr := range subject.Names {
		switch {
		case attr.Type.Equal(SystemRolesASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.SystemRoles = append(id.SystemRoles, val)
			}
		case attr.Type.Equal(KubeUsersASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.KubernetesUsers = append(id.KubernetesUsers, val)
			}
		case attr.Type.Equal(KubeGroupsASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.KubernetesGroups = append(id.KubernetesGroups, val)
			}
		case attr.Type.Equal(KubeClusterASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.KubernetesCluster = val
			}
		case attr.Type.Equal(AppSessionIDASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.SessionID = val
			}
		case attr.Type.Equal(AppPublicAddrASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.PublicAddr = val
			}
		case attr.Type.Equal(AppClusterNameASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.ClusterName = val
			}
		case attr.Type.Equal(AppNameASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.Name = val
			}
		case attr.Type.Equal(AppAWSRoleARNASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.AWSRoleARN = val
			}
		case attr.Type.Equal(AWSRoleARNsASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.AWSRoleARNs = append(id.AWSRoleARNs, val)
			}
		case attr.Type.Equal(AppAzureIdentityASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.AzureIdentity = val
			}
		case attr.Type.Equal(AzureIdentityASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.AzureIdentities = append(id.AzureIdentities, val)
			}
		case attr.Type.Equal(AppGCPServiceAccountASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToApp.GCPServiceAccount = val
			}
		case attr.Type.Equal(GCPServiceAccountsASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.GCPServiceAccounts = append(id.GCPServiceAccounts, val)
			}
		case attr.Type.Equal(RenewableCertificateASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.Renewable = val == types.True
			}
		case attr.Type.Equal(TeleportClusterASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.TeleportCluster = val
			}
		case attr.Type.Equal(MFAVerifiedASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.MFAVerified = val
			}
		case attr.Type.Equal(PreviousIdentityExpiresASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				asTime, err := time.Parse(time.RFC3339, val)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				id.PreviousIdentityExpires = asTime
			}
		case attr.Type.Equal(LoginIPASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.LoginIP = val
			}
		case attr.Type.Equal(DatabaseServiceNameASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToDatabase.ServiceName = val
			}
		case attr.Type.Equal(DatabaseProtocolASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToDatabase.Protocol = val
			}
		case attr.Type.Equal(DatabaseUsernameASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToDatabase.Username = val
			}
		case attr.Type.Equal(DatabaseNameASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToDatabase.Database = val
			}
		case attr.Type.Equal(RequestedDatabaseRolesExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.RouteToDatabase.Roles = append(id.RouteToDatabase.Roles, val)
			}
		case attr.Type.Equal(DatabaseNamesASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.DatabaseNames = append(id.DatabaseNames, val)
			}
		case attr.Type.Equal(DatabaseUsersASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.DatabaseUsers = append(id.DatabaseUsers, val)
			}
		case attr.Type.Equal(ImpersonatorASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.Impersonator = val
			}
		case attr.Type.Equal(ActiveRequestsASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.ActiveRequests = append(id.ActiveRequests, val)
			}
		case attr.Type.Equal(DisallowReissueASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.DisallowReissue = val == types.True
			}
		case attr.Type.Equal(GenerationASN1ExtensionOID):
			// This doesn't seem to play nice with int types, so we'll parse it
			// from a string.
			val, ok := attr.Value.(string)
			if ok {
				generation, err := strconv.ParseUint(val, 10, 64)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				id.Generation = generation
			}
		case attr.Type.Equal(BotASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.BotName = val
			}
		case attr.Type.Equal(BotInstanceASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.BotInstanceID = val
			}
		case attr.Type.Equal(AllowedResourcesASN1ExtensionOID):
			allowedResourcesStr, ok := attr.Value.(string)
			if ok {
				allowedResourceIDs, err := types.ResourceIDsFromString(allowedResourcesStr)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				id.AllowedResourceIDs = allowedResourceIDs
			}
		case attr.Type.Equal(PrivateKeyPolicyASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.PrivateKeyPolicy = keys.PrivateKeyPolicy(val)
			}
		case attr.Type.Equal(ConnectionDiagnosticIDASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.ConnectionDiagnosticID = val
			}
		case attr.Type.Equal(DeviceIDExtensionOID):
			if val, ok := attr.Value.(string); ok {
				id.DeviceExtensions.DeviceID = val
			}
		case attr.Type.Equal(DeviceAssetTagExtensionOID):
			if val, ok := attr.Value.(string); ok {
				id.DeviceExtensions.AssetTag = val
			}
		case attr.Type.Equal(DeviceCredentialIDExtensionOID):
			if val, ok := attr.Value.(string); ok {
				id.DeviceExtensions.CredentialID = val
			}
		case attr.Type.Equal(PinnedIPASN1ExtensionOID):
			if val, ok := attr.Value.(string); ok {
				id.PinnedIP = val
			}
		case attr.Type.Equal(UserTypeASN1ExtensionOID):
			if val, ok := attr.Value.(string); ok {
				id.UserType = types.UserType(val)
			}
		case attr.Type.Equal(JoinAttributesASN1ExtensionOID):
			if val, ok := attr.Value.(string); ok {
				id.JoinAttributes = &workloadidentityv1pb.JoinAttrs{}
				unmarshaler := protojson.UnmarshalOptions{
					// We specifically want to DiscardUnknown or unmarshaling
					// will fail if the proto message was issued by a newer
					// auth server w/ new fields.
					DiscardUnknown: true,
				}
				if err := unmarshaler.Unmarshal([]byte(val), id.JoinAttributes); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		}
	}

	if err := id.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return id, nil
}

func (id Identity) GetUserMetadata() events.UserMetadata {
	var device *events.DeviceMetadata
	if id.DeviceExtensions != (DeviceExtensions{}) {
		device = &events.DeviceMetadata{
			DeviceId:     id.DeviceExtensions.DeviceID,
			AssetTag:     id.DeviceExtensions.AssetTag,
			CredentialId: id.DeviceExtensions.CredentialID,
		}
	}

	userKind := events.UserKind_USER_KIND_HUMAN
	if id.BotName != "" {
		userKind = events.UserKind_USER_KIND_BOT
	}

	return events.UserMetadata{
		User:              id.Username,
		Impersonator:      id.Impersonator,
		AWSRoleARN:        id.RouteToApp.AWSRoleARN,
		AzureIdentity:     id.RouteToApp.AzureIdentity,
		GCPServiceAccount: id.RouteToApp.GCPServiceAccount,
		AccessRequests:    id.ActiveRequests,
		UserKind:          userKind,
		TrustedDevice:     device,
		BotName:           id.BotName,
		BotInstanceID:     id.BotInstanceID,
	}
}

func (id Identity) GetSessionMetadata(sid string) events.SessionMetadata {
	return events.SessionMetadata{
		SessionID:        sid,
		WithMFA:          id.MFAVerified,
		PrivateKeyPolicy: string(id.PrivateKeyPolicy),
	}
}

// IsMFAVerified returns whether this identity is MFA verified. This MFA
// verification may or may not have taken place recently, so it should not
// be treated as blanket MFA verification uncritically. For example, MFA
// should be re-verified for login procedures or admin actions.
func (id *Identity) IsMFAVerified() bool {
	return id.MFAVerified != "" || id.PrivateKeyPolicy.MFAVerified()
}

// CertificateRequest is a X.509 signing certificate request
type CertificateRequest struct {
	// Clock is a clock used to get current or test time
	Clock clockwork.Clock
	// PublicKey is a public key to sign
	PublicKey crypto.PublicKey
	// Subject is a subject to include in certificate
	Subject pkix.Name
	// NotAfter is a time after which the issued certificate
	// will be no longer valid
	NotAfter time.Time
	// DNSNames is a list of DNS names to add to certificate
	DNSNames []string
	// Optional. ExtraExtensions to populate.
	// Note: ExtraExtensions can override ExtKeyUsage and SANs (like DNSNames).
	ExtraExtensions []pkix.Extension
	// Optional. KeyUsage for the certificate.
	KeyUsage x509.KeyUsage
	// Optional. CRL endpoints.
	CRLDistributionPoints []string
}

// CheckAndSetDefaults checks and sets default values
func (c *CertificateRequest) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.PublicKey == nil {
		return trace.BadParameter("missing parameter PublicKey")
	}
	if c.Subject.CommonName == "" {
		return trace.BadParameter("missing parameter Subject.Common name")
	}
	if c.NotAfter.IsZero() {
		return trace.BadParameter("missing parameter NotAfter")
	}
	if c.KeyUsage == 0 {
		c.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	}

	c.DNSNames = utils.Deduplicate(c.DNSNames)

	return nil
}

// GenerateCertificate generates certificate from request
func (ca *CertAuthority) GenerateCertificate(req CertificateRequest) ([]byte, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"not_after":   req.NotAfter,
		"dns_names":   req.DNSNames,
		"key_usage":   req.KeyUsage,
		"common_name": req.Subject.CommonName,
	}).Debug("Generating TLS certificate")

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      req.Subject,
		// NotBefore is one minute in the past to prevent "Not yet valid" errors on
		// time skewed clusters.
		NotBefore:   req.Clock.Now().UTC().Add(-1 * time.Minute),
		NotAfter:    req.NotAfter,
		KeyUsage:    req.KeyUsage,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		// BasicConstraintsValid is true to not allow any intermediate certs.
		BasicConstraintsValid: true,
		IsCA:                  false,
		ExtraExtensions:       req.ExtraExtensions,
		CRLDistributionPoints: req.CRLDistributionPoints,
	}

	// sort out principals into DNS names and IP addresses
	for i := range req.DNSNames {
		if ip := net.ParseIP(req.DNSNames[i]); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, req.DNSNames[i])
		}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, req.PublicKey, ca.Signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes}), nil
}

// shouldPersistJoinAttrs returns true if the join attributes should be persisted
// into the X509 identity. This provides an emergency "off" handle for this
// new behavior until we are confident it is working as expected.
func shouldPersistJoinAttrs() bool {
	return os.Getenv("TELEPORT_UNSTABLE_DISABLE_JOIN_ATTRS") != "yes"
}
