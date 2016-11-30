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
// * presence service that takes care of heratbeats
// * web service that takes care of web logins
// * ca service - certificate authorities
package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gokyle/hotp"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// User represents teleport or external user
type User interface {
	// GetName returns user name
	GetName() string
	// GetAllowedLogins returns user's allowed linux logins
	GetAllowedLogins() []string
	// GetIdentities returns a list of connected OIDCIdentities
	GetIdentities() []OIDCIdentity
	// String returns user
	String() string
	// Check checks if all parameters are correct
	Check() error
	// Equals checks if user equals to another
	Equals(other User) bool
	// WebSessionInfo returns web session information
	WebSessionInfo() User
}

// TeleportUser is an optional user entry in the database
type TeleportUser struct {
	// Name is a user name
	Name string `json:"name"`

	// AllowedLogins represents a list of OS users this teleport
	// user is allowed to login as
	AllowedLogins []string `json:"allowed_logins"`

	// OIDCIdentities lists associated OpenID Connect identities
	// that let user log in using externally verified identity
	OIDCIdentities []OIDCIdentity `json:"oidc_identities"`
}

// Equals checks if user equals to another
func (u *TeleportUser) Equals(other User) bool {
	if u.Name != other.GetName() {
		return false
	}
	otherLogins := other.GetAllowedLogins()
	if len(u.AllowedLogins) != len(otherLogins) {
		return false
	}
	for i := range u.AllowedLogins {
		if u.AllowedLogins[i] != otherLogins[i] {
			return false
		}
	}
	otherIdentities := other.GetIdentities()
	if len(u.OIDCIdentities) != len(otherIdentities) {
		return false
	}
	for i := range u.OIDCIdentities {
		if !u.OIDCIdentities[i].Equals(&otherIdentities[i]) {
			return false
		}
	}
	return true
}

// WebSessionInfo returns web session information
func (u *TeleportUser) WebSessionInfo() User {
	return u
}

// GetAllowedLogins returns user's allowed linux logins
func (u *TeleportUser) GetAllowedLogins() []string {
	return u.AllowedLogins
}

// GetIdentities returns a list of connected OIDCIdentities
func (u *TeleportUser) GetIdentities() []OIDCIdentity {
	return u.OIDCIdentities
}

// GetName returns user name
func (u *TeleportUser) GetName() string {
	return u.Name
}

func (u *TeleportUser) String() string {
	return fmt.Sprintf("User(name=%v, allowed_logins=%v, identities=%v)", u.Name, u.AllowedLogins, u.OIDCIdentities)
}

