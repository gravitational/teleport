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
	"context"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// AuthenticateUserRequest is a request to authenticate interactive user
type AuthenticateUserRequest struct {
	// Username is a user name
	Username string `json:"username"`
	// Pass is a password used in local authentication schemes
	Pass *PassCreds `json:"pass,omitempty"`
	// U2F is a sign response credentials used to authenticate via U2F
	U2F *U2FSignResponseCreds `json:"u2f,omitempty"`
	// Webauthn is a signed credential assertion used to authenticate via WebAuthn
	// or U2F devices.
	Webauthn *wanlib.CredentialAssertionResponse `json:"webauthn,omitempty"`
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
	if a.Pass == nil && a.U2F == nil && a.Webauthn == nil && a.OTP == nil && a.Session == nil {
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
	SignResponse u2f.AuthenticateChallengeResponse `json:"sign_response"`
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
func (s *Server) AuthenticateUser(req AuthenticateUserRequest) error {
	mfaDev, err := s.authenticateUser(context.TODO(), req)
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserLocalLoginFailureCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: req.Username,
		},
		Method: events.LoginMethodLocal,
	}
	if mfaDev != nil {
		m := mfaDeviceEventMetadata(mfaDev)
		event.MFADevice = &m
	}
	if err != nil {
		event.Code = events.UserLocalLoginFailureCode
		event.Status.Success = false
		event.Status.Error = err.Error()
	} else {
		event.Code = events.UserLocalLoginCode
		event.Status.Success = true
	}
	if err := s.emitter.EmitAuditEvent(s.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit login event.")
	}
	return err
}

func (s *Server) authenticateUser(ctx context.Context, req AuthenticateUserRequest) (*types.MFADevice, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	user := req.Username

	// Try 2nd-factor-enabled authentication schemes first.
	var authenticateFn func() (*types.MFADevice, error)
	var failMsg string // failMsg kept obscure on purpose, use logging for details
	switch {
	// cases in order of preference
	case req.Webauthn != nil:
		authenticateFn = func() (*types.MFADevice, error) {
			mfaResponse := &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: wanlib.CredentialAssertionResponseToProto(req.Webauthn),
				},
			}
			return s.validateMFAAuthResponse(ctx, user, mfaResponse, s.Identity /* u2fStorage */)
		}
		failMsg = "invalid Webauthn response"
	case req.U2F != nil:
		authenticateFn = func() (*types.MFADevice, error) {
			mfaResponse := &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_U2F{
					U2F: &proto.U2FResponse{
						KeyHandle:  req.U2F.SignResponse.KeyHandle,
						ClientData: req.U2F.SignResponse.ClientData,
						Signature:  req.U2F.SignResponse.SignatureData,
					},
				},
			}
			return s.validateMFAAuthResponse(ctx, user, mfaResponse, s.Identity /* u2fStorage */)
		}
		failMsg = "invalid U2F response"
	case req.OTP != nil:
		authenticateFn = func() (*types.MFADevice, error) {
			// OTP cannot be validated by validateMFAAuthResponse because we need to
			// check the user's password too.
			res, err := s.checkPassword(user, req.OTP.Password, req.OTP.Token)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return res.mfaDev, nil
		}
		failMsg = "invalid username, password or second factor"
	}
	if authenticateFn != nil {
		var dev *types.MFADevice
		err := s.WithUserLock(user, func() error {
			var err error
			dev, err = authenticateFn()
			return err
		})
		switch {
		case err != nil:
			log.Debugf("User %v failed to authenticate: %v.", user, err)
			return nil, trace.AccessDenied(failMsg)
		case dev == nil:
			log.Debugf(
				"MFA authentication returned nil device (Webauthn = %v, U2F = %v, TOTP = %v): %v.",
				req.Webauthn != nil, req.U2F != nil, req.OTP != nil, err)
			return nil, trace.AccessDenied(failMsg)
		default:
			return dev, nil
		}
	}

	// Try password-only authentication last.
	if req.Pass == nil {
		return nil, trace.AccessDenied("unsupported authentication method")
	}

	authPreference, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// When using password only make sure that auth preference does not require
	// second factor, otherwise users could bypass it.
	switch authPreference.GetSecondFactor() {
	case constants.SecondFactorOff:
		// No 2FA required, check password only.
	case constants.SecondFactorOptional:
		// 2FA is optional. Make sure that a user does not have MFA devices
		// registered.
		devs, err := s.Identity.GetMFADevices(ctx, user, false /* withSecrets */)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if len(devs) != 0 {
			log.Warningf("MFA bypass attempt by user %q, access denied.", user)
			return nil, trace.AccessDenied("missing second factor authentication")
		}
	default:
		// Some form of MFA is required but none provided. Either client is
		// buggy (didn't send MFA response) or someone is trying to bypass
		// MFA.
		log.Warningf("MFA bypass attempt by user %q, access denied.", user)
		return nil, trace.AccessDenied("missing second factor")
	}
	if err = s.WithUserLock(user, func() error {
		return s.checkPasswordWOToken(user, req.Pass.Password)
	}); err != nil {
		// provide obscure message on purpose, while logging the real
		// error server side
		log.Debugf("User %v failed to authenticate: %v.", user, err)
		return nil, trace.AccessDenied("invalid username or password")
	}
	return nil, nil
}

