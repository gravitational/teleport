/*
Copyright 2017-2019 Gravitational, Inc.

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

package tlsca

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAuthority,
})

// FromAuthority returns the CertificateAutority's TLS certificate authority from TLS key pairs.
func FromAuthority(ca types.CertAuthority) (*CertAuthority, error) {
	if len(ca.GetTLSKeyPairs()) == 0 {
		return nil, trace.BadParameter("no TLS key pairs found for certificate authority")
	}
	return FromKeys(ca.GetTLSKeyPairs()[0].Cert, ca.GetTLSKeyPairs()[0].Key)
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
		ca.Signer, err = ParsePrivateKeyPEM(keyPEM)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return ca, nil
}

// CertAuthority is X.509 certificate authority
type CertAuthority struct {
	// Cert is a CA certificate
	Cert *x509.Certificate
	// Signer is a private key based signer
	Signer crypto.Signer
}

// Identity is an identity of the user or service, e.g. Proxy or Node
type Identity struct {
	// Username is a username or name of the node connection
	Username string
	// Impersonator is a username of a user impersonating this user
	Impersonator string
	// Roles is a list of Teleport roles encoded in the identity
	Roles []string
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
	// ClientIP is an observed IP of the client that this Identity represents.
	ClientIP string
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
}

// String returns string representation of the database routing struct.
func (d RouteToDatabase) String() string {
	return fmt.Sprintf("Database(Service=%v, Protocol=%v, Username=%v, Database=%v)",
		d.ServiceName, d.Protocol, d.Username, d.Database)
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

// CheckAndSetDefaults checks and sets default values
func (id *Identity) CheckAndSetDefaults() error {
	if id.Username == "" {
		return trace.BadParameter("missing identity username")
	}
	if len(id.Roles) == 0 {
		return trace.BadParameter("missing identity groups")
	}

	return nil
}

// Custom ranges are taken from this article
//
// https://serverfault.com/questions/551477/is-there-reserved-oid-space-for-internal-enterprise-cas
//
// http://oid-info.com/get/1.3.9999
//
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

	// ClientIPASN1ExtensionOID is an extension ID used when encoding/decoding
	// the client IP into certificates.
	ClientIPASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 9}

	// AppNameASN1ExtensionOID is an extension ID used when encoding/decoding
	// application name into a certificate.
	AppNameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 10}

	// UsernameASN1ExtensionOID is an extension ID used when encoding/decoding
	// Teleport user name into certificates.
	UsernameASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 11}

	// RolesASN1ExtensionOID is an extension ID used when encoding/decoding
	// Teleport user roles into certificates.
	RolesASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 12}

	// UsageASN1ExtensionOID is an extension ID used when encoding/decoding
	// accepted usage into certificates.
	UsageASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 13}

	// PrincipalsASN1ExtensionOID is an extension ID used when encoding/decoding
	// principals into certificates.
	PrincipalsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 14}

	// RouteToClusterASN1ExtensionOID is an extension ID used when encoding/decoding
	// route to cluster into certificates.
	RouteToClusterASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 15}

	// TraitsASN1ExtensionOID is an extension ID used when encoding/decoding
	// user traits into certificates.
	TraitsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 16}

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
)

// Extensions encodes this identity as x509v3 extensions.
func (id *Identity) Extensions() (extensions []pkix.Extension, err error) {
	rawTraits, err := wrappers.MarshalTraits(&id.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if id.Username != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    UsernameASN1ExtensionOID,
			Value: []byte(id.Username),
		})
	}
	for i := range id.Roles {
		extensions = append(extensions, pkix.Extension{
			Id:    RolesASN1ExtensionOID,
			Value: []byte(id.Roles[i]),
		})
	}
	for i := range id.Usage {
		extensions = append(extensions, pkix.Extension{
			Id:    UsageASN1ExtensionOID,
			Value: []byte(id.Usage[i]),
		})
	}
	for i := range id.Principals {
		extensions = append(extensions, pkix.Extension{
			Id:    PrincipalsASN1ExtensionOID,
			Value: []byte(id.Principals[i]),
		})
	}
	if id.RouteToCluster != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    RouteToClusterASN1ExtensionOID,
			Value: []byte(id.RouteToCluster),
		})
	}
	if len(id.Traits) != 0 {
		extensions = append(extensions, pkix.Extension{
			Id:    TraitsASN1ExtensionOID,
			Value: rawTraits,
		})
	}
	for i := range id.KubernetesUsers {
		extensions = append(extensions, pkix.Extension{
			Id:    KubeUsersASN1ExtensionOID,
			Value: []byte(id.KubernetesUsers[i]),
		})
	}
	for i := range id.KubernetesGroups {
		extensions = append(extensions, pkix.Extension{
			Id:    KubeGroupsASN1ExtensionOID,
			Value: []byte(id.KubernetesGroups[i]),
		})
	}
	if id.KubernetesCluster != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    KubeClusterASN1ExtensionOID,
			Value: []byte(id.KubernetesCluster),
		})
	}
	if id.RouteToApp.SessionID != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    AppSessionIDASN1ExtensionOID,
			Value: []byte(id.RouteToApp.SessionID),
		})
	}
	if id.RouteToApp.PublicAddr != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    AppPublicAddrASN1ExtensionOID,
			Value: []byte(id.RouteToApp.PublicAddr),
		})
	}
	if id.RouteToApp.ClusterName != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    AppClusterNameASN1ExtensionOID,
			Value: []byte(id.RouteToApp.ClusterName),
		})
	}
	if id.RouteToApp.Name != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    AppNameASN1ExtensionOID,
			Value: []byte(id.RouteToApp.Name),
		})
	}
	if id.TeleportCluster != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    TeleportClusterASN1ExtensionOID,
			Value: []byte(id.TeleportCluster),
		})
	}
	if id.MFAVerified != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    MFAVerifiedASN1ExtensionOID,
			Value: []byte(id.MFAVerified),
		})
	}
	if id.ClientIP != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    ClientIPASN1ExtensionOID,
			Value: []byte(id.ClientIP),
		})
	}
	if id.RouteToDatabase.ServiceName != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    DatabaseServiceNameASN1ExtensionOID,
			Value: []byte(id.RouteToDatabase.ServiceName),
		})
	}
	if id.RouteToDatabase.Protocol != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    DatabaseProtocolASN1ExtensionOID,
			Value: []byte(id.RouteToDatabase.Protocol),
		})
	}
	if id.RouteToDatabase.Username != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    DatabaseUsernameASN1ExtensionOID,
			Value: []byte(id.RouteToDatabase.Username),
		})
	}
	if id.RouteToDatabase.Database != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    DatabaseNameASN1ExtensionOID,
			Value: []byte(id.RouteToDatabase.Database),
		})
	}
	for i := range id.DatabaseNames {
		extensions = append(extensions, pkix.Extension{
			Id:    DatabaseNamesASN1ExtensionOID,
			Value: []byte(id.DatabaseNames[i]),
		})
	}
	for i := range id.DatabaseUsers {
		extensions = append(extensions, pkix.Extension{
			Id:    DatabaseUsersASN1ExtensionOID,
			Value: []byte(id.DatabaseUsers[i]),
		})
	}
	if id.Impersonator != "" {
		extensions = append(extensions, pkix.Extension{
			Id:    ImpersonatorASN1ExtensionOID,
			Value: []byte(id.Impersonator),
		})
	}
	return extensions, nil
}

// Subject converts identity to X.509 subject name
//
// DELETE IN 8.0.0: By then all officially compatible clients will be parsing
// identity from x509v3 extensions so encoding in subject will no longer be
// needed.
func (id *Identity) Subject() (pkix.Name, error) {
	rawTraits, err := wrappers.MarshalTraits(&id.Traits)
	if err != nil {
		return pkix.Name{}, trace.Wrap(err)
	}

	subject := pkix.Name{
		CommonName: id.Username,
	}
	subject.Organization = append([]string{}, id.Roles...)
	subject.OrganizationalUnit = append([]string{}, id.Usage...)
	subject.Locality = append([]string{}, id.Principals...)

	// DELETE IN (5.0.0)
	// Groups are marshaled to both ASN1 extension
	// and old Province section, for backwards-compatibility,
	// however begin migration to ASN1 extensions in the future
	// for this and other properties
	subject.Province = append([]string{}, id.KubernetesGroups...)
	subject.StreetAddress = []string{id.RouteToCluster}
	subject.PostalCode = []string{string(rawTraits)}

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
	if id.ClientIP != "" {
		subject.ExtraNames = append(subject.ExtraNames,
			pkix.AttributeTypeAndValue{
				Type:  ClientIPASN1ExtensionOID,
				Value: id.ClientIP,
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

	return subject, nil
}

// FromCertificate parses identity from the provided x509 certificate.
func FromCertificate(cert *x509.Certificate) (*Identity, error) {
	return FromSubjectAndExtensions(cert.Subject, cert.Extensions, cert.NotAfter)
}

// FromCSR parses identity from the provided certificate request.
func FromCSR(csr *x509.CertificateRequest) (*Identity, error) {
	return FromSubjectAndExtensions(csr.Subject, csr.Extensions, time.Time{})
}

// FromSubjectAndExtensions parses identity from the provided x509v3 extensions
// with fallback to subject where identity was encoded previously.
func FromSubjectAndExtensions(subject pkix.Name, extensions []pkix.Extension, expires time.Time) (*Identity, error) {
	id := &Identity{
		Expires: expires,
	}

	// First decode identity properties that were previously encoded in the
	// standard subject fields (such as CN, O, OU, etc).
	id.Username = getExtensionValue(extensions, UsernameASN1ExtensionOID)
	if len(id.Username) == 0 {
		id.Username = subject.CommonName
	}
	id.Roles = getExtensionValues(extensions, RolesASN1ExtensionOID)
	if len(id.Roles) == 0 {
		id.Roles = subject.Organization
	}
	id.Usage = getExtensionValues(extensions, UsageASN1ExtensionOID)
	if len(id.Usage) == 0 {
		id.Usage = subject.OrganizationalUnit
	}
	id.Principals = getExtensionValues(extensions, PrincipalsASN1ExtensionOID)
	if len(id.Principals) == 0 {
		id.Principals = subject.Locality
	}
	id.RouteToCluster = getExtensionValue(extensions, RouteToClusterASN1ExtensionOID)
	if len(id.RouteToCluster) == 0 && len(subject.StreetAddress) > 0 {
		id.RouteToCluster = subject.StreetAddress[0]
	}
	rawTraits := getExtensionValue(extensions, TraitsASN1ExtensionOID)
	if len(rawTraits) == 0 && len(subject.PostalCode) > 0 {
		rawTraits = subject.PostalCode[0]
	}
	if len(rawTraits) != 0 {
		if err := wrappers.UnmarshalTraits([]byte(rawTraits), &id.Traits); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Remaining identity properties were being encoded in the subject as
	// extensions fields with the same OIDs as we're using now for x509v3
	// extensions.
	id.KubernetesUsers = getValues(subject, extensions, KubeUsersASN1ExtensionOID)
	id.KubernetesGroups = getValues(subject, extensions, KubeGroupsASN1ExtensionOID)
	id.KubernetesCluster = getValue(subject, extensions, KubeClusterASN1ExtensionOID)
	id.RouteToApp.SessionID = getValue(subject, extensions, AppSessionIDASN1ExtensionOID)
	id.RouteToApp.PublicAddr = getValue(subject, extensions, AppPublicAddrASN1ExtensionOID)
	id.RouteToApp.ClusterName = getValue(subject, extensions, AppClusterNameASN1ExtensionOID)
	id.RouteToApp.Name = getValue(subject, extensions, AppNameASN1ExtensionOID)
	id.TeleportCluster = getValue(subject, extensions, TeleportClusterASN1ExtensionOID)
	id.MFAVerified = getValue(subject, extensions, MFAVerifiedASN1ExtensionOID)
	id.ClientIP = getValue(subject, extensions, ClientIPASN1ExtensionOID)
	id.RouteToDatabase.ServiceName = getValue(subject, extensions, DatabaseServiceNameASN1ExtensionOID)
	id.RouteToDatabase.Protocol = getValue(subject, extensions, DatabaseProtocolASN1ExtensionOID)
	id.RouteToDatabase.Username = getValue(subject, extensions, DatabaseUsernameASN1ExtensionOID)
	id.RouteToDatabase.Database = getValue(subject, extensions, DatabaseNameASN1ExtensionOID)
	id.DatabaseNames = getValues(subject, extensions, DatabaseNamesASN1ExtensionOID)
	id.DatabaseUsers = getValues(subject, extensions, DatabaseUsersASN1ExtensionOID)
	id.Impersonator = getValue(subject, extensions, ImpersonatorASN1ExtensionOID)

	// Make sure the identity we got in the end is valid.
	if err := id.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return id, nil
}

// getValue returns a string value of the specified extension from x509v3
// extensions with fallback to subject.
func getValue(subject pkix.Name, extensions []pkix.Extension, oid asn1.ObjectIdentifier) string {
	value := getExtensionValue(extensions, oid)
	if len(value) == 0 {
		value = getSubjectValue(subject, oid)
	}
	return value
}

// getValues returns a list of string values of the specified extension from
// x509v3 extensions with fallback to subject.
func getValues(subject pkix.Name, extensions []pkix.Extension, oid asn1.ObjectIdentifier) []string {
	values := getExtensionValues(extensions, oid)
	if len(values) == 0 {
		values = getSubjectValues(subject, oid)
	}
	return values
}

// getExtensionValue returns a string value of the specified x509v3 extension.
func getExtensionValue(extensions []pkix.Extension, oid asn1.ObjectIdentifier) string {
	for _, extension := range extensions {
		if extension.Id.Equal(oid) {
			return string(extension.Value)
		}
	}
	return ""
}

// getExtensionValues returns a list of string values of the specified x509v3
// extension.
func getExtensionValues(extensions []pkix.Extension, oid asn1.ObjectIdentifier) (result []string) {
	for _, extension := range extensions {
		if extension.Id.Equal(oid) {
			result = append(result, string(extension.Value))
		}
	}
	return result
}

// getSubjectValue returns a string value of the specified extension encoded
// in subject.
func getSubjectValue(subject pkix.Name, oid asn1.ObjectIdentifier) string {
	for _, value := range subject.Names {
		if value.Type.Equal(oid) {
			if s, ok := value.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// getSubjectValues returns a list of string values of the specified extension
// encoded in subject.
func getSubjectValues(subject pkix.Name, oid asn1.ObjectIdentifier) (result []string) {
	for _, value := range subject.Names {
		if value.Type.Equal(oid) {
			if s, ok := value.Value.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}

// CertificateRequest is a X.509 signing certificate request
type CertificateRequest struct {
	// Clock is a clock used to get current or test time
	Clock clockwork.Clock
	// PublicKey is a public key to sign
	PublicKey crypto.PublicKey
	// Subject is a subject to include in certificate
	Subject pkix.Name
	// Extensions is x509v3 extensions to include in the certificate
	Extensions []pkix.Extension
	// NotAfter is a time after which the issued certificate
	// will be no longer valid
	NotAfter time.Time
	// DNSNames is a list of DNS names to add to certificate
	DNSNames []string
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
		"common_name": req.Subject.CommonName,
		"org":         req.Subject.Organization,
		"org_unit":    req.Subject.OrganizationalUnit,
		"locality":    req.Subject.Locality,
	}).Infof("Generating TLS certificate %v.", req)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      req.Subject,
		// Must use ExtraExtensions, not Extensions, when marshaling certificates.
		ExtraExtensions: req.Extensions,
		// NotBefore is one minute in the past to prevent "Not yet valid" errors on
		// time skewed clusters.
		NotBefore:   req.Clock.Now().UTC().Add(-1 * time.Minute),
		NotAfter:    req.NotAfter,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		// BasicConstraintsValid is true to not allow any intermediate certs.
		BasicConstraintsValid: true,
		IsCA:                  false,
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