// Check checks validity of all parameters
func (u *TeleportUser) Check() error {
	if !cstrings.IsValidUnixUser(u.Name) {
		return trace.BadParameter("'%v' is not a valid user name", u.Name)
	}
	if len(u.AllowedLogins) == 0 {
		return trace.BadParameter("'%v' has no valid allowed logins", u.Name)
	}
	for _, login := range u.AllowedLogins {
		if !cstrings.IsValidUnixUser(login) {
			return trace.BadParameter("'%v' is not a valid user name", login)
		}
	}
	for _, id := range u.OIDCIdentities {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Identity is responsible for managing user entries
type Identity interface {
	// GetUsers returns a list of users registered with the local auth server
	GetUsers() ([]User, error)

	// UpsertUser updates parameters about user
	UpsertUser(user User) error

	// GetUser returns a user by name
	GetUser(user string) (User, error)

	// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
	// user specified with this identity
	GetUserByOIDCIdentity(id OIDCIdentity) (User, error)

	// DeleteUser deletes a user with all the keys from the backend
	DeleteUser(user string) error

	// UpsertPasswordHash upserts user password hash
	UpsertPasswordHash(user string, hash []byte) error

	// GetPasswordHash returns the password hash for a given user
	GetPasswordHash(user string) ([]byte, error)

	// UpsertHOTP upserts HOTP state for user
	UpsertHOTP(user string, otp *hotp.HOTP) error

	// GetHOTP gets HOTP token state for a user
	GetHOTP(user string) (*hotp.HOTP, error)

	// UpsertWebSession updates or inserts a web session for a user and session id
	UpsertWebSession(user, sid string, session WebSession, ttl time.Duration) error

	// GetWebSession returns a web session state for a given user and session id
	GetWebSession(user, sid string) (*WebSession, error)

	// DeleteWebSession deletes web session from the storage
	DeleteWebSession(user, sid string) error

	// UpsertPassword upserts new password and HOTP token
	UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error)

	// CheckPassword is called on web user or tsh user login
	CheckPassword(user string, password []byte, hotpToken string) error

	// CheckPasswordWOToken checks just password without checking HOTP tokens
	// used in case of SSH authentication, when token has been validated
	CheckPasswordWOToken(user string, password []byte) error

	// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
	UpsertSignupToken(token string, tokenData SignupToken, ttl time.Duration) error

	// GetSignupToken returns signup token data
	GetSignupToken(token string) (*SignupToken, error)

	// GetSignupTokens returns a list of signup tokens
	GetSignupTokens() ([]SignupToken, error)

	// DeleteSignupToken deletes signup token from the storage
	DeleteSignupToken(token string) error

	// UpsertOIDCConnector upserts OIDC Connector
	UpsertOIDCConnector(connector OIDCConnector, ttl time.Duration) error

	// DeleteOIDCConnector deletes OIDC Connector
	DeleteOIDCConnector(connectorID string) error

	// GetOIDCConnector returns OIDC connector data, , withSecrets adds or removes client secret from return results
	GetOIDCConnector(id string, withSecrets bool) (*OIDCConnector, error)

	// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
	GetOIDCConnectors(withSecrets bool) ([]OIDCConnector, error)

	// CreateOIDCAuthRequest creates new auth request
	CreateOIDCAuthRequest(req OIDCAuthRequest, ttl time.Duration) error

	// GetOIDCAuthRequest returns OIDC auth request if found
	GetOIDCAuthRequest(stateToken string) (*OIDCAuthRequest, error)
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

// WebSession stores key and value used to authenticate with SSH
// notes on behalf of user
type WebSession struct {
	// Pub is a public certificate signed by auth server
	Pub []byte `json:"pub"`
	// Priv is a private OpenSSH key used to auth with SSH nodes
	Priv []byte `json:"priv"`
	// BearerToken is a special bearer token used for additional
	// bearer authentication
	BearerToken string `json:"bearer_token"`
	// Expires - absolute time when token expires
	Expires time.Time `json:"expires"`
}

// SignupToken stores metadata about user signup token
// is stored and generated when tctl add user is executed
type SignupToken struct {
	Token           string       `json:"token"`
	User            TeleportUser `json:"user"`
	Hotp            []byte       `json:"hotp"`
	HotpFirstValues []string     `json:"hotp_first_values"`
	HotpQR          []byte       `json:"hotp_qr"`
	Expires         time.Time    `json:"expires"`
}

// OIDCConnector specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type OIDCConnector struct {
	// ID is a provider id, 'e.g.' google, used internally
	ID string `json:"id"`
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `json:"issuer_url"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	ClientID string `json:"client_id"`
	// ClientSecret is used to authenticate our client and should not
	// be visible to end user
	ClientSecret string `json:"client_secret"`
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successfull authentication
	// Should match the URL on Provider's side
	RedirectURL string `json:"redirect_url"`
	// Display - Friendly name for this provider.
	Display string `json:"display"`
}

// Check returns nil if all parameters are great, err otherwise
func (o *OIDCConnector) Check() error {
	if o.ID == "" {
		return trace.BadParameter("ID: missing connector id")
	}
	if _, err := url.Parse(o.IssuerURL); err != nil {
		return trace.BadParameter("IssuerURL: bad url: '%v'", o.IssuerURL)
	}
	if _, err := url.Parse(o.RedirectURL); err != nil {
		return trace.BadParameter("RedirectURL: bad url: '%v'", o.RedirectURL)
	}
	if o.ClientID == "" {
		return trace.BadParameter("ClientID: missing client id")
	}
	if o.ClientSecret == "" {
		return trace.BadParameter("ClientSecret: missing client secret")
	}
	return nil
}

// OIDCIdentity is OpenID Connect identity that is linked
// to particular user and connector and lets user to log in using external
// credentials, e.g. google
type OIDCIdentity struct {
	// ConnectorID is id of registered OIDC connector, e.g. 'google-example.com'
	ConnectorID string `json:"connector_id"`

	// Email is OIDC verified email claim
	// e.g. bob@example.com
	Email string `json:"username"`
}

// String returns debug friendly representation of this identity
func (i *OIDCIdentity) String() string {
	return fmt.Sprintf("OIDCIdentity(connectorID=%v, email=%v)", i.ConnectorID, i.Email)
}

// Equals returns true if this identity equals to passed one
func (i *OIDCIdentity) Equals(other *OIDCIdentity) bool {
	return i.ConnectorID == other.ConnectorID && i.Email == other.Email
}

// Check returns nil if all parameters are great, err otherwise
func (i *OIDCIdentity) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if i.Email == "" {
		return trace.BadParameter("Email: missing email")
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

	// RedirectURL will be used by browser
	RedirectURL string `json:"redirect_url"`

	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successfull auth
	PublicKey []byte `json:"public_key"`

	// CertTTL is the TTL of the certificate user wants to get
	CertTTL time.Duration `json:"cert_ttl"`

	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication
	CreateWebSession bool `json:"create_web_session"`

	// ClientRedirectURL is a URL client wants to be redirected
	// after successfull authentication
	ClientRedirectURL string `json:"client_redirect_url"`
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

var mtx sync.Mutex
var unmarshaler UserUnmarshaler

func SetUserUnmarshaler(u UserUnmarshaler) {
	mtx.Lock()
	defer mtx.Unlock()
	unmarshaler = u
}

func GetUserUnmarshaler() UserUnmarshaler {
	mtx.Lock()
	defer mtx.Unlock()
	if unmarshaler == nil {
		return TeleportUserUnmarshaler
	}
	return unmarshaler
}

type UserUnmarshaler func(bytes []byte) (User, error)

func TeleportUserUnmarshaler(bytes []byte) (User, error) {
	var u *TeleportUser
	err := json.Unmarshal(bytes, &u)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}
