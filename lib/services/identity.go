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
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/ssh"
)

// UserGetter is responsible for getting users
type UserGetter interface {
	// GetUser returns a user by name
	GetUser(user string, withSecrets bool) (User, error)
}

// UserService is reponsible for basic user management
type UsersService interface {
	UserGetter
	// UpsertUser updates parameters about user
	UpsertUser(user User) error
	// DeleteUser deletes a user with all the keys from the backend
	DeleteUser(user string) error
	// GetUsers returns a list of users registered with the local auth server
	GetUsers(withSecrets bool) ([]User, error)
	// DeleteAllUsers deletes all users
	DeleteAllUsers() error
}

// Identity is responsible for managing user entries and external identities
type Identity interface {
	// CreateUser creates user, only if the user entry does not exist
	CreateUser(user User) error

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
	GetUserByOIDCIdentity(id ExternalIdentity) (User, error)

	// GetUserBySAMLIdentity returns a user by its specified OIDC Identity, returns first
	// user specified with this identity
	GetUserBySAMLIdentity(id ExternalIdentity) (User, error)

	// GetUserByGithubIdentity returns a user by its specified Github identity
	GetUserByGithubIdentity(id ExternalIdentity) (User, error)

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

	// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
	UpsertTOTP(user string, secretKey string) error

	// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens.
	GetTOTP(user string) (string, error)

	// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
	// during the 30 second window it's valid.
	UpsertUsedTOTPToken(user string, otpToken string) error

	// GetUsedTOTPToken returns the last successfully used TOTP token.
	GetUsedTOTPToken(user string) (string, error)

	// DeleteUsedTOTPToken removes the used token from the backend. This should only
	// be used during tests.
	DeleteUsedTOTPToken(user string) error

	// UpsertWebSession updates or inserts a web session for a user and session
	UpsertWebSession(user, sid string, session WebSession) error

	// GetWebSession returns a web session state for a given user and session id
	GetWebSession(user, sid string) (WebSession, error)

	// DeleteWebSession deletes web session from the storage
	DeleteWebSession(user, sid string) error

	// UpsertPassword upserts new password and OTP token
	UpsertPassword(user string, password []byte) error

	// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
	UpsertSignupToken(token string, tokenData SignupToken, ttl time.Duration) error

	// GetSignupToken returns signup token data
	GetSignupToken(token string) (*SignupToken, error)

	// GetSignupTokens returns a list of signup tokens
	GetSignupTokens() ([]SignupToken, error)

	// DeleteSignupToken deletes signup token from the storage
	DeleteSignupToken(token string) error

	// UpsertU2FRegisterChallenge upserts a U2F challenge for a new user corresponding to the token
	UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error

	// GetU2FRegisterChallenge returns a U2F challenge for a new user corresponding to the token
	GetU2FRegisterChallenge(token string) (*u2f.Challenge, error)

	// UpsertU2FRegistration upserts a U2F registration from a valid register response
	UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error

	// GetU2FRegistration returns a U2F registration from a valid register response
	GetU2FRegistration(user string) (*u2f.Registration, error)

	// UpsertU2FSignChallenge upserts a U2F sign (auth) challenge
	UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error

	// GetU2FSignChallenge returns a U2F sign (auth) challenge
	GetU2FSignChallenge(user string) (*u2f.Challenge, error)

	// UpsertU2FRegistrationCounter upserts a counter associated with a U2F registration
	UpsertU2FRegistrationCounter(user string, counter uint32) error

	// GetU2FRegistrationCounter returns a counter associated with a U2F registration
	GetU2FRegistrationCounter(user string) (uint32, error)

	// UpsertOIDCConnector upserts OIDC Connector
	UpsertOIDCConnector(connector OIDCConnector) error

	// DeleteOIDCConnector deletes OIDC Connector
	DeleteOIDCConnector(connectorID string) error

	// GetOIDCConnector returns OIDC connector data, withSecrets adds or removes client secret from return results
	GetOIDCConnector(id string, withSecrets bool) (OIDCConnector, error)

	// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
	GetOIDCConnectors(withSecrets bool) ([]OIDCConnector, error)

	// CreateOIDCAuthRequest creates new auth request
	CreateOIDCAuthRequest(req OIDCAuthRequest, ttl time.Duration) error

	// GetOIDCAuthRequest returns OIDC auth request if found
	GetOIDCAuthRequest(stateToken string) (*OIDCAuthRequest, error)

	// CreateSAMLConnector creates SAML Connector
	CreateSAMLConnector(connector SAMLConnector) error

	// UpsertSAMLConnector upserts SAML Connector
	UpsertSAMLConnector(connector SAMLConnector) error

	// DeleteSAMLConnector deletes OIDC Connector
	DeleteSAMLConnector(connectorID string) error

	// GetSAMLConnector returns OIDC connector data, withSecrets adds or removes secrets from return results
	GetSAMLConnector(id string, withSecrets bool) (SAMLConnector, error)

	// GetSAMLConnectors returns registered connectors, withSecrets adds or removes secret from return results
	GetSAMLConnectors(withSecrets bool) ([]SAMLConnector, error)

	// CreateSAMLAuthRequest creates new auth request
	CreateSAMLAuthRequest(req SAMLAuthRequest, ttl time.Duration) error

	// GetSAMLAuthRequest returns OSAML auth request if found
	GetSAMLAuthRequest(id string) (*SAMLAuthRequest, error)

	// CreateGithubConnector creates a new Github connector
	CreateGithubConnector(connector GithubConnector) error
	// UpsertGithubConnector creates or updates a new Github connector
	UpsertGithubConnector(connector GithubConnector) error
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(withSecrets bool) ([]GithubConnector, error)
	// GetGithubConnector returns a Github connector by its name
	GetGithubConnector(name string, withSecrets bool) (GithubConnector, error)
	// DeleteGithubConnector deletes a Github connector by its name
	DeleteGithubConnector(name string) error
	// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
	CreateGithubAuthRequest(req GithubAuthRequest) error
	// GetGithubAuthRequest retrieves Github auth request by the token
	GetGithubAuthRequest(stateToken string) (*GithubAuthRequest, error)
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

// SignupToken stores metadata about user signup token
// is stored and generated when tctl add user is executed
type SignupToken struct {
	Token     string    `json:"token"`
	User      UserV1    `json:"user"`
	OTPKey    string    `json:"otp_key"`
	OTPQRCode []byte    `json:"otp_qr_code"`
	Expires   time.Time `json:"expires"`
}

const ExternalIdentitySchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
     "connector_id": {"type": "string"},
     "username": {"type": "string"}
   }
}`

// String returns debug friendly representation of this identity
func (i *ExternalIdentity) String() string {
	return fmt.Sprintf("OIDCIdentity(connectorID=%v, username=%v)", i.ConnectorID, i.Username)
}

// Equals returns true if this identity equals to passed one
func (i *ExternalIdentity) Equals(other *ExternalIdentity) bool {
	return i.ConnectorID == other.ConnectorID && i.Username == other.Username
}

// Check returns nil if all parameters are great, err otherwise
func (i *ExternalIdentity) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if i.Username == "" {
		return trace.BadParameter("Username: missing username")
	}
	return nil
}

// GithubAuthRequest is the request to start Github OAuth2 flow
type GithubAuthRequest struct {
	// ConnectorID is the name of the connector to use
	ConnectorID string `json:"connector_id"`
	// Type is opaque string that helps callbacks identify the request type
	Type string `json:"type"`
	// StateToken is used to validate the request
	StateToken string `json:"state_token"`
	// CSRFToken is used to protect against CSRF attacks
	CSRFToken string `json:"csrf_token"`
	// PublicKey is an optional public key to sign in case of successful auth
	PublicKey []byte `json:"public_key"`
	// CertTTL is TTL of the cert that's generated in case of successful auth
	CertTTL time.Duration `json:"cert_ttl"`
	// CreateWebSession indicates that a user wants to generate a web session
	// after successul authentication
	CreateWebSession bool `json:"create_web_session"`
	// RedirectURL will be used by browser
	RedirectURL string `json:"redirect_url"`
	// ClientRedirectURL is the URL where client will be redirected after
	// successful auth
	ClientRedirectURL string `json:"client_redirect_url"`
	// Compatibility specifies OpenSSH compatibility flags
	Compatibility string `json:"compatibility,omitempty"`
	// Expires is a global expiry time header can be set on any resource in the system.
	Expires *time.Time `json:"expires,omitempty"`
}

// SetTTL sets Expires header using realtime clock
func (r *GithubAuthRequest) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	expireTime := clock.Now().UTC().Add(ttl)
	r.Expires = &expireTime
}

// SetExpiry sets expiry time for the object
func (r *GithubAuthRequest) SetExpiry(expires time.Time) {
	r.Expires = &expires
}

// Expires returns object expiry setting.
func (r *GithubAuthRequest) Expiry() time.Time {
	if r.Expires == nil {
		return time.Time{}
	}
	return *r.Expires
}

// Check makes sure the request is valid
func (r *GithubAuthRequest) Check() error {
	if r.ConnectorID == "" {
		return trace.BadParameter("missing ConnectorID")
	}
	if r.StateToken == "" {
		return trace.BadParameter("missing StateToken")
	}
	if len(r.PublicKey) != 0 {
		_, _, _, _, err := ssh.ParseAuthorizedKey(r.PublicKey)
		if err != nil {
			return trace.BadParameter("bad PublicKey: %v", err)
		}
		if (r.CertTTL > defaults.MaxCertDuration) || (r.CertTTL < defaults.MinCertDuration) {
			return trace.BadParameter("wrong CertTTL")
		}
	}
	return nil
}

// OIDCAuthRequest is a request to authenticate with OIDC
// provider, the state about request is managed by auth server
type OIDCAuthRequest struct {
	// ConnectorID is ID of OIDC connector this request uses
	ConnectorID string `json:"connector_id"`

	// Type is opaque string that helps callbacks identify the request type
	Type string `json:"type"`

	// CheckUser tells validator if it should expect and check user
	CheckUser bool `json:"check_user"`

	// StateToken is generated by service and is used to validate
	// reuqest coming from
	StateToken string `json:"state_token"`

	// CSRFToken is associated with user web session token
	CSRFToken string `json:"csrf_token"`

	// RedirectURL will be used by browser
	RedirectURL string `json:"redirect_url"`

	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successful auth
	PublicKey []byte `json:"public_key"`

	// CertTTL is the TTL of the certificate user wants to get
	CertTTL time.Duration `json:"cert_ttl"`

	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication
	CreateWebSession bool `json:"create_web_session"`

	// ClientRedirectURL is a URL client wants to be redirected
	// after successful authentication
	ClientRedirectURL string `json:"client_redirect_url"`

	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
}

// Check returns nil if all parameters are great, err otherwise
func (i *OIDCAuthRequest) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if i.StateToken == "" {
		return trace.BadParameter("StateToken: missing value")
	}
	if len(i.PublicKey) != 0 {
		_, _, _, _, err := ssh.ParseAuthorizedKey(i.PublicKey)
		if err != nil {
			return trace.BadParameter("PublicKey: bad key: %v", err)
		}
		if (i.CertTTL > defaults.MaxCertDuration) || (i.CertTTL < defaults.MinCertDuration) {
			return trace.BadParameter("CertTTL: wrong certificate TTL")
		}
	}

	return nil
}

// SAMLAuthRequest is a request to authenticate with OIDC
// provider, the state about request is managed by auth server
type SAMLAuthRequest struct {
	// ID is a unique request ID
	ID string `json:"id"`

	// ConnectorID is ID of OIDC connector this request uses
	ConnectorID string `json:"connector_id"`

	// Type is opaque string that helps callbacks identify the request type
	Type string `json:"type"`

	// CheckUser tells validator if it should expect and check user
	CheckUser bool `json:"check_user"`

	// RedirectURL will be used by browser
	RedirectURL string `json:"redirect_url"`

	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successful auth
	PublicKey []byte `json:"public_key"`

	// CertTTL is the TTL of the certificate user wants to get
	CertTTL time.Duration `json:"cert_ttl"`

	// CSRFToken is associated with user web session token
	CSRFToken string `json:"csrf_token"`

	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication
	CreateWebSession bool `json:"create_web_session"`

	// ClientRedirectURL is a URL client wants to be redirected
	// after successful authentication
	ClientRedirectURL string `json:"client_redirect_url"`

	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
}

// Check returns nil if all parameters are great, err otherwise
func (i *SAMLAuthRequest) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if len(i.PublicKey) != 0 {
		_, _, _, _, err := ssh.ParseAuthorizedKey(i.PublicKey)
		if err != nil {
			return trace.BadParameter("PublicKey: bad key: %v", err)
		}
		if (i.CertTTL > defaults.MaxCertDuration) || (i.CertTTL < defaults.MinCertDuration) {
			return trace.BadParameter("CertTTL: wrong certificate TTL")
		}
	}

	return nil
}

// Users represents a slice of users,
// makes it sort compatible (sorts by username)
type Users []User

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
