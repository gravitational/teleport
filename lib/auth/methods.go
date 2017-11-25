/*
Copyright 2017 Gravitational, Inc.

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

package auth

import (
	"bytes"
	"crypto/rsa"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/tstranex/u2f"
)

// AuthenticateUserRequest is a request to authenticate interactive user
type AuthenticateUserRequest struct {
	// Username is a user name
	Username string `json:"username"`
	// Pass is a password used in local authentication schemes
	Pass *PassCreds `json:"pass,omitempty"`
	// U2F is a sign response crdedentials used to authenticate via U2F
	U2F *U2FSignResponseCreds `json:"u2f,omitempty"`
	// OTP is a password and second factor, used in two factor authentication
	OTP *OTPCreds `json:"otp,omitempty"`
	// Session is a web session credential used to authenticate web sessions
	Session *SessionCreds `json:"session,omitempty"`
}

// CheckAndSetDefaults checks and sets defaults
func (a *AuthenticateUserRequest) CheckAndSetDefaults() error {
	if a.Username == "" {
		return trace.BadParameter("missing parameter 'username'")
	}
	if a.Pass == nil && a.U2F == nil && a.OTP == nil && a.Session == nil {
		return trace.BadParameter("at least one authentication method is required")
	}
	return nil
}

// PassCreds is a password credential
type PassCreds struct {
	// Password is a user password
	Password []byte `json:"password"`
}

// U2FSignResponseCreds is a U2F signature sent by U2F device
type U2FSignResponseCreds struct {
	// SignResponse is a U2F sign resposne
	SignResponse u2f.SignResponse `json:"sign_response"`
}

// OTPCreds is a two factor authencication credentials
type OTPCreds struct {
	// Password is a user password
	Password []byte `json:"password"`
	// Token is a user second factor token
	Token string `json:"token"`
}

// SessionCreds is a web session credentials
type SessionCreds struct {
	// ID is a web session id
	ID string `json:"id"`
}

// AuthenticateUser authenticates user based on the request type
func (s *AuthServer) AuthenticateUser(req AuthenticateUserRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	authPreference, err := s.GetAuthPreference()
	if err != nil {
		return trace.Wrap(err)
	}

	switch {
	case req.Pass != nil:
		// authenticate using password only, make sure
		// that auth preference does not require second factor
		// otherwise users can bypass the second factor
		if authPreference.GetSecondFactor() != teleport.OFF {
			return trace.AccessDenied("missing second factor")
		}
		err := s.WithUserLock(req.Username, func() error {
			return s.CheckPasswordWOToken(req.Username, req.Pass.Password)
		})
		if err != nil {
			// provide obscure message on purpose, while logging the real
			// error server side
			log.Debugf("Failed to authenticate: %v.", err)
			return trace.AccessDenied("invalid username or password")
		}
		return nil
	case req.U2F != nil:
		// authenticate using U2F - code checks challenge response
		// signed by U2F device of the user
		err := s.WithUserLock(req.Username, func() error {
			return s.CheckU2FSignResponse(req.Username, &req.U2F.SignResponse)
		})
		if err != nil {
			// provide obscure message on purpose, while logging the real
			// error server side
			log.Debugf("Failed to authenticate: %v.", err)
			return trace.AccessDenied("invalid U2F response")
		}
		return nil
	case req.OTP != nil:
		err := s.WithUserLock(req.Username, func() error {
			return s.CheckPassword(req.Username, req.OTP.Password, req.OTP.Token)
		})
		if err != nil {
			// provide obscure message on purpose, while logging the real
			// error server side
			log.Debugf("Failed to authenticate: %v.", err)
			return trace.AccessDenied("invalid username, password or second factor")
		}
		return nil
	default:
		return trace.AccessDenied("unsupported authentication method")
	}
}

// AuthenticateWebUser authenticates web user, creates and  returns web session
// in case if authentication is successfull. In case if existing session id
// is used to authenticate, returns session associated with the existing session id
// instead of creating the new one
func (s *AuthServer) AuthenticateWebUser(req AuthenticateUserRequest) (services.WebSession, error) {
	if req.Session != nil {
		session, err := s.GetWebSession(req.Username, req.Session.ID)
		if err != nil {
			return nil, trace.AccessDenied("session is invalid or has expired")
		}
		return session, nil
	}
	if err := s.AuthenticateUser(req); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := s.NewWebSession(req.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.UpsertWebSession(req.Username, sess); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err = services.GetWebSessionMarshaler().GenerateWebSession(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// AuthenticateSSHRequest is a request to authenticate SSH client user via CLI
type AuthenticateSSHRequest struct {
	// AuthenticateUserRequest is a request with credentials
	AuthenticateUserRequest
	// PublicKey is a public key in ssh authorized_keys format
	PublicKey []byte `json:"public_key"`
	// TTL is a requested TTL for certificates to be issues
	TTL time.Duration `json:"ttl"`
	// CompatibilityMode sets certificate compatibility mode with old SSH clients
	CompatibilityMode string `json:"compatibility_mode"`
}

// CheckAndSetDefaults checks and sets default certificate values
func (a *AuthenticateSSHRequest) CheckAndSetDefaults() error {
	if err := a.AuthenticateUserRequest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if len(a.PublicKey) == 0 {
		return trace.BadParameter("missing parameter 'public_key'")
	}
	compatibility, err := utils.CheckCompatibilityFlag(a.CompatibilityMode)
	if err != nil {
		return trace.Wrap(err)
	}
	a.CompatibilityMode = compatibility
	return nil
}

// SSHLoginResponse is a response returned by web proxy, it preserves backwards compatibility
// on the wire, which is the primary reason for non-matching json tags
type SSHLoginResponse struct {
	// User contains a logged in user informationn
	Username string `json:"username"`
	// Cert is a PEM encoded  signed certificate
	Cert []byte `json:"cert"`
	// TLSCertPEM is a PEM encoded TLS certificate signed by TLS certificate authority
	TLSCert []byte `json:"tls_cert"`
	// HostSigners is a list of signing host public keys trusted by proxy
	HostSigners []TrustedCerts `json:"host_signers"`
}

// TrustedCerts contains host certificates, it preserves backwards compatibility
// on the wire, which is the primary reason for non-matching json tags
type TrustedCerts struct {
	// ClusterName identifies teleport cluster name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	ClusterName string `json:"domain_name"`
	// HostCertificates is a list of SSH public keys that can be used to check
	// host certificate signatures
	HostCertificates [][]byte `json:"checking_keys"`
	// TLSCertificates  is a list of TLS certificates of the certificate authoritiy
	// of the authentication server
	TLSCertificates [][]byte `json:"tls_certs"`
}

// SSHCertPublicKeys returns a list of trusted host SSH certificate authority public keys
func (c *TrustedCerts) SSHCertPublicKeys() ([]ssh.PublicKey, error) {
	out := make([]ssh.PublicKey, 0, len(c.HostCertificates))
	for _, keyBytes := range c.HostCertificates {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, publicKey)
	}
	return out, nil
}

// AuthoritiesToTrustedCerts serializes authorities to TrustedCerts data structure
func AuthoritiesToTrustedCerts(authorities []services.CertAuthority) []TrustedCerts {
	out := make([]TrustedCerts, len(authorities))
	for i, ca := range authorities {
		out[i] = TrustedCerts{
			ClusterName:      ca.GetClusterName(),
			HostCertificates: ca.GetCheckingKeys(),
			TLSCertificates:  services.TLSCerts(ca),
		}
	}
	return out
}

// AuthenticateSSHUser authenticates web user, creates and  returns web session
// in case if authentication is successful
func (s *AuthServer) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	if err := s.AuthenticateUser(req.AuthenticateUserRequest); err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := s.GetUser(req.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, err := services.FetchRoles(user.GetRoles(), s, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostCertAuthorities, err := s.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := s.generateUserCert(certRequest{
		user:          user,
		roles:         roles,
		ttl:           req.TTL,
		publicKey:     req.PublicKey,
		compatibility: req.CompatibilityMode,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SSHLoginResponse{
		Username:    req.Username,
		Cert:        certs.ssh,
		TLSCert:     certs.tls,
		HostSigners: AuthoritiesToTrustedCerts(hostCertAuthorities),
	}, nil
}

// DELETE IN: 2.6.0
// This method is used only for upgrades from 2.4.0 to 2.5.0
// ExchangeCertsRequest is a request to exchange TLS certificates
// for clusters that already trust each other
type ExchangeCertsRequest struct {
	// PublicKey is public key of the trusted certificate authority
	PublicKey []byte `json:"public_key"`
	// TLSCert is TLS certificate associated with the public key
	TLSCert []byte `json:"tls_cert"`
}

// CheckAndSetDefaults checks and sets default values
func (req *ExchangeCertsRequest) CheckAndSetDefaults() error {
	if len(req.PublicKey) == 0 {
		return trace.BadParameter("missing parameter 'public_key'")
	}
	if len(req.TLSCert) == 0 {
		return trace.BadParameter("missing parameter 'tls_cert'")
	}
	return nil
}

// DELETE IN: 2.6.0
// ExchangeCertsResponse is a resposne to exchange certificates request
type ExchangeCertsResponse struct {
	// TLSCert is a PEM encoded certificate of a local certificate authority
	TLSCert []byte `json:"tls_cert"`
}

// DELETE IN: 2.6.0
// This method is used to ugprade from 2.4.0 to 2.5.0
// ExchangeCerts is a method to exchange TLS certificates between certificate authorities
// of the trusted clusters. A remote auth server that wishes to exchange TLS certs with a local auth server
// sends a request that consists of a public key already trusted by the local server and
// TLS certificate for the public key. The local server ensures that the TLS certificate
// was issued to the public key that is already trusted preventing random certificates
// to be injected by the remote server. This is a minor security enforcement.
func (s *AuthServer) ExchangeCerts(req ExchangeCertsRequest) (*ExchangeCertsResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	remoteCA, err := s.findCertAuthorityByPublicKey(req.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := CheckPublicKeysEqual(req.PublicKey, req.TLSCert); err != nil {
		return nil, trace.Wrap(err)
	}

	// make sure that cluster name in TLS cert is not the same as cluster name
	cert, err := tlsca.ParseCertificatePEM(req.TLSCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteClusterName, err := tlsca.ClusterName(cert.Subject)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if remoteClusterName == clusterName.GetName() {
		return nil, trace.BadParameter("remote cluster name can not be the same as local cluster name")
	}

	remoteCA.SetTLSKeyPairs([]services.TLSKeyPair{
		{
			Cert: req.TLSCert,
		},
	})

	err = s.UpsertCertAuthority(remoteCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	thisHostCA, err := s.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: clusterName.GetClusterName()}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ExchangeCertsResponse{
		TLSCert: thisHostCA.GetTLSKeyPairs()[0].Cert,
	}, nil

}

func (s *AuthServer) findCertAuthorityByPublicKey(publicKey []byte) (services.CertAuthority, error) {
	authorities, err := s.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ca := range authorities {
		for _, key := range ca.GetCheckingKeys() {
			if bytes.Equal(key, publicKey) {
				return ca, nil
			}
		}
	}
	return nil, trace.NotFound("certificate authority with public key is not found")
}

// CheckPublicKeysEqual compares RSA based SSH certificate with the
// TLS certificate, returns nil if both certificates are using the same public
// key and refer to the same cluster name, error otherwise
func CheckPublicKeysEqual(sshKeyBytes []byte, certBytes []byte) error {
	cert, err := tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	certPublicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return trace.BadParameter("expected RSA public key, got %T", cert.PublicKey)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(sshKeyBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	cryptoPubKey, ok := publicKey.(ssh.CryptoPublicKey)
	if !ok {
		return trace.BadParameter("unexpected key type: %T", publicKey)
	}
	rsaPublicKey, ok := cryptoPubKey.CryptoPublicKey().(*rsa.PublicKey)
	if !ok {
		return trace.BadParameter("unexpected key type: %T", publicKey)
	}
	if certPublicKey.E != rsaPublicKey.E {
		return trace.CompareFailed("different public keys")
	}
	if certPublicKey.N.Cmp(rsaPublicKey.N) != 0 {
		return trace.CompareFailed("different public keys")
	}
	return nil
}
