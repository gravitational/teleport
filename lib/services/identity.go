/*
Copyright 2015 Gravitational, Inc.

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

// Package services implements API services exposed by Teleport:
// * presence service that takes care of heartbeats
// * web service that takes care of web logins
// * ca service - certificate authorities
package services

import (
	"context"
	"crypto"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
)

// UserGetter is responsible for getting users
type UserGetter interface {
	// GetUser returns a user by name
	GetUser(user string, withSecrets bool) (types.User, error)
}

// UsersService is responsible for basic user management
type UsersService interface {
	UserGetter
	// UpdateUser updates an existing user.
	UpdateUser(ctx context.Context, user types.User) error
	// UpdateAndSwapUser reads an existing user, runs `fn` against it and writes
	// the result to storage. Return `false` from `fn` to avoid storage changes.
	// Roughly equivalent to [GetUser] followed by [CompareAndSwapUser].
	// Returns the storage user.
	UpdateAndSwapUser(ctx context.Context, user string, withSecrets bool, fn func(types.User) (changed bool, err error)) (types.User, error)
	// UpsertUser updates parameters about user
	UpsertUser(user types.User) error
	// CompareAndSwapUser updates an existing user, but fails if the user does
	// not match an expected backend value.
	CompareAndSwapUser(ctx context.Context, new, existing types.User) error
	// DeleteUser deletes a user with all the keys from the backend
	DeleteUser(ctx context.Context, user string) error
	// GetUsers returns a list of users registered with the local auth server
	GetUsers(withSecrets bool) ([]types.User, error)
	// DeleteAllUsers deletes all users
	DeleteAllUsers() error
}

// Identity is responsible for managing user entries and external identities
type Identity interface {
	// CreateUser creates user, only if the user entry does not exist
	CreateUser(user types.User) error

	// UsersService implements most methods
	UsersService

	// AddUserLoginAttempt logs user login attempt
	AddUserLoginAttempt(user string, attempt LoginAttempt, ttl time.Duration) error

	// GetUserLoginAttempts returns user login attempts
	GetUserLoginAttempts(user string) ([]LoginAttempt, error)

	// DeleteUserLoginAttempts removes all login attempts of a user. Should be
	// called after successful login.
	DeleteUserLoginAttempts(user string) error

	// GetUserByOIDCIdentity returns a user by its specified OIDC Identity, returns first
	// user specified with this identity
	GetUserByOIDCIdentity(id types.ExternalIdentity) (types.User, error)

	// GetUserBySAMLIdentity returns a user by its specified OIDC Identity, returns first
	// user specified with this identity
	GetUserBySAMLIdentity(id types.ExternalIdentity) (types.User, error)

	// GetUserByGithubIdentity returns a user by its specified Github identity
	GetUserByGithubIdentity(id types.ExternalIdentity) (types.User, error)

	// UpsertPasswordHash upserts user password hash
	UpsertPasswordHash(user string, hash []byte) error

	// GetPasswordHash returns the password hash for a given user
	GetPasswordHash(user string) ([]byte, error)

	// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
	// during the 30 second window it's valid.
	UpsertUsedTOTPToken(user string, otpToken string) error

	// GetUsedTOTPToken returns the last successfully used TOTP token.
	GetUsedTOTPToken(user string) (string, error)

	// UpsertPassword upserts new password and OTP token
	UpsertPassword(user string, password []byte) error

	// UpsertWebauthnLocalAuth creates or updates the local auth configuration for
	// Webauthn.
	// WebauthnLocalAuth is a component of LocalAuthSecrets.
	// Automatically indexes the WebAuthn user ID for lookup by
	// GetTeleportUserByWebauthnID.
	UpsertWebauthnLocalAuth(ctx context.Context, user string, wla *types.WebauthnLocalAuth) error

	// GetWebauthnLocalAuth retrieves the existing local auth configuration for
	// Webauthn, if any.
	// WebauthnLocalAuth is a component of LocalAuthSecrets.
	GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error)

	// GetTeleportUserByWebauthnID reads a Teleport username from a WebAuthn user
	// ID (aka user handle).
	// See UpsertWebauthnLocalAuth and types.WebauthnLocalAuth.
	GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error)

	// UpsertWebauthnSessionData creates or updates WebAuthn session data in
	// storage, for the purpose of later verifying an authentication or
	// registration challenge.
	// Session data is expected to expire according to backend settings.
	UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error

	// GetWebauthnSessionData retrieves a previously-stored session data by ID,
	// if it exists and has not expired.
	GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error)

	// DeleteWebauthnSessionData deletes session data by ID, if it exists and has
	// not expired.
	DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error

	// UpsertGlobalWebauthnSessionData creates or updates WebAuthn session data in
	// storage, for the purpose of later verifying an authentication challenge.
	// Session data is expected to expire according to backend settings.
	// Used for passwordless challenges.
	UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wantypes.SessionData) error

	// GetGlobalWebauthnSessionData retrieves previously-stored session data by ID,
	// if it exists and has not expired.
	// Used for passwordless challenges.
	GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wantypes.SessionData, error)

	// DeleteGlobalWebauthnSessionData deletes session data by ID, if it exists
	// and has not expired.
	DeleteGlobalWebauthnSessionData(ctx context.Context, scope, id string) error

	// UpsertMFADevice upserts an MFA device for the user.
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error

	// GetMFADevices gets all MFA devices for the user.
	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)

	// DeleteMFADevice deletes an MFA device for the user by ID.
	DeleteMFADevice(ctx context.Context, user, id string) error

	// UpsertOIDCConnector upserts OIDC Connector
	UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error

	// DeleteOIDCConnector deletes OIDC Connector
	DeleteOIDCConnector(ctx context.Context, connectorID string) error

	// GetOIDCConnector returns OIDC connector data, withSecrets adds or removes client secret from return results
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (types.OIDCConnector, error)

	// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)

	// CreateOIDCAuthRequest creates new auth request
	CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest, ttl time.Duration) error

	// GetOIDCAuthRequest returns OIDC auth request if found
	GetOIDCAuthRequest(ctx context.Context, stateToken string) (*types.OIDCAuthRequest, error)

	// UpsertSAMLConnector upserts SAML Connector
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error

	// DeleteSAMLConnector deletes OIDC Connector
	DeleteSAMLConnector(ctx context.Context, connectorID string) error

	// GetSAMLConnector returns OIDC connector data, withSecrets adds or removes secrets from return results
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)

	// GetSAMLConnectors returns registered connectors, withSecrets adds or removes secret from return results
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)

	// CreateSAMLAuthRequest creates new auth request
	CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest, ttl time.Duration) error

	// GetSAMLAuthRequest returns SAML auth request if found
	GetSAMLAuthRequest(ctx context.Context, id string) (*types.SAMLAuthRequest, error)

	// CreateSSODiagnosticInfo creates new SSO diagnostic info record.
	CreateSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string, entry types.SSODiagnosticInfo) error

	// GetSSODiagnosticInfo returns SSO diagnostic info records.
	GetSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string) (*types.SSODiagnosticInfo, error)

	// UpsertGithubConnector creates or updates a new Github connector
	UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error

	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)

	// GetGithubConnector returns a Github connector by its name
	GetGithubConnector(ctx context.Context, name string, withSecrets bool) (types.GithubConnector, error)

	// DeleteGithubConnector deletes a Github connector by its name
	DeleteGithubConnector(ctx context.Context, name string) error

	// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
	CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) error

	// GetGithubAuthRequest retrieves Github auth request by the token
	GetGithubAuthRequest(ctx context.Context, stateToken string) (*types.GithubAuthRequest, error)

	// CreateUserToken creates a new user token.
	CreateUserToken(ctx context.Context, token types.UserToken) (types.UserToken, error)

	// DeleteUserToken deletes a user token.
	DeleteUserToken(ctx context.Context, tokenID string) error

	// GetUserTokens returns all user tokens.
	GetUserTokens(ctx context.Context) ([]types.UserToken, error)

	// GetUserToken returns a user token by id.
	GetUserToken(ctx context.Context, tokenID string) (types.UserToken, error)

	// UpsertUserTokenSecrets upserts a user token secrets.
	UpsertUserTokenSecrets(ctx context.Context, secrets types.UserTokenSecrets) error

	// GetUserTokenSecrets returns a user token secrets.
	GetUserTokenSecrets(ctx context.Context, tokenID string) (types.UserTokenSecrets, error)

	// UpsertRecoveryCodes upserts a user's new recovery codes.
	UpsertRecoveryCodes(ctx context.Context, user string, recovery *types.RecoveryCodesV1) error

	// GetRecoveryCodes gets a user's recovery codes.
	GetRecoveryCodes(ctx context.Context, user string, withSecrets bool) (*types.RecoveryCodesV1, error)

	// CreateUserRecoveryAttempt logs user recovery attempt.
	CreateUserRecoveryAttempt(ctx context.Context, user string, attempt *types.RecoveryAttempt) error

	// GetUserRecoveryAttempts returns user recovery attempts sorted by oldest to latest time.
	GetUserRecoveryAttempts(ctx context.Context, user string) ([]*types.RecoveryAttempt, error)

	// DeleteUserRecoveryAttempts removes all recovery attempts of a user.
	DeleteUserRecoveryAttempts(ctx context.Context, user string) error

	// UpsertKeyAttestationData upserts a verified public key attestation response.
	UpsertKeyAttestationData(ctx context.Context, attestationData *keys.AttestationData, ttl time.Duration) error

	// GetKeyAttestationData gets a verified public key attestation response.
	GetKeyAttestationData(ctx context.Context, publicKey crypto.PublicKey) (*keys.AttestationData, error)

	HeadlessAuthenticationService

	types.WebSessionsGetter
	types.WebTokensGetter

	// AppSession defines application session features.
	AppSession
	// SnowflakeSession defines Snowflake session features.
	SnowflakeSession
	// SAMLIdPSession defines SAML IdP session features.
	SAMLIdPSession
}

// AppSession defines application session features.
type AppSession interface {
	// GetAppSession gets an application web session.
	GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error)
	// GetAppSessions gets all application web sessions.
	GetAppSessions(context.Context) ([]types.WebSession, error)
	// ListAppSessions gets a paginated list of application web sessions.
	ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error)
	// UpsertAppSession upserts an application web session.
	UpsertAppSession(context.Context, types.WebSession) error
	// DeleteAppSession removes an application web session.
	DeleteAppSession(context.Context, types.DeleteAppSessionRequest) error
	// DeleteAllAppSessions removes all application web sessions.
	DeleteAllAppSessions(context.Context) error
	// DeleteUserAppSessions deletes all user’s application sessions.
	DeleteUserAppSessions(ctx context.Context, req *proto.DeleteUserAppSessionsRequest) error
}

// SnowflakeSession defines Snowflake session features.
type SnowflakeSession interface {
	// GetSnowflakeSession gets a Snowflake web session.
	GetSnowflakeSession(context.Context, types.GetSnowflakeSessionRequest) (types.WebSession, error)
	// GetSnowflakeSessions gets all Snowflake web sessions.
	GetSnowflakeSessions(context.Context) ([]types.WebSession, error)
	// UpsertSnowflakeSession upserts a Snowflake web session.
	UpsertSnowflakeSession(context.Context, types.WebSession) error
	// DeleteSnowflakeSession removes a Snowflake web session.
	DeleteSnowflakeSession(context.Context, types.DeleteSnowflakeSessionRequest) error
	// DeleteAllSnowflakeSessions removes all Snowflake web sessions.
	DeleteAllSnowflakeSessions(context.Context) error
}

// SAMLIdPSession defines SAML IdP session features.
type SAMLIdPSession interface {
	// GetSAMLIdPSession gets a SAML IdP session.
	GetSAMLIdPSession(context.Context, types.GetSAMLIdPSessionRequest) (types.WebSession, error)
	// ListSAMLIdPSessions gets a paginated list of SAML IdP sessions.
	ListSAMLIdPSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error)
	// UpsertSAMLIdPSession upserts a SAML IdP session.
	UpsertSAMLIdPSession(context.Context, types.WebSession) error
	// DeleteSAMLIdPSession removes a SAML IdP session.
	DeleteSAMLIdPSession(context.Context, types.DeleteSAMLIdPSessionRequest) error
	// DeleteAllSAMLIdPSessions removes all SAML IdP sessions.
	DeleteAllSAMLIdPSessions(context.Context) error
	// DeleteUserSAMLIdPSessions deletes all of a user's SAML IdP sessions.
	DeleteUserSAMLIdPSessions(ctx context.Context, user string) error
}

// HeadlessAuthenticationService is responsible for headless authentication resource management
type HeadlessAuthenticationService interface {
	// GetHeadlessAuthentication gets a headless authentication.
	GetHeadlessAuthentication(ctx context.Context, username, name string) (*types.HeadlessAuthentication, error)

	// UpsertHeadlessAuthentication upserts a headless authentication.
	UpsertHeadlessAuthentication(ctx context.Context, ha *types.HeadlessAuthentication) error

	// CompareAndSwapHeadlessAuthentication performs a compare
	// and swap replacement on a headless authentication resource.
	CompareAndSwapHeadlessAuthentication(ctx context.Context, old, new *types.HeadlessAuthentication) (*types.HeadlessAuthentication, error)

	// DeleteHeadlessAuthentication deletes a headless authentication from the backend.
	DeleteHeadlessAuthentication(ctx context.Context, username, name string) error
}

// VerifyPassword makes sure password satisfies our requirements (relaxed),
// mostly to avoid putting garbage in
func VerifyPassword(password []byte) error {
	if len(password) < defaults.MinPasswordLength {
		return trace.BadParameter(
			"password is too short, min length is %v", defaults.MinPasswordLength)
	}
	if len(password) > defaults.MaxPasswordLength {
		return trace.BadParameter(
			"password is too long, max length is %v", defaults.MaxPasswordLength)
	}
	return nil
}

// Users represents a slice of users,
// makes it sort compatible (sorts by username)
type Users []types.User

func (u Users) Len() int {
	return len(u)
}

func (u Users) Less(i, j int) bool {
	return u[i].GetName() < u[j].GetName()
}

func (u Users) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

// SortedLoginAttempts sorts login attempts by time
type SortedLoginAttempts []LoginAttempt

// Len returns length of a role list
func (s SortedLoginAttempts) Len() int {
	return len(s)
}

// Less stacks latest attempts to the end of the list
func (s SortedLoginAttempts) Less(i, j int) bool {
	return s[i].Time.Before(s[j].Time)
}

// Swap swaps two attempts
func (s SortedLoginAttempts) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// LastFailed calculates last x successive attempts are failed
func LastFailed(x int, attempts []LoginAttempt) bool {
	var failed int
	for i := len(attempts) - 1; i >= 0; i-- {
		if !attempts[i].Success {
			failed++
		} else {
			return false
		}
		if failed >= x {
			return true
		}
	}
	return false
}
