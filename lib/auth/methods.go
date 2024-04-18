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

package auth

import (
	"bytes"
	"context"
	"errors"
	"net"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
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
	Webauthn *wantypes.CredentialAssertionResponse `json:"webauthn,omitempty"`
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
func (a *Server) AuthenticateUser(ctx context.Context, req AuthenticateUserRequest) (services.UserState, services.AccessChecker, error) {
	username := req.Username

	requiredExt := mfav1.ChallengeExtensions{
		Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
	}
	verifyMFALocks, mfaDev, actualUsername, err := a.authenticateUser(
		ctx, req, requiredExt)
	if err != nil {
		// Log event after authentication failure
		if err := a.emitAuthAuditEvent(ctx, authAuditProps{
			username:       req.Username,
			clientMetadata: req.ClientMetadata,
			authErr:        err,
		}); err != nil {
			log.WithError(err).Warn("Failed to emit login event.")
		}
		return nil, nil, trace.Wrap(err)
	}

	switch {
	case username != "" && actualUsername != "" && username != actualUsername:
		log.Warnf("Authenticate user mismatch (%q vs %q). Using request user (%q)", username, actualUsername, username)
	case username == "" && actualUsername != "":
		log.Debugf("User %q authenticated via passwordless", actualUsername)
		username = actualUsername
	}

	user, err := a.GetUser(ctx, username, false /* withSecrets */)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// After we're sure that the user has been logged in successfully, we should call
	// the registered login hooks. Login hooks can be registered by other processes to
	// execute arbitrary operations after a successful login.
	if err := a.CallLoginHooks(ctx, user); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	userState, err := a.GetUserOrLoginState(ctx, user.GetName())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	accessInfo := services.AccessInfoFromUserState(userState)
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Verify if the MFA device is locked.
	if err := verifyMFALocks(verifyMFADeviceLocksParams{
		Checker: checker,
	}); err != nil {
		// Log MFA lock failure as an authn failure.
		if err := a.emitAuthAuditEvent(ctx, authAuditProps{
			username:       req.Username,
			clientMetadata: req.ClientMetadata,
			mfaDevice:      mfaDev,
			checker:        checker,
			authErr:        err,
		}); err != nil {
			log.WithError(err).Warn("Failed to emit login event.")
		}
		return nil, nil, trace.Wrap(err)
	}

	// Log event after authentication success
	if err := a.emitAuthAuditEvent(ctx, authAuditProps{
		username:       username,
		clientMetadata: req.ClientMetadata,
		mfaDevice:      mfaDev,
		checker:        checker,
	}); err != nil {
		log.WithError(err).Warn("Failed to emit login event.")
	}

	return userState, checker, trace.Wrap(err)
}

type authAuditProps struct {
	username       string
	clientMetadata *ForwardedClientMetadata
	mfaDevice      *types.MFADevice
	checker        services.AccessChecker
	authErr        error
}

func (a *Server) emitAuthAuditEvent(ctx context.Context, props authAuditProps) error {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserLocalLoginCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		UserMetadata: apievents.UserMetadata{
			User: props.username,
		},
		Method: events.LoginMethodLocal,
	}

	if props.authErr != nil {
		event.Code = events.UserLocalLoginFailureCode
		event.Status.Success = false
		event.Status.Error = props.authErr.Error()
	}

	if props.clientMetadata != nil {
		event.RemoteAddr = props.clientMetadata.RemoteAddr
		event.UserAgent = trimUserAgent(props.clientMetadata.UserAgent)
	}

	if props.mfaDevice != nil {
		m := mfaDeviceEventMetadata(props.mfaDevice)
		event.MFADevice = &m
	}

	// Add required key policy to the event.
	if props.checker != nil {
		authPref, err := a.GetAuthPreference(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		privateKeyPolicy, err := props.checker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
		if err != nil {
			return trace.Wrap(err)
		}
		event.RequiredPrivateKeyPolicy = string(privateKeyPolicy)
	}

	return trace.Wrap(a.emitter.EmitAuditEvent(a.closeCtx, event))
}

var (
	// authenticateHeadlessError is the generic error returned for failed headless
	// authentication attempts.
	authenticateHeadlessError = trace.AccessDenied("headless authentication failed")
	// authenticateWebauthnError is the generic error returned for failed WebAuthn
	// authentication attempts.
	authenticateWebauthnError = trace.AccessDenied("invalid credentials")
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

type verifyMFADeviceLocksParams struct {
	// Checker used to verify locks.
	// Optional, created via a [UserState] fetch if nil.
	Checker services.AccessChecker

	// ClusterLockingMode used to verify locks.
	// Optional, acquired from [Server.GetAuthPreference] if nil.
	ClusterLockingMode constants.LockingMode
}

// authenticateUser authenticates a user through various methods (password, MFA,
// passwordless). Note that "authenticate" may mean different things, depending
// on the request and required extensions. The caller is responsible for making
// sure that the request and extensions correctly describe the conditions that
// are being asserted during the authentication process. One important caveat is
// that if the request contains a WebAuthn challenge response, the user name is
// known, and `authenticateUser` is called without a password, the function only
// performs a second factor check; in this case, the caller must provide
// appropriate required extensions to validate the WebAuthn challenge response.
//
// The `requiredExt` parameter is used to assert that the MFA challenge has a
// correct extensions. It is ignored for passwordless authentication, where we
// override it with an extension that has scope set to
// `mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN`.
//
// Returns a callback to verify MFA device locks, the MFA device used to
// authenticate (if applicable), and the authenticated user name.
//
// Callers MUST call the verifyLocks callback.
func (a *Server) authenticateUser(
	ctx context.Context,
	req AuthenticateUserRequest,
	requiredExt mfav1.ChallengeExtensions,
) (verifyLocks func(verifyMFADeviceLocksParams) error, mfaDev *types.MFADevice, user string, err error) {
	mfaDev, user, err = a.authenticateUserInternal(ctx, req, requiredExt)
	if err != nil || mfaDev == nil {
		return func(verifyMFADeviceLocksParams) error { return nil }, mfaDev, user, trace.Wrap(err)
	}

	verifyLocks = func(p verifyMFADeviceLocksParams) error {
		if p.Checker == nil {
			userState, err := a.GetUserOrLoginState(ctx, user)
			if err != nil {
				return trace.Wrap(err)
			}
			accessInfo := services.AccessInfoFromUserState(userState)
			clusterName, err := a.GetClusterName()
			if err != nil {
				return trace.Wrap(err)
			}
			checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
			if err != nil {
				return trace.Wrap(err)
			}
			p.Checker = checker
		}

		if p.ClusterLockingMode == "" {
			authPref, err := a.GetAuthPreference(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			p.ClusterLockingMode = authPref.GetLockingMode()
		}

		// The MFA device needs to be explicitly verified, as it won't be verified
		// as part of certificate issuance in various scenarios (password change,
		// non-session certificates, etc)
		return a.verifyLocksForUserCerts(verifyLocksForUserCertsReq{
			checker:     p.Checker,
			defaultMode: p.ClusterLockingMode,
			username:    user,
			mfaVerified: mfaDev.Id,
		})
	}
	return verifyLocks, mfaDev, user, nil
}

// Do not use this method directly, use authenticateUser instead.
func (a *Server) authenticateUserInternal(
	ctx context.Context,
	req AuthenticateUserRequest,
	requiredExt mfav1.ChallengeExtensions,
) (mfaDev *types.MFADevice, user string, err error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, "", trace.Wrap(err)
	}
	user = req.Username
	passwordless := user == ""

	// Only one path if passwordless, other variants shouldn't see an empty user.
	if passwordless {
		if requiredExt.Scope != mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN {
			return nil, "", trace.BadParameter(
				"passwordless authentication can only be used with the PASSWORDLESS scope")
		}
		return a.authenticatePasswordless(ctx, req)
	}

	// Try 2nd-factor-enabled authentication schemes first.
	var authenticateFn func() (*types.MFADevice, error)
	var authErr error // error message kept obscure on purpose, use logging for details
	switch {
	// cases in order of preference
	case req.HeadlessAuthenticationID != "":
		// handle authentication before the user lock to prevent locking out users
		// due to timed-out/canceled headless authentication attempts.
		mfaDevice, err := a.authenticateHeadless(ctx, req)
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
			if req.Pass != nil {
				if err = a.checkPasswordWOToken(user, req.Pass.Password); err != nil {
					return nil, trace.Wrap(err)
				}
			}
			mfaResponse := &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: wantypes.CredentialAssertionResponseToProto(req.Webauthn),
				},
			}
			mfaData, err := a.ValidateMFAAuthResponse(ctx, mfaResponse, user, &requiredExt)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return mfaData.Device, nil
		}
		authErr = authenticateWebauthnError
	case req.OTP != nil:
		authenticateFn = func() (*types.MFADevice, error) {
			// OTP cannot be validated by validateMFAAuthResponse because we need to
			// check the user's password too.
			res, err := a.checkPassword(user, req.OTP.Password, req.OTP.Token)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return res.mfaDev, nil
		}
		authErr = invalidUserPass2FError
	}
	if authenticateFn != nil {
		err := a.WithUserLock(ctx, user, func() error {
			var err error
			mfaDev, err = authenticateFn()
			return err
		})
		switch {
		case err != nil:
			log.Debugf("User %v failed to authenticate: %v.", user, err)
			if fieldErr := getErrorByTraceField(err); fieldErr != nil {
				return nil, "", trace.Wrap(fieldErr)
			}

			return nil, "", trace.Wrap(authErr)
		case mfaDev == nil:
			log.Debugf(
				"MFA authentication returned nil device (Webauthn = %v, TOTP = %v, Headless = %v): %v.",
				req.Webauthn != nil, req.OTP != nil, req.HeadlessAuthenticationID != "", err)
			return nil, "", trace.Wrap(authErr)
		default:
			return mfaDev, user, nil
		}
	}

	// Try password-only authentication last.
	if req.Pass == nil {
		return nil, "", trace.AccessDenied("unsupported authentication method")
	}

	authPreference, err := a.GetAuthPreference(ctx)
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
		devs, err := a.Services.GetMFADevices(ctx, user, false /* withSecrets */)
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
	if err = a.WithUserLock(ctx, user, func() error {
		return a.checkPasswordWOToken(user, req.Pass.Password)
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

func (a *Server) authenticatePasswordless(ctx context.Context, req AuthenticateUserRequest) (*types.MFADevice, string, error) {
	mfaResponse := &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(req.Webauthn),
		},
	}

	requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN}
	mfaData, err := a.ValidateMFAAuthResponse(ctx, mfaResponse, "" /* user */, requiredExt)
	if err != nil {
		log.Debugf("Passwordless authentication failed: %v", err)
		return nil, "", trace.Wrap(authenticateWebauthnError)
	}

	// A distinction between passwordless and "plain" MFA is that we can't
	// acquire the user lock beforehand (or at all on failures!)
	// We do grab it here so successful logins go through the regular process.
	if err := a.WithUserLock(ctx, mfaData.User, func() error { return nil }); err != nil {
		log.Debugf("WithUserLock for user %q failed during passwordless authentication: %v", mfaData.User, err)
		return nil, mfaData.User, trace.Wrap(authenticateWebauthnError)
	}

	return mfaData.Device, mfaData.User, nil
}

