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
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAuthority,
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
	// Groups is a list of groups (Teleport roles) encoded in the identity
	Groups []string
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
	// AWSRoleARNs is a list of allowed AWS role ARNs user can assume.
	AWSRoleARNs []string
	// ActiveRequests is a list of UUIDs of active requests for this Identity.
	ActiveRequests []string
	// DisallowReissue is a flag that, if set, instructs the auth server to
	// deny any attempts to reissue new certificates while authenticated with
	// this certificate.
	DisallowReissue bool
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

	// AppAWSRoleARNASN1ExtensionOID is an extension ID used when encoding/decoding
	// AWS role ARN into a certificate.
	AppAWSRoleARNASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 11}

	// AWSRoleARNsASN1ExtensionOID is an extension ID used when encoding/decoding
	// allowed AWS role ARNs into a certificate.
	AWSRoleARNsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 12}

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
)

// Subject converts identity to X.509 subject name
func (id *Identity) Subject() (pkix.Name, error) {
	rawTraits, err := wrappers.MarshalTraits(&id.Traits)
	if err != nil {
		return pkix.Name{}, trace.Wrap(err)
	}

	subject := pkix.Name{
		CommonName: id.Username,
	}
	subject.Organization = append([]string{}, id.Groups...)
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
		case attr.Type.Equal(ClientIPASN1ExtensionOID):
			val, ok := attr.Value.(string)
			if ok {
				id.ClientIP = val
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
		}
	}

	// DELETE IN(5.0.0): This logic is using Province field
	// from subject in case if Kubernetes groups were not populated
	// from ASN1 extension, after 5.0 Province field will be ignored
	if len(id.KubernetesGroups) == 0 {
		id.KubernetesGroups = subject.Province
	}

	if err := id.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return id, nil
}

func (id Identity) GetUserMetadata() events.UserMetadata {
	return events.UserMetadata{
		User:           id.Username,
		Impersonator:   id.Impersonator,
		AccessRequests: id.ActiveRequests,
	}
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