// AuthenticateWebUser authenticates web user, creates and returns a web session
// if authentication is successful. In case the existing session ID is used to authenticate,
// returns the existing session instead of creating a new one
func (s *Server) AuthenticateWebUser(req AuthenticateUserRequest) (types.WebSession, error) {
	ctx := context.TODO()
	authPref, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Disable all local auth requests,
	// except session ID renewal requests that are using the same method.
	// This condition uses Session as a blanket check, because any new method added
	// to the local auth will be disabled by default.
	if !authPref.GetAllowLocalAuth() && req.Session == nil {
		s.emitNoLocalAuthEvent(req.Username)
		return nil, trace.AccessDenied(noLocalAuth)
	}

	if req.Session != nil {
		session, err := s.GetWebSession(context.TODO(), types.GetWebSessionRequest{
			User:      req.Username,
			SessionID: req.Session.ID,
		})
		if err != nil {
			return nil, trace.AccessDenied("session is invalid or has expired")
		}
		return session, nil
	}

	if err := s.AuthenticateUser(req); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := s.GetUser(req.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.createUserWebSession(context.TODO(), user)
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
	RouteToCluster    string `json:"route_to_cluster"`
	// KubernetesCluster sets the target kubernetes cluster for the TLS
	// certificate. This can be empty on older clients.
	KubernetesCluster string `json:"kubernetes_cluster"`
}

// CheckAndSetDefaults checks and sets default certificate values
func (a *AuthenticateSSHRequest) CheckAndSetDefaults() error {
	if err := a.AuthenticateUserRequest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if len(a.PublicKey) == 0 {
		return trace.BadParameter("missing parameter 'public_key'")
	}
	certificateFormat, err := utils.CheckCertificateFormatFlag(a.CompatibilityMode)
	if err != nil {
		return trace.Wrap(err)
	}
	a.CompatibilityMode = certificateFormat
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
func AuthoritiesToTrustedCerts(authorities []types.CertAuthority) []TrustedCerts {
	out := make([]TrustedCerts, len(authorities))
	for i, ca := range authorities {
		out[i] = TrustedCerts{
			ClusterName:      ca.GetClusterName(),
			HostCertificates: services.GetSSHCheckingKeys(ca),
			TLSCertificates:  services.GetTLSCerts(ca),
		}
	}
	return out
}

// AuthenticateSSHUser authenticates an SSH user and returns SSH and TLS
// certificates for the public key in req.
func (s *Server) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	ctx := context.TODO()
	authPref, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authPref.GetAllowLocalAuth() {
		s.emitNoLocalAuthEvent(req.Username)
		return nil, trace.AccessDenied(noLocalAuth)
	}

	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.AuthenticateUser(req.AuthenticateUserRequest); err != nil {
		return nil, trace.Wrap(err)
	}

	// It's safe to extract the roles and traits directly from services.User as
	// this endpoint is only used for local accounts.
	user, err := s.GetUser(req.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), s, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the host CA for this cluster only.
	authority, err := s.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostCertAuthorities := []types.CertAuthority{
		authority,
	}

	certs, err := s.generateUserCert(certRequest{
		user:              user,
		ttl:               req.TTL,
		publicKey:         req.PublicKey,
		compatibility:     req.CompatibilityMode,
		checker:           checker,
		traits:            user.GetTraits(),
		routeToCluster:    req.RouteToCluster,
		kubernetesCluster: req.KubernetesCluster,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	UserLoginCount.Inc()
	return &SSHLoginResponse{
		Username:    req.Username,
		Cert:        certs.SSH,
		TLSCert:     certs.TLS,
		HostSigners: AuthoritiesToTrustedCerts(hostCertAuthorities),
	}, nil
}

// emitNoLocalAuthEvent creates and emits a local authentication is disabled message.
func (s *Server) emitNoLocalAuthEvent(username string) {
	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.AuthAttempt{
		Metadata: apievents.Metadata{
			Type: events.AuthAttemptEvent,
			Code: events.AuthAttemptFailureCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: username,
		},
		Status: apievents.Status{
			Success: false,
			Error:   noLocalAuth,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit no local auth event.")
	}
}

func (s *Server) createUserWebSession(ctx context.Context, user types.User) (types.WebSession, error) {
	// It's safe to extract the roles and traits directly from services.User as this method
	// is only used for local accounts.
	return s.createWebSession(ctx, types.NewWebSessionRequest{
		User:      user.GetName(),
		Roles:     user.GetRoles(),
		Traits:    user.GetTraits(),
		LoginTime: s.clock.Now().UTC(),
	})
}

const noLocalAuth = "local auth disabled"