func (a *Server) authenticateHeadless(ctx context.Context, req AuthenticateUserRequest) (mfa *types.MFADevice, err error) {
	// Delete the headless authentication upon failure.
	defer func() {
		if err != nil {
			if err := a.DeleteHeadlessAuthentication(a.CloseContext(), req.Username, req.HeadlessAuthenticationID); err != nil && !trace.IsNotFound(err) {
				log.Debugf("Failed to delete headless authentication: %v", err)
			}
		}
	}()

	// this authentication requires two client callbacks to create a headless authentication
	// stub and approve/deny the headless authentication.
	ctx, cancel := context.WithTimeout(ctx, defaults.HeadlessLoginTimeout)
	defer cancel()

	// Headless Authentication should expire when the callback expires.
	expires := a.clock.Now().Add(defaults.HeadlessLoginTimeout)

	// Create the headless authentication and validate request details.
	ha, err := types.NewHeadlessAuthentication(req.Username, req.HeadlessAuthenticationID, expires)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
	ha.PublicKey = req.PublicKey
	ha.ClientIpAddress = req.ClientMetadata.RemoteAddr
	if err := services.ValidateHeadlessAuthentication(ha); err != nil {
		return nil, trace.Wrap(err)
	}

	emitHeadlessLoginEvent(ctx, events.UserHeadlessLoginRequestedCode, a.emitter, ha, nil)

	// HTTP server has shorter WriteTimeout than is needed, so we override WriteDeadline of the connection.
	if conn, err := authz.ConnFromContext(ctx); err == nil {
		if err := conn.SetWriteDeadline(a.GetClock().Now().Add(defaults.HeadlessLoginTimeout)); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Headless authentication requests are made without any prior authentication. To avoid DDos
	// attacks on the Auth server's backend, we don't create the headless authentication in the
	// backend until an authenticated client creates a headless authentication stub. This serves
	// as indirect authorization to insert the full headless authentication details into the backend.
	if _, err := a.waitForHeadlessStub(ctx, ha); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.UpsertHeadlessAuthentication(ctx, ha); err != nil {
		return nil, trace.Wrap(err)
	}

	// Wait for the request to be approved/denied.
	approvedHeadlessAuthn, err := a.waitForHeadlessApproval(ctx, req.Username, req.HeadlessAuthenticationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify that the headless authentication has not been tampered with.
	if approvedHeadlessAuthn.User != req.Username {
		return nil, trace.AccessDenied("headless authentication user mismatch")
	}
	if !bytes.Equal(req.PublicKey, ha.PublicKey) {
		return nil, trace.AccessDenied("headless authentication public key mismatch")
	}

	return approvedHeadlessAuthn.MfaDevice, nil
}

func (a *Server) waitForHeadlessStub(ctx context.Context, ha *types.HeadlessAuthentication) (*types.HeadlessAuthentication, error) {
	sub, err := a.headlessAuthenticationWatcher.Subscribe(ctx, ha.User, services.HeadlessAuthenticationUserStubID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer sub.Close()

	stub, err := sub.WaitForUpdate(ctx, func(ha *types.HeadlessAuthentication) (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return stub, nil
}

func (a *Server) waitForHeadlessApproval(ctx context.Context, username, reqID string) (*types.HeadlessAuthentication, error) {
	sub, err := a.headlessAuthenticationWatcher.Subscribe(ctx, username, reqID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer sub.Close()

	headlessAuthn, err := sub.WaitForUpdate(ctx, func(ha *types.HeadlessAuthentication) (bool, error) {
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

	return headlessAuthn, nil
}

// AuthenticateWebUser authenticates web user, creates and returns a web session
// if authentication is successful. In case the existing session ID is used to authenticate,
// returns the existing session instead of creating a new one
func (a *Server) AuthenticateWebUser(ctx context.Context, req AuthenticateUserRequest) (types.WebSession, error) {
	username := req.Username // Empty if passwordless.

	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Disable all local auth requests,
	// except session ID renewal requests that are using the same method.
	// This condition uses Session as a blanket check, because any new method added
	// to the local auth will be disabled by default.
	if !authPref.GetAllowLocalAuth() && req.Session == nil {
		a.emitNoLocalAuthEvent(username)
		return nil, trace.AccessDenied(noLocalAuth)
	}

	if req.Session != nil {
		session, err := a.GetWebSession(ctx, types.GetWebSessionRequest{
			User:      username,
			SessionID: req.Session.ID,
		})
		if err != nil {
			return nil, trace.AccessDenied("session is invalid or has expired")
		}
		return session, nil
	}

	user, _, err := a.AuthenticateUser(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var loginIP, userAgent string
	if cm := req.ClientMetadata; cm != nil {
		loginIP, _, err = net.SplitHostPort(cm.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userAgent = cm.UserAgent
	}

	sess, err := a.CreateWebSessionFromReq(ctx, NewWebSessionRequest{
		User:             user.GetName(),
		LoginIP:          loginIP,
		Roles:            user.GetRoles(),
		Traits:           user.GetTraits(),
		LoginTime:        a.clock.Now().UTC(),
		AttestWebSession: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the device trust DeviceWebToken.
	// We only get a token if the server is enabled for Device Trust and the user
	// has a suitable trusted device.
	if loginIP != "" && userAgent != "" {
		webToken, err := a.createDeviceWebToken(ctx, &devicepb.DeviceWebToken{
			WebSessionId:     sess.GetName(),
			BrowserUserAgent: userAgent,
			BrowserIp:        loginIP,
			User:             sess.GetUser(),
		})
		switch {
		case err != nil:
			log.WithError(err).Warn("Failed to create DeviceWebToken for user")
		case webToken != nil: // May be nil even if err==nil.
			sess.SetDeviceWebToken(&types.DeviceWebToken{
				Id:    webToken.Id,
				Token: webToken.Token,
			})
		}
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
func (a *Server) AuthenticateSSHUser(ctx context.Context, req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	username := req.Username // Empty if passwordless.

	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authPref.GetAllowLocalAuth() {
		a.emitNoLocalAuthEvent(username)
		return nil, trace.AccessDenied(noLocalAuth)
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// It's safe to extract the roles and traits directly from services.User as
	// this endpoint is only used for local accounts.
	user, checker, err := a.AuthenticateUser(ctx, req.AuthenticateUserRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return the host CA for this cluster only.
	authority, err := a.GetCertAuthority(ctx, types.CertAuthID{
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
		ha, err := a.GetHeadlessAuthentication(ctx, req.Username, req.HeadlessAuthenticationID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !bytes.Equal(req.PublicKey, ha.PublicKey) {
			return nil, trace.AccessDenied("headless authentication public key mismatch")
		}
		certReq.mfaVerified = ha.MfaDevice.Metadata.Name
		certReq.ttl = time.Minute
	}

	certs, err := a.generateUserCert(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	UserLoginCount.Inc()
	return &SSHLoginResponse{
		Username:    user.GetName(),
		Cert:        certs.SSH,
		TLSCert:     certs.TLS,
		HostSigners: AuthoritiesToTrustedCerts(hostCertAuthorities),
	}, nil
}

// emitNoLocalAuthEvent creates and emits a local authentication is disabled message.
func (a *Server) emitNoLocalAuthEvent(username string) {
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.AuthAttempt{
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

func (a *Server) createUserWebSession(ctx context.Context, user services.UserState, loginIP string) (types.WebSession, error) {
	// It's safe to extract the roles and traits directly from services.User as this method
	// is only used for local accounts.
	return a.CreateWebSessionFromReq(ctx, NewWebSessionRequest{
		User:      user.GetName(),
		LoginIP:   loginIP,
		Roles:     user.GetRoles(),
		Traits:    user.GetTraits(),
		LoginTime: a.clock.Now().UTC(),
	})
}

func getErrorByTraceField(err error) error {
	var traceErr trace.Error
	ok := errors.As(err, &traceErr)
	switch {
	case !ok:
		log.WithError(err).Warn("Unexpected error type, wanted TraceError")
		return trace.AccessDenied("an error has occurred")
	case traceErr.GetFields()[ErrFieldKeyUserMaxedAttempts] != nil:
		return trace.AccessDenied(MaxFailedAttemptsErrMsg)
	}

	return nil
}

func trimUserAgent(userAgent string) string {
	if len(userAgent) > maxUserAgentLen {
		return userAgent[:maxUserAgentLen-3] + "..."
	}
	return userAgent
}

const noLocalAuth = "local auth disabled"
