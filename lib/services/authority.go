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

package services

import (
	"crypto/x509"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/trace"
	"github.com/tstranex/u2f"
)

// NewCertAuthority returns new cert authority
// DELETE in 7.0.0
func NewCertAuthority(
	caType CertAuthType,
	clusterName string,
	signingKeys [][]byte,
	checkingKeys [][]byte,
	roles []string,
	signingAlg CertAuthoritySpecV2_SigningAlgType,
) CertAuthority {
	return types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:         caType,
		ClusterName:  clusterName,
		SigningKeys:  signingKeys,
		CheckingKeys: checkingKeys,
		Roles:        roles,
		SigningAlg:   signingAlg,
	})
}

// HostCertParams defines all parameters needed to generate a host certificate
type HostCertParams struct {
	// PrivateCASigningKey is the private key of the CA that will sign the public key of the host
	PrivateCASigningKey []byte
	// CASigningAlg is the signature algorithm used by the CA private key.
	CASigningAlg string
	// PublicHostKey is the public key of the host
	PublicHostKey []byte
	// HostID is used by Teleport to uniquely identify a node within a cluster
	HostID string
	// Principals is a list of additional principals to add to the certificate.
	Principals []string
	// NodeName is the DNS name of the node
	NodeName string
	// ClusterName is the name of the cluster within which a node lives
	ClusterName string
	// Roles identifies the roles of a Teleport instance
	Roles teleport.Roles
	// TTL defines how long a certificate is valid for
	TTL time.Duration
}

// Check checks parameters for errors
func (c HostCertParams) Check() error {
	if len(c.PrivateCASigningKey) == 0 || c.CASigningAlg == "" {
		return trace.BadParameter("PrivateCASigningKey and CASigningAlg are required")
	}
	if c.HostID == "" && len(c.Principals) == 0 {
		return trace.BadParameter("HostID [%q] or Principals [%q] are required",
			c.HostID, c.Principals)
	}
	if c.ClusterName == "" {
		return trace.BadParameter("ClusterName [%q] is required", c.ClusterName)
	}

	if err := c.Roles.Check(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ChangePasswordReq defines a request to change user password
type ChangePasswordReq struct {
	// User is user ID
	User string
	// OldPassword is user current password
	OldPassword []byte `json:"old_password"`
	// NewPassword is user new password
	NewPassword []byte `json:"new_password"`
	// SecondFactorToken is user 2nd factor token
	SecondFactorToken string `json:"second_factor_token"`
	// U2FSignResponse is U2F sign response
	U2FSignResponse *u2f.SignResponse `json:"u2f_sign_response"`
}

// UserCertParams defines OpenSSH user certificate parameters
type UserCertParams struct {
	// PrivateCASigningKey is the private key of the CA that will sign the public key of the user
	PrivateCASigningKey []byte
	// CASigningAlg is the signature algorithm used by the CA private key.
	CASigningAlg string
	// PublicUserKey is the public key of the user
	PublicUserKey []byte
	// TTL defines how long a certificate is valid for
	TTL time.Duration
	// Username is teleport username
	Username string
	// AllowedLogins is a list of SSH principals
	AllowedLogins []string
	// PermitX11Forwarding permits X11 forwarding for this cert
	PermitX11Forwarding bool
	// PermitAgentForwarding permits agent forwarding for this cert
	PermitAgentForwarding bool
	// PermitPortForwarding permits port forwarding.
	PermitPortForwarding bool
	// Roles is a list of roles assigned to this user
	Roles []string
	// CertificateFormat is the format of the SSH certificate.
	CertificateFormat string
	// RouteToCluster specifies the target cluster
	// if present in the certificate, will be used
	// to route the requests to
	RouteToCluster string
	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits
	// ActiveRequests tracks privilege escalation requests applied during
	// certificate construction.
	ActiveRequests RequestIDs
}

// Check checks the user certificate parameters
func (c UserCertParams) Check() error {
	if len(c.PrivateCASigningKey) == 0 || c.CASigningAlg == "" {
		return trace.BadParameter("PrivateCASigningKey and CASigningAlg are required")
	}
	if c.TTL < defaults.MinCertDuration {
		return trace.BadParameter("TTL can't be less than %v", defaults.MinCertDuration)
	}
	if len(c.AllowedLogins) == 0 {
		return trace.BadParameter("AllowedLogins are required")
	}
	return nil
}

// CertPoolFromCertAuthorities returns certificate pools from TLS certificates
// set up in the certificate authorities list
func CertPoolFromCertAuthorities(cas []CertAuthority) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	for _, ca := range cas {
		keyPairs := ca.GetTLSKeyPairs()
		if len(keyPairs) == 0 {
			continue
		}
		for _, keyPair := range keyPairs {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			certPool.AddCert(cert)
		}
		return certPool, nil
	}
	return certPool, nil
}

// CertPool returns certificate pools from TLS certificates
// set up in the certificate authority
func CertPool(ca CertAuthority) (*x509.CertPool, error) {
	keyPairs := ca.GetTLSKeyPairs()
	if len(keyPairs) == 0 {
		return nil, trace.BadParameter("certificate authority has no TLS certificates")
	}
	certPool := x509.NewCertPool()
	for _, keyPair := range keyPairs {
		cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPool.AddCert(cert)
	}
	return certPool, nil
}

// TLSCerts returns TLS certificates from CA
func TLSCerts(ca CertAuthority) [][]byte {
	pairs := ca.GetTLSKeyPairs()
	out := make([][]byte, len(pairs))
	for i, pair := range pairs {
		out[i] = append([]byte{}, pair.Cert...)
	}
	return out
}
