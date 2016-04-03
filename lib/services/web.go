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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gokyle/hotp"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
)

// User is an optional user entry in the database
type User struct {
	// Name is a user name
	Name string `json:"name"`

	// AllowedLogins represents a list of OS users this teleport
	// user is allowed to login as
	AllowedLogins []string `json:"allowed_logins"`

	// OIDCIdentities lists associated OpenID Connect identities
	// that let user log in using externally verified identity
	OIDCIdentities []OIDCIdentity `json:"oidc_identities"`
}

func (u *User) String() string {
	return fmt.Sprintf("User(name=%v, allowed_logins=%v, identities=%v)", u.Name, u.AllowedLogins, u.OIDCIdentities)
}

// Check checks validity of all parameters
func (u *User) Check() error {
	if !cstrings.IsValidUnixUser(u.Name) {
		return trace.Wrap(
			teleport.BadParameter("Name", fmt.Sprintf("'%v' is not a valid user name", u.Name)))
	}
	if len(u.AllowedLogins) == 0 {
		return trace.Wrap(teleport.BadParameter(
			"AllowedLogins", fmt.Sprintf("'%v' has no valid allowed logins", u.Name)))
	}
	for _, login := range u.AllowedLogins {
		if !cstrings.IsValidUnixUser(login) {
			return trace.Wrap(teleport.BadParameter(
				"login", fmt.Sprintf("'%v' is not a valid user name", login)))
		}
	}
	for _, id := range u.OIDCIdentities {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// AuthorizedKey is a public key that is authorized to access SSH
// servers
type AuthorizedKey struct {
	// ID is a unique key id
	ID string `json:"id"`
	// Value is a value of the public key
	Value []byte `json:"value"`
}

// IdentityService is responsible for managing web users and currently
// user accounts as well
type IdentityService struct {
	backend backend.Backend
}

// NewIdentityService returns new instance of WebService
func NewIdentityService(backend backend.Backend) *IdentityService {
	return &IdentityService{
		backend: backend,
	}
}

// GetUsers returns a list of users registered with the local auth server
func (s *IdentityService) GetUsers() ([]User, error) {
	keys, err := s.backend.GetKeys([]string{"web", "users"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]User, len(keys))
	for i, name := range keys {
		u, err := s.GetUser(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = *u
	}
	return out, nil
}

// UpsertUser updates parameters about user
func (s *IdentityService) UpsertUser(user User) error {
	if !cstrings.IsValidUnixUser(user.Name) {
		return trace.Wrap(
			teleport.BadParameter("user.Name", fmt.Sprintf("'%v is not a valid unix username'", user.Name)))
	}

	for _, l := range user.AllowedLogins {
		if !cstrings.IsValidUnixUser(l) {
			return trace.Wrap(
				teleport.BadParameter("login", fmt.Sprintf("'%v is not a valid unix username'", l)))
		}
	}
	for _, i := range user.OIDCIdentities {
		if err := i.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	data, err := json.Marshal(user)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal([]string{"web", "users", user.Name}, "params", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUser returns a user by name
func (s *IdentityService) GetUser(user string) (*User, error) {
	u := User{Name: user}
	data, err := s.backend.GetVal([]string{"web", "users", user}, "params")
	if err != nil {
		if teleport.IsNotFound(err) {
			return &u, nil
		}
		return nil, trace.Wrap(err)
	}
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, trace.Wrap(err)
	}
	return &u, nil
}

// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserByOIDCIdentity(id OIDCIdentity) (*User, error) {
	users, err := s.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.OIDCIdentities {
			if uid.Equals(&id) {
				return &u, nil
			}
		}
	}
	return nil, trace.Wrap(teleport.NotFound(fmt.Sprintf("user with identity %v not found", &id)))
}

// DeleteUser deletes a user with all the keys from the backend
func (s *IdentityService) DeleteUser(user string) error {
	err := s.backend.DeleteBucket([]string{"web", "users"}, user)
	if err != nil {
		if teleport.IsNotFound(err) {
			return trace.Wrap(teleport.NotFound(fmt.Sprintf("user '%v' is not found", user)))
		}
	}
	return trace.Wrap(err)
}

// UpsertPasswordHash upserts user password hash
func (s *IdentityService) UpsertPasswordHash(user string, hash []byte) error {
	err := s.backend.UpsertVal([]string{"web", "users", user}, "pwd", hash, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPasswordHash returns the password hash for a given user
func (s *IdentityService) GetPasswordHash(user string) ([]byte, error) {
	hash, err := s.backend.GetVal([]string{"web", "users", user}, "pwd")
	if err != nil {
		if teleport.IsNotFound(err) {
			return nil, trace.Wrap(teleport.NotFound(fmt.Sprintf("user '%v' is not found", user)))
		}
		return nil, trace.Wrap(err)
	}
	return hash, nil
}

// UpsertHOTP upserts HOTP state for user
func (s *IdentityService) UpsertHOTP(user string, otp *hotp.HOTP) error {
	bytes, err := hotp.Marshal(otp)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"web", "users", user},
		"hotp", bytes, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetHOTP gets HOTP token state for a user
func (s *IdentityService) GetHOTP(user string) (*hotp.HOTP, error) {
	bytes, err := s.backend.GetVal([]string{"web", "users", user},
		"hotp")
	if err != nil {
		if teleport.IsNotFound(err) {
			return nil, trace.Wrap(teleport.NotFound(fmt.Sprintf("user '%v' is not found", user)))
		}
		return nil, trace.Wrap(err)
	}
	otp, err := hotp.Unmarshal(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return otp, nil
}

// UpsertWebSession updates or inserts a web session for a user and session id
func (s *IdentityService) UpsertWebSession(user, sid string, session WebSession, ttl time.Duration) error {
	bytes, err := json.Marshal(session)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"web", "users", user, "sessions"},
		sid, bytes, ttl)
	if teleport.IsNotFound(err) {
		return trace.Wrap(teleport.NotFound(fmt.Sprintf("user '%v' is not found", user)))
	}
	return trace.Wrap(err)
}

// GetWebSession returns a web session state for a given user and session id
func (s *IdentityService) GetWebSession(user, sid string) (*WebSession, error) {
	val, err := s.backend.GetVal(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var session WebSession
	err = json.Unmarshal(val, &session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session, nil
}

// GetWebSessionsKeys returns public keys associated with the session
func (s *IdentityService) GetWebSessionsKeys(user string) ([]AuthorizedKey, error) {
	keys, err := s.backend.GetKeys([]string{"web", "users", user, "sessions"})
	if err != nil {
		return nil, err
	}

	values := make([]AuthorizedKey, len(keys))
	for i, key := range keys {
		session, err := s.GetWebSession(user, key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		values[i].Value = session.Pub
	}
	return values, nil
}

// DeleteWebSession deletes web session from the storage
func (s *IdentityService) DeleteWebSession(user, sid string) error {
	err := s.backend.DeleteKey(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	return err
}

// UpsertPassword upserts new password and HOTP token
func (s *IdentityService) UpsertPassword(user string,
	password []byte) (hotpURL string, hotpQR []byte, err error) {

	if err := verifyPassword(password); err != nil {
		return "", nil, err
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	otp, err := hotp.GenerateHOTP(defaults.HOTPTokenDigits, false)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	hotpQR, err = otp.QR(user)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	hotpURL = otp.URL(user)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	err = s.UpsertPasswordHash(user, hash)
	if err != nil {
		return "", nil, err
	}
	err = s.UpsertHOTP(user, otp)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	return hotpURL, hotpQR, nil

}

// CheckPassword is called on web user or tsh user login
func (s *IdentityService) CheckPassword(user string, password []byte, hotpToken string) error {
	if err := verifyPassword(password); err != nil {
		return trace.Wrap(err)
	}
	hash, err := s.GetPasswordHash(user)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		return trace.Wrap(teleport.BadParameter("password", "passwords do not match"))
	}
	otp, err := s.GetHOTP(user)
	if err != nil {
		return trace.Wrap(err)
	}
	if !otp.Scan(hotpToken, defaults.HOTPFirstTokensRange) {
		return trace.Wrap(teleport.BadParameter("token", "bad one time token"))
	}
	if err := s.UpsertHOTP(user, otp); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CheckPasswordWOToken checks just password without checking HOTP tokens
// used in case of SSH authentication, when token has been validated
func (s *IdentityService) CheckPasswordWOToken(user string, password []byte) error {
	if err := verifyPassword(password); err != nil {
		return trace.Wrap(err)
	}
	hash, err := s.GetPasswordHash(user)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		return &teleport.BadParameterError{Err: "passwords do not match"}
	}

	return nil
}

// verifyPassword makes sure password satisfies our requirements (relaxed),
// mostly to avoid putting garbage in
func verifyPassword(password []byte) error {
	if len(password) < defaults.MinPasswordLength {
		return teleport.BadParameter(
			"password",
			fmt.Sprintf(
				"password is too short, min length is %v", defaults.MinPasswordLength))
	}
	if len(password) > defaults.MaxPasswordLength {
		return teleport.BadParameter(
			"password",
			fmt.Sprintf(
				"password is too long, max length is %v", defaults.MaxPasswordLength))
	}
	return nil
}

// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
func (s *IdentityService) UpsertSignupToken(token string, tokenData SignupToken, ttl time.Duration) error {
	if ttl < time.Second || ttl > defaults.MaxSignupTokenTTL {
		ttl = defaults.MaxSignupTokenTTL
	}
	out, err := json.Marshal(tokenData)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal(userTokensPath, token, out, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil

}

// GetSignupToken returns signup token data
func (s *IdentityService) GetSignupToken(token string) (*SignupToken, error) {
	out, err := s.backend.GetVal(userTokensPath, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data *SignupToken
	err = json.Unmarshal(out, &data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// DeleteSignupToken deletes signup token from the storage
func (s *IdentityService) DeleteSignupToken(token string) error {
	err := s.backend.DeleteKey(userTokensPath, token)
	return trace.Wrap(err)
}

// UpsertOIDCConnector upserts OIDC Connector
func (s *IdentityService) UpsertOIDCConnector(connector OIDCConnector, ttl time.Duration) error {
	if err := connector.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal(connectorsPath, connector.ID, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteOIDCConnector deletes OIDC Connector
func (s *IdentityService) DeleteOIDCConnector(connectorID string) error {
	err := s.backend.DeleteKey(connectorsPath, connectorID)
	return trace.Wrap(err)
}

// GetOIDCConnector returns OIDC connector data, , withSecrets adds or removes client secret from return results
func (s *IdentityService) GetOIDCConnector(id string, withSecrets bool) (*OIDCConnector, error) {
	out, err := s.backend.GetVal(connectorsPath, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data *OIDCConnector
	err = json.Unmarshal(out, &data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		data.ClientSecret = ""
	}
	return data, nil
}

// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (s *IdentityService) GetOIDCConnectors(withSecrets bool) ([]OIDCConnector, error) {
	connectorIDs, err := s.backend.GetKeys(connectorsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]OIDCConnector, 0, len(connectorIDs))
	for _, id := range connectorIDs {
		connector, err := s.GetOIDCConnector(id, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors = append(connectors, *connector)
	}
	return connectors, nil
}

// CreateOIDCAuthRequest creates new auth request
func (s *IdentityService) CreateOIDCAuthRequest(req OIDCAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.CreateVal(authRequestsPath, req.StateToken, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCAuthRequest returns OIDC auth request if found
func (s *IdentityService) GetOIDCAuthRequest(stateToken string) (*OIDCAuthRequest, error) {
	data, err := s.backend.GetVal(authRequestsPath, stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req *OIDCAuthRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
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
	Token           string   `json:"token"`
	User            User     `json:"user"`
	Hotp            []byte   `json:"hotp"`
	HotpFirstValues []string `json:"hotp_first_values"`
	HotpQR          []byte   `json:"hotp_qr"`
}

// OIDCConnector specifies configuration fo Open ID Connect compatible external
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
}

// Check returns nil if all parameters are great, err otherwise
func (o *OIDCConnector) Check() error {
	if o.ID == "" {
		return trace.Wrap(teleport.BadParameter("ID", "missing connector id"))
	}
	if _, err := url.Parse(o.IssuerURL); err != nil {
		return trace.Wrap(teleport.BadParameter("IssuerURL", fmt.Sprintf("bad url: '%v'", o.IssuerURL)))
	}
	if _, err := url.Parse(o.RedirectURL); err != nil {
		return trace.Wrap(teleport.BadParameter("RedirectURL", fmt.Sprintf("bad url: '%v'", o.RedirectURL)))
	}
	if o.ClientID == "" {
		return trace.Wrap(teleport.BadParameter("ClientID", "missing client id"))
	}
	if o.ClientSecret == "" {
		return trace.Wrap(teleport.BadParameter("ClientID", "missing client secret"))
	}
	return nil
}

var (
	userTokensPath   = []string{"addusertokens"}
	connectorsPath   = []string{"web", "connectors", "oidc", "connectors"}
	authRequestsPath = []string{"web", "connectors", "oidc", "requests"}
)

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
		return trace.Wrap(teleport.BadParameter("ConnectorID", "missing value"))
	}
	if i.Email == "" {
		return trace.Wrap(teleport.BadParameter("Email", "missing email"))
	}
	return nil
}

// OIDCAuthRequest is a request to authenticate with OIDC
// provider, the state about request is managed by auth server
type OIDCAuthRequest struct {
	// ConnectorID is ID of OIDC connector this request uses
	ConnectorID string `json:"connector_id"`

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
		return trace.Wrap(teleport.BadParameter("ConnectorID", "missing value"))
	}
	if i.StateToken == "" {
		return trace.Wrap(teleport.BadParameter("StateToken", "missing value"))
	}
	if len(i.PublicKey) != 0 {
		_, _, _, _, err := ssh.ParseAuthorizedKey(i.PublicKey)
		if err != nil {
			return trace.Wrap(teleport.BadParameter("PublicKey", fmt.Sprintf("bad key: %v", err)))
		}
		if (i.CertTTL > defaults.MaxCertDuration) || (i.CertTTL < defaults.MinCertDuration) {
			return trace.Wrap(teleport.BadParameter("CertTTL", "wrong certificate TTL"))
		}
	}

	return nil
}
