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
	"context"
	"errors"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// maxUserAgentLen is the maximum length of a user agent that will be logged.
	// There is no current consensus on what the maximum length of a User-Agent
	// should be and there were reports of extremely large UAs especially from
	// older versions of IE. 2048 was picked because it still allowed for very
	// large UAs but keeps from causing logging issues. For reference Nginx
	// defaults to 4k or 8k header size limits for ALL headers so 2k seems more
	// than sufficient.
	maxUserAgentLen = 2048
)

// AuthenticateUserRequest is a request to authenticate interactive user
type AuthenticateUserRequest struct {
	// Username is a username
	Username string `json:"username"`
	// PublicKey is a public key in ssh authorized_keys format
	PublicKey []byte `json:"public_key"`
	// Pass is a password used in local authentication schemes
	Pass *PassCreds `json:"pass,omitempty"`
	// Webauthn is a signed credential assertion, used in MFA authentication
	Webauthn *wanlib.CredentialAssertionResponse `json:"webauthn,omitempty"`
	// OTP is a password and second factor, used for MFA authentication
	OTP *OTPCreds `json:"otp,omitempty"`
	// Session is a web session credential used to authenticate web sessions
	Session *SessionCreds `json:"session,omitempty"`
	// ClientMetadata includes forwarded information about a client
	ClientMetadata *ForwardedClientMetadata `json:"client_metadata,omitempty"`
	// HeadlessAuthenticationID is the ID for a headless authentication resource.
	HeadlessAuthenticationID string `json:"headless_authentication_id"`
}

// ForwardedClientMetadata can be used by the proxy web API to forward information about
// the client to the auth service.
type ForwardedClientMetadata struct {
	UserAgent string `json:"user_agent,omitempty"`
	// RemoteAddr is the IP address of the end user. This IP address is derived
	// either from a direct client connection, or from a PROXY protocol header
	// if the connection is forwarded through a load balancer.
	RemoteAddr string `json:"remote_addr,omitempty"`
}

// CheckAndSetDefaults checks and sets defaults
func (a *AuthenticateUserRequest) CheckAndSetDefaults() error {
	switch {
	case a.Username == "" && a.Webauthn != nil: // OK, passwordless.
	case a.Username == "":
		return trace.BadParameter("missing parameter 'username'")
	case a.Pass == nil && a.Webauthn == nil && a.OTP == nil && a.Session == nil && a.HeadlessAuthenticationID == "":
		return trace.BadParameter("at least one authentication method is required")
	}
	return nil
}

// PassCreds is a password credential
type PassCreds struct {
	// Password is a user password
	Password []byte `json:"password"`
}

// OTPCreds is a two-factor authentication credentials
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

// AuthenticateUser authenticates user based on the request type.
// Returns the username of the authenticated user.
func (s *Server) AuthenticateUser(ctx context.Context, req AuthenticateUserRequest) (string, error) {
	user := req.Username

	mfaDev, actualUser, err := s.authenticateUser(ctx, req)
	// err is handled below.
	switch {
	case user != "" && actualUser != "" && user != actualUser:
		log.Warnf("Authenticate user mismatch (%q vs %q). Using request user (%q)", user, actualUser, user)
	case user == "" && actualUser != "":
		log.Debugf("User %q authenticated via passwordless", actualUser)
		user = actualUser
	}

	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserLocalLoginFailureCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: user,
		},
		Method: events.LoginMethodLocal,
	}
	if mfaDev != nil {
		m := mfaDeviceEventMetadata(mfaDev)
		event.MFADevice = &m
	}
	if req.ClientMetadata != nil {
		event.RemoteAddr = req.ClientMetadata.RemoteAddr
		if len(req.ClientMetadata.UserAgent) > maxUserAgentLen {
			event.UserAgent = req.ClientMetadata.UserAgent[:maxUserAgentLen-3] + "..."
		} else {
			event.UserAgent = req.ClientMetadata.UserAgent
		}
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
	return user, trace.Wrap(err)
}

var (
	// authenticateHeadlessError is the generic error returned for failed headless
	// authentication attempts.
	authenticateHeadlessError = trace.AccessDenied("headless authentication failed")
	// authenticateWebauthnError is the generic error returned for failed WebAuthn
	// authentication attempts.
	authenticateWebauthnError = trace.AccessDenied("invalid Webauthn response")
	// invalidUserPassError is the error for when either the provided username or
	// password is incorrect.
	invalidUserPassError = trace.AccessDenied("invalid username or password")
	// invalidUserpass2FError is the error for when either the provided username,
	// password, or second factor is incorrect.
	invalidUserPass2FError = trace.AccessDenied("invalid username, password or second factor")
)

