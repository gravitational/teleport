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
	"math/big"
	"net"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAuthority,
})

// New returns new CA from PEM encoded certificate and private
// key. Private Key is optional, if omitted CA won't be able to
// issue new certificates, only verify them
func New(certPEM, keyPEM []byte) (*CertAuthority, error) {
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
	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits
}

// CheckAndSetDefaults checks and sets default values
func (i *Identity) CheckAndSetDefaults() error {
	if i.Username == "" {
		return trace.BadParameter("missing identity username")
	}
	if len(i.Groups) == 0 {
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

// KubeUsersASN1ExtensionOID is an extension ID used when encoding/decoding
// license payload into certificates
var KubeUsersASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 1}

// KubeGroupsASN1ExtensionOID is an extension ID used when encoding/decoding
// license payload into certificates
var KubeGroupsASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 2}

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
}

// CheckAndSetDefaults checks and sets default values
func (c *CertificateRequest) CheckAndSetDefaults() error {
	if c.Clock == nil {
		return trace.BadParameter("missing parameter Clock")
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
