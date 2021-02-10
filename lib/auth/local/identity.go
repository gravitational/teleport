/*
Copyright 2021 Gravitational, Inc.

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

package local

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"

	"github.com/gokyle/hotp"
)

// Identity is responsible for managing user entries and external identities
type Identity interface {
	auth.UsersService

	AppSession
	WebSessionsGetter
	WebTokensGetter

	// CreateUser creates user, only if the user entry does not exist
	CreateUser(user types.User) error

	// AddUserLoginAttempt logs user login attempt
	AddUserLoginAttempt(user string, attempt auth.LoginAttempt, ttl time.Duration) error

	// GetUserLoginAttempts returns user login attempts
	GetUserLoginAttempts(user string) ([]auth.LoginAttempt, error)

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

	// UpsertHOTP upserts HOTP state for user
	// Deprecated: HOTP use is deprecated, use UpsertTOTP instead.
	UpsertHOTP(user string, otp *hotp.HOTP) error

	// GetHOTP gets HOTP token state for a user
	// Deprecated: HOTP use is deprecated, use GetTOTP instead.
	GetHOTP(user string) (*hotp.HOTP, error)

	// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
	// during the 30 second window it's valid.
	UpsertUsedTOTPToken(user string, otpToken string) error

	// GetUsedTOTPToken returns the last successfully used TOTP token.
	GetUsedTOTPToken(user string) (string, error)

	// UpsertPassword upserts new password and OTP token
	UpsertPassword(user string, password []byte) error

	// UpsertU2FRegisterChallenge upserts a U2F challenge for a new user corresponding to the token
	UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error

	// GetU2FRegisterChallenge returns a U2F challenge for a new user corresponding to the token
	GetU2FRegisterChallenge(token string) (*u2f.Challenge, error)

	// UpsertU2FSignChallenge upserts a U2F sign (auth) challenge
	UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error

	// GetU2FSignChallenge returns a U2F sign (auth) challenge
	GetU2FSignChallenge(user string) (*u2f.Challenge, error)

	// UpsertMFADevice upserts an MFA device for the user.
	UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error

	// GetMFADevices gets all MFA devices for the user.
	GetMFADevices(ctx context.Context, user string) ([]*types.MFADevice, error)

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
	CreateOIDCAuthRequest(req auth.OIDCAuthRequest, ttl time.Duration) error

	// GetOIDCAuthRequest returns OIDC auth request if found
	GetOIDCAuthRequest(stateToken string) (*auth.OIDCAuthRequest, error)

	// CreateSAMLConnector creates SAML Connector
	CreateSAMLConnector(connector types.SAMLConnector) error

	// UpsertSAMLConnector upserts SAML Connector
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error

	// DeleteSAMLConnector deletes OIDC Connector
	DeleteSAMLConnector(ctx context.Context, connectorID string) error

	// GetSAMLConnector returns OIDC connector data, withSecrets adds or removes secrets from return results
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)

	// GetSAMLConnectors returns registered connectors, withSecrets adds or removes secret from return results
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)

	// CreateSAMLAuthRequest creates new auth request
	CreateSAMLAuthRequest(req auth.SAMLAuthRequest, ttl time.Duration) error

	// GetSAMLAuthRequest returns OSAML auth request if found
	GetSAMLAuthRequest(id string) (*auth.SAMLAuthRequest, error)

	// CreateGithubConnector creates a new Github connector
	CreateGithubConnector(connector types.GithubConnector) error

	// UpsertGithubConnector creates or updates a new Github connector
	UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error

	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)

	// GetGithubConnector returns a Github connector by its name
	GetGithubConnector(ctx context.Context, name string, withSecrets bool) (types.GithubConnector, error)

	// DeleteGithubConnector deletes a Github connector by its name
	DeleteGithubConnector(ctx context.Context, name string) error

	// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
	CreateGithubAuthRequest(req auth.GithubAuthRequest) error

	// GetGithubAuthRequest retrieves Github auth request by the token
	GetGithubAuthRequest(stateToken string) (*auth.GithubAuthRequest, error)

	// CreateResetPasswordToken creates a token
	CreateResetPasswordToken(ctx context.Context, resetPasswordToken types.ResetPasswordToken) (types.ResetPasswordToken, error)

	// DeleteResetPasswordToken deletes a token
	DeleteResetPasswordToken(ctx context.Context, tokenID string) error

	// GetResetPasswordTokens returns tokens
	GetResetPasswordTokens(ctx context.Context) ([]types.ResetPasswordToken, error)

	// GetResetPasswordToken returns a token
	GetResetPasswordToken(ctx context.Context, tokenID string) (types.ResetPasswordToken, error)

	// UpsertResetPasswordTokenSecrets upserts token secrets
	UpsertResetPasswordTokenSecrets(ctx context.Context, secrets types.ResetPasswordTokenSecrets) error

	// GetResetPasswordTokenSecrets returns token secrets
	GetResetPasswordTokenSecrets(ctx context.Context, tokenID string) (types.ResetPasswordTokenSecrets, error)
}