// IsInvalidLocalCredentialError checks if an error resulted from an incorrect username,
// password, or second factor.
func IsInvalidLocalCredentialError(err error) bool {
	return errors.Is(err, invalidUserPassError) || errors.Is(err, invalidUserPass2FError)
}

// authenticateUser authenticates a user through various methods (password, MFA,
// passwordless)
// Returns the device used to authenticate (if applicable) and the username.
func (s *Server) authenticateUser(ctx context.Context, req AuthenticateUserRequest) (*types.MFADevice, string, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, "", trace.Wrap(err)
	}
	user := req.Username
	passwordless := user == ""

	// Only one path if passwordless, other variants shouldn't see an empty user.
	if passwordless {
		return s.authenticatePasswordless(ctx, req)
	}

	// Try 2nd-factor-enabled authentication schemes first.
	var authenticateFn func() (*types.MFADevice, error)
	var authErr error // error message kept obscure on purpose, use logging for details
	switch {
	// cases in order of preference
	case req.HeadlessAuthenticationID != "":
		// handle authentication before the user lock to prevent locking out users
		// due to timed-out/canceled headless authentication attempts.
		mfaDevice, err := s.authenticateHeadless(ctx, req)
		if err != nil {
			log.Debugf("Headless Authentication for user %q failed while waiting for approval: %v", user, err)
			return nil, "", trace.Wrap(authenticateHeadlessError)
		}
		authenticateFn = func() (*types.MFADevice, error) {
			return mfaDevice, nil
		}
		authErr = authenticateHeadlessError
	case req.Webauthn != nil:
		authenticateFn = func() (*types.MFADevice, error) {
			mfaResponse := &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: wanlib.CredentialAssertionResponseToProto(req.Webauthn),
				},
			}
			dev, _, err := s.validateMFAAuthResponse(ctx, mfaResponse, user, passwordless)
			return dev, trace.Wrap(err)
		}
		authErr = authenticateWebauthnError
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
		authErr = invalidUserPass2FError
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
			if fieldErr := getErrorByTraceField(err); fieldErr != nil {
				return nil, "", trace.Wrap(fieldErr)
			}

			return nil, "", trace.Wrap(authErr)
		case dev == nil:
			log.Debugf(
				"MFA authentication returned nil device (Webauthn = %v, TOTP = %v, Headless = %v): %v.",
				req.Webauthn != nil, req.OTP != nil, req.HeadlessAuthenticationID != "", err)
			return nil, "", trace.Wrap(authErr)
		default:
			return dev, user, nil
		}
	}

	// Try password-only authentication last.
	if req.Pass == nil {
		return nil, "", trace.AccessDenied("unsupported authentication method")
	}

	authPreference, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// When using password only make sure that auth preference does not require
	// second factor, otherwise users could bypass it.
	switch authPreference.GetSecondFactor() {
	case constants.SecondFactorOff:
		// No 2FA required, check password only.
	case constants.SecondFactorOptional:
		// 2FA is optional. Make sure that a user does not have MFA devices
		// registered.
		devs, err := s.Services.GetMFADevices(ctx, user, false /* withSecrets */)
		if err != nil && !trace.IsNotFound(err) {
			return nil, "", trace.Wrap(err)
		}
		if len(devs) != 0 {
			log.Warningf("MFA bypass attempt by user %q, access denied.", user)
			return nil, "", trace.AccessDenied("missing second factor authentication")
		}
	default:
		// Some form of MFA is required but none provided. Either client is
		// buggy (didn't send MFA response) or someone is trying to bypass
		// MFA.
		log.Warningf("MFA bypass attempt by user %q, access denied.", user)
		return nil, "", trace.AccessDenied("missing second factor")
	}
	if err = s.WithUserLock(user, func() error {
		return s.checkPasswordWOToken(user, req.Pass.Password)
	}); err != nil {
		if fieldErr := getErrorByTraceField(err); fieldErr != nil {
			return nil, "", trace.Wrap(fieldErr)
		}
		// provide obscure message on purpose, while logging the real
		// error server side
		log.Debugf("User %v failed to authenticate: %v.", user, err)
		return nil, "", trace.Wrap(invalidUserPassError)
	}
	return nil, user, nil
}

func (s *Server) authenticatePasswordless(ctx context.Context, req AuthenticateUserRequest) (*types.MFADevice, string, error) {
	mfaResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wanlib.CredentialAssertionResponseToProto(req.Webauthn),
		},
	}
	dev, user, err := s.validateMFAAuthResponse(ctx, mfaResponse, "", true /* passwordless */)
	if err != nil {
		log.Debugf("Passwordless authentication failed: %v", err)
		return nil, "", trace.Wrap(authenticateWebauthnError)
	}

	// A distinction between passwordless and "plain" MFA is that we can't
	// acquire the user lock beforehand (or at all on failures!)
	// We do grab it here so successful logins go through the regular process.
	if err := s.WithUserLock(user, func() error { return nil }); err != nil {
		log.Debugf("WithUserLock for user %q failed during passwordless authentication: %v", user, err)
		return nil, user, trace.Wrap(authenticateWebauthnError)
	}

	return dev, user, nil
}

func (s *Server) authenticateHeadless(ctx context.Context, req AuthenticateUserRequest) (mfa *types.MFADevice, err error) {
	// Delete the headless authentication upon failure.
	defer func() {
		if err != nil {
			if err := s.DeleteHeadlessAuthentication(s.CloseContext(), req.HeadlessAuthenticationID); err != nil && !trace.IsNotFound(err) {
				log.Debugf("Failed to delete headless authentication: %v", err)
			}
		}
	}()

	// this authentication requires two client callbacks to create a headless authentication
	// stub and approve/deny the headless authentication, so we use a standard callback timeout.
	ctx, cancel := context.WithTimeout(ctx, defaults.CallbackTimeout)
	defer cancel()

	headlessAuthn := &types.HeadlessAuthentication{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: req.HeadlessAuthenticationID,
			},
		},
		User:            req.Username,
		PublicKey:       req.PublicKey,
		ClientIpAddress: req.ClientMetadata.RemoteAddr,
	}

	// Headless Authentication should expire when the callback expires.
	headlessAuthn.SetExpiry(s.clock.Now().Add(defaults.CallbackTimeout))

	if err := services.ValidateHeadlessAuthentication(headlessAuthn); err != nil {
		return nil, trace.Wrap(err)
	}

	// Wait for a headless authenticated stub to be inserted by an authenticated
	// call to GetHeadlessAuthentication. We do this to avoid immediately inserting
	// backend items from an unauthenticated endpoint.
	headlessAuthnStub, err := s.headlessAuthenticationWatcher.Wait(ctx, req.HeadlessAuthenticationID, func(ha *types.HeadlessAuthentication) (bool, error) {
		// Only headless authentication stub can be inserted without the standard validation.
		if services.ValidateHeadlessAuthentication(ha) == nil {
			return false, trace.AlreadyExists("headless auth request already exists")
		}
		return true, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Update headless authentication with login details.
	if _, err := s.CompareAndSwapHeadlessAuthentication(ctx, headlessAuthnStub, headlessAuthn); err != nil {
		return nil, trace.Wrap(err)
	}

	// Wait for the request to be approved/denied.
	headlessAuthn, err = s.headlessAuthenticationWatcher.Wait(ctx, req.HeadlessAuthenticationID, func(ha *types.HeadlessAuthentication) (bool, error) {
		switch ha.State {
		case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED:
			if ha.MfaDevice == nil {
				return false, trace.AccessDenied("expected mfa approval for headless authentication approval")
			}
			return true, nil
		case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED:
			return false, trace.AccessDenied("headless authentication denied")
		}
		return false, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify that the headless authentication has not been tampered with.
	if headlessAuthn.User != req.Username {
		return nil, trace.AccessDenied("user mismatch")
	}

	return headlessAuthn.MfaDevice, nil
}

// AuthenticateWebUser authenticates web user, creates and returns a web session
// if authentication is successful. In case the existing session ID is used to authenticate,
// returns the existing session instead of creating a new one
func (s *Server) AuthenticateWebUser(ctx context.Context, req AuthenticateUserRequest) (types.WebSession, error) {
	username := req.Username // Empty if passwordless.

	authPref, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Disable all local auth requests,
	// except session ID renewal requests that are using the same method.
	// This condition uses Session as a blanket check, because any new method added
	// to the local auth will be disabled by default.
	if !authPref.GetAllowLocalAuth() && req.Session == nil {
		s.emitNoLocalAuthEvent(username)
		return nil, trace.AccessDenied(noLocalAuth)
	}

	if req.Session != nil {
		session, err := s.GetWebSession(ctx, types.GetWebSessionRequest{
			User:      username,
			SessionID: req.Session.ID,
		})
		if err != nil {
			return nil, trace.AccessDenied("session is invalid or has expired")
		}
		return session, nil
	}

	actualUser, err := s.AuthenticateUser(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username = actualUser

	user, err := s.GetUser(username, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.createUserWebSession(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

// AuthenticateSSHRequest is a request to authenticate SSH client user via CLI
type AuthenticateSSHRequest struct {
	// AuthenticateUserRequest is a request with credentials
	AuthenticateUserRequest
	// TTL is a requested TTL for certificates to be issues
	TTL time.Duration `json:"ttl"`
	// CompatibilityMode sets certificate compatibility mode with old SSH clients
	CompatibilityMode string `json:"compatibility_mode"`
	RouteToCluster    string `json:"route_to_cluster"`
	// KubernetesCluster sets the target kubernetes cluster for the TLS
	// certificate. This can be empty on older clients.
	KubernetesCluster string `json:"kubernetes_cluster"`
	// AttestationStatement is an attestation statement associated with the given public key.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
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
	// User contains a logged-in user information
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
	// AuthorizedKeys is a list of SSH public keys in authorized_keys format
	// that can be used to check host key signatures.
	AuthorizedKeys [][]byte `json:"checking_keys"`
	// TLSCertificates is a list of TLS certificates of the certificate authority
	// of the authentication server
	TLSCertificates [][]byte `json:"tls_certs"`
}

// SSHCertPublicKeys returns a list of trusted host SSH certificate authority public keys
func (c *TrustedCerts) SSHCertPublicKeys() ([]ssh.PublicKey, error) {
	out := make([]ssh.PublicKey, 0, len(c.AuthorizedKeys))
	for _, keyBytes := range c.AuthorizedKeys {
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
			ClusterName:     ca.GetClusterName(),
			AuthorizedKeys:  services.GetSSHCheckingKeys(ca),
			TLSCertificates: services.GetTLSCerts(ca),
		}
	}
	return out
}

// AuthenticateSSHUser authenticates an SSH user and returns SSH and TLS
// certificates for the public key in req.
func (s *Server) AuthenticateSSHUser(ctx context.Context, req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	username := req.Username // Empty if passwordless.

	authPref, err := s.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authPref.GetAllowLocalAuth() {
		s.emitNoLocalAuthEvent(username)
		return nil, trace.AccessDenied(noLocalAuth)
	}

	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actualUser, err := s.AuthenticateUser(ctx, req.AuthenticateUserRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username = actualUser

	// It's safe to extract the roles and traits directly from services.User as
	// this endpoint is only used for local accounts.
	user, err := s.GetUser(username, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessInfo := services.AccessInfoFromUser(user)
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the host CA for this cluster only.
	authority, err := s.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostCertAuthorities := []types.CertAuthority{
		authority,
	}

	clientIP := ""
	if req.ClientMetadata != nil && req.ClientMetadata.RemoteAddr != "" {
		host, err := utils.Host(req.ClientMetadata.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clientIP = host
	}
	if checker.PinSourceIP() && clientIP == "" {
		return nil, trace.BadParameter("source IP pinning is enabled but client IP is unknown")
	}

	certReq := certRequest{
		user:                 user,
		ttl:                  req.TTL,
		publicKey:            req.PublicKey,
		compatibility:        req.CompatibilityMode,
		checker:              checker,
		traits:               user.GetTraits(),
		routeToCluster:       req.RouteToCluster,
		kubernetesCluster:    req.KubernetesCluster,
		loginIP:              clientIP,
		attestationStatement: req.AttestationStatement,
	}

	// For headless authentication, a short-lived mfa-verified cert should be generated.
	if req.HeadlessAuthenticationID != "" {
		ha, err := s.GetHeadlessAuthentication(ctx, req.HeadlessAuthenticationID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !bytes.Equal(req.PublicKey, ha.PublicKey) {
			return nil, trace.AccessDenied("headless authentication public key mismatch")
		}
		certReq.mfaVerified = ha.MfaDevice.Metadata.Name
		certReq.ttl = time.Minute
	}

	certs, err := s.generateUserCert(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	UserLoginCount.Inc()
	return &SSHLoginResponse{
		Username:    username,
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
	return s.CreateWebSessionFromReq(ctx, types.NewWebSessionRequest{
		User:      user.GetName(),
		Roles:     user.GetRoles(),
		Traits:    user.GetTraits(),
		LoginTime: s.clock.Now().UTC(),
	})
}

func getErrorByTraceField(err error) error {
	traceErr, ok := err.(trace.Error)
	switch {
	case !ok:
		log.WithError(err).Warn("Unexpected error type, wanted TraceError")
		return trace.AccessDenied("an error has occurred")
	case traceErr.GetFields()[ErrFieldKeyUserMaxedAttempts] != nil:
		return trace.AccessDenied(MaxFailedAttemptsErrMsg)
	}

	return nil
}

const noLocalAuth = "local auth disabled"
