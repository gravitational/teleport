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

package local

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
)

// IdentityService is responsible for managing web users and currently
// user accounts as well
type IdentityService struct {
	backend.Backend
}

// NewIdentityService returns a new instance of IdentityService object
func NewIdentityService(backend backend.Backend) *IdentityService {
	return &IdentityService{
		Backend: backend,
	}
}

// DeleteAllUsers deletes all users
func (s *IdentityService) DeleteAllUsers() error {
	return s.DeleteBucket([]string{"web"}, "users")
}

// GetUsers returns a list of users registered with the local auth server
func (s *IdentityService) GetUsers() ([]services.User, error) {
	keys, err := s.GetKeys([]string{"web", "users"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.User, 0, len(keys))
	for _, name := range keys {
		u, err := s.GetUser(name)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		out = append(out, u)
	}
	return out, nil
}

// CreateUser creates user if it does not exist
func (s *IdentityService) CreateUser(user services.User) error {
	if err := user.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetUserMarshaler().MarshalUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.CreateVal([]string{"web", "users", user.GetName()}, "params", []byte(data), backend.TTL(s.Clock(), user.Expiry()))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertUser updates parameters about user
func (s *IdentityService) UpsertUser(user services.User) error {
	if err := user.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetUserMarshaler().MarshalUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), user.Expiry())
	err = s.UpsertVal([]string{"web", "users", user.GetName()}, "params", []byte(data), ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUser returns a user by name
func (s *IdentityService) GetUser(user string) (services.User, error) {
	data, err := s.GetVal([]string{"web", "users", user}, "params")
	if err != nil {
		return nil, trace.NotFound("user %v is not found", user)
	}
	u, err := services.GetUserMarshaler().UnmarshalUser(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserByOIDCIdentity(id services.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetOIDCIdentities() {
			if uid.Equals(&id) {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with identity %v not found", &id)
}

// GetUserBySAMLCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserBySAMLIdentity(id services.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetSAMLIdentities() {
			if uid.Equals(&id) {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with identity %v not found", &id)
}

// GetUserByGithubIdentity returns the first found user with specified Github identity
func (s *IdentityService) GetUserByGithubIdentity(id services.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetGithubIdentities() {
			if uid.Equals(&id) {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with identity %v not found", &id)
}

// DeleteUser deletes a user with all the keys from the backend
func (s *IdentityService) DeleteUser(user string) error {
	err := s.DeleteBucket([]string{"web", "users"}, user)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("user %q is not found", user)
		}
	}
	return trace.Wrap(err)
}

// UpsertPasswordHash upserts user password hash
func (s *IdentityService) UpsertPasswordHash(username string, hash []byte) error {
	userPrototype, err := services.NewUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	user, err := services.GetUserMarshaler().GenerateUser(userPrototype)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.CreateUser(user)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	err = s.UpsertVal([]string{"web", "users", username}, "pwd", hash, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPasswordHash returns the password hash for a given user
func (s *IdentityService) GetPasswordHash(user string) ([]byte, error) {
	hash, err := s.GetVal([]string{"web", "users", user}, "pwd")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user '%v' is not found", user)
		}
		return nil, trace.Wrap(err)
	}
	return hash, nil
}

// UpsertHOTP upserts HOTP state for user
// Deprecated: HOTP use is deprecated, use UpsertTOTP instead.
func (s *IdentityService) UpsertHOTP(user string, otp *hotp.HOTP) error {
	bytes, err := hotp.Marshal(otp)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal([]string{"web", "users", user}, "hotp", bytes, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetHOTP gets HOTP token state for a user
// Deprecated: HOTP use is deprecated, use GetTOTP instead.
func (s *IdentityService) GetHOTP(user string) (*hotp.HOTP, error) {
	bytes, err := s.GetVal([]string{"web", "users", user}, "hotp")
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user %q is not found", user)
		}
		return nil, trace.Wrap(err)
	}

	otp, err := hotp.Unmarshal(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return otp, nil
}

// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
func (s *IdentityService) UpsertTOTP(user string, secretKey string) error {
	err := s.UpsertVal([]string{"web", "users", user}, "totp", []byte(secretKey), 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens
func (s *IdentityService) GetTOTP(user string) (string, error) {
	bytes, err := s.GetVal([]string{"web", "users", user}, "totp")
	if err != nil {
		if trace.IsNotFound(err) {
			return "", trace.NotFound("user %q not found", user)
		}
		return "", trace.Wrap(err)
	}

	return string(bytes), nil
}

// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
// during the 30 second window it's valid.
func (s *IdentityService) UpsertUsedTOTPToken(user string, otpToken string) error {
	err := s.UpsertVal([]string{"web", "users", user}, "used_totp", []byte(otpToken), 30*time.Second)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUsedTOTPToken returns the last successfully used TOTP token. If no token is found zero is returned.
func (s *IdentityService) GetUsedTOTPToken(user string) (string, error) {
	bytes, err := s.GetVal([]string{"web", "users", user}, "used_totp")
	if err != nil {
		if trace.IsNotFound(err) {
			return "0", nil
		}
		return "", trace.Wrap(err)
	}

	return string(bytes), nil
}

// DeleteUsedTOTPToken removes the used token from the backend. This should only
// be used during tests.
func (s *IdentityService) DeleteUsedTOTPToken(user string) error {
	return s.DeleteKey([]string{"web", "users", user}, "used_totp")
}

// UpsertWebSession updates or inserts a web session for a user and session id
// the session will be created with bearer token expiry time TTL, because
// it is expected to be extended by the client before then
func (s *IdentityService) UpsertWebSession(user, sid string, session services.WebSession) error {
	session.SetUser(user)
	session.SetName(sid)
	bytes, err := services.GetWebSessionMarshaler().MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	ttl := backend.AnyTTL(clockwork.NewRealClock(), session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry())
	err = s.UpsertVal([]string{"web", "users", user, "sessions"},
		sid, bytes, ttl)
	if trace.IsNotFound(err) {
		return trace.NotFound("user '%v' is not found", user)
	}
	return trace.Wrap(err)
}

// AddUserLoginAttempt logs user login attempt
func (s *IdentityService) AddUserLoginAttempt(user string, attempt services.LoginAttempt, ttl time.Duration) error {
	if err := attempt.Check(); err != nil {
		return trace.Wrap(err)
	}
	bytes, err := json.Marshal(attempt)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.UpsertVal([]string{"web", "users", user, "attempts"},
		uuid.New(), bytes, ttl)
	if trace.IsNotFound(err) {
		return trace.NotFound("user '%v' is not found", user)
	}
	return trace.Wrap(err)
}

// GetUserLoginAttempts returns user login attempts
func (s *IdentityService) GetUserLoginAttempts(user string) ([]services.LoginAttempt, error) {
	keys, err := s.GetKeys([]string{"web", "users", user, "attempts"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.LoginAttempt, 0, len(keys))
	for _, id := range keys {
		data, err := s.GetVal([]string{"web", "users", user, "attempts"}, id)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		var a services.LoginAttempt
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, a)
	}
	sort.Sort(services.SortedLoginAttempts(out))
	return out, nil
}

// DeleteUserLoginAttempts removes all login attempts of a user. Should be
// called after successful login.
func (s *IdentityService) DeleteUserLoginAttempts(user string) error {
	err := s.DeleteBucket([]string{"web", "users", user}, "attempts")
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound(fmt.Sprintf("user '%v' is not found", user))
		}
	}
	return trace.Wrap(err)
}

// GetWebSession returns a web session state for a given user and session id
func (s *IdentityService) GetWebSession(user, sid string) (services.WebSession, error) {
	val, err := s.GetVal([]string{"web", "users", user, "sessions"}, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(val)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this is for backwards compatibility to ensure we
	// always have these values
	session.SetUser(user)
	session.SetName(sid)
	return session, nil
}

// DeleteWebSession deletes web session from the storage
func (s *IdentityService) DeleteWebSession(user, sid string) error {
	err := s.DeleteKey(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	return err
}

// UpsertPassword upserts new password hash into a backend.
func (s *IdentityService) UpsertPassword(user string, password []byte) error {
	err := services.VerifyPassword(password)
	if err != nil {
		return trace.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertPasswordHash(user, hash)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

var (
	userTokensPath         = []string{"addusertokens"}
	u2fRegChalPath         = []string{"adduseru2fchallenges"}
	oidcConnectorsPath     = []string{"web", "connectors", "oidc", "connectors"}
	oidcAuthRequestsPath   = []string{"web", "connectors", "oidc", "requests"}
	samlConnectorsPath     = []string{"web", "connectors", "saml", "connectors"}
	samlAuthRequestsPath   = []string{"web", "connectors", "saml", "requests"}
	githubConnectorsPath   = []string{"web", "connectors", "github", "connectors"}
	githubAuthRequestsPath = []string{"web", "connectors", "github", "requests"}
)

// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
func (s *IdentityService) UpsertSignupToken(token string, tokenData services.SignupToken, ttl time.Duration) error {
	if ttl < time.Second || ttl > defaults.MaxSignupTokenTTL {
		ttl = defaults.MaxSignupTokenTTL
	}
	tokenData.Expires = time.Now().UTC().Add(ttl)
	out, err := json.Marshal(tokenData)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal(userTokensPath, token, out, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil

}

// GetSignupToken returns signup token data
func (s *IdentityService) GetSignupToken(token string) (*services.SignupToken, error) {
	out, err := s.GetVal(userTokensPath, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data *services.SignupToken
	err = json.Unmarshal(out, &data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// GetSignupTokens returns all non-expired user tokens
func (s *IdentityService) GetSignupTokens() (tokens []services.SignupToken, err error) {
	keys, err := s.GetKeys(userTokensPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, key := range keys {
		token, err := s.GetSignupToken(key)
		if err != nil {
			log.Error(err)
		}
		tokens = append(tokens, *token)
	}
	return tokens, trace.Wrap(err)
}

// DeleteSignupToken deletes signup token from the storage
func (s *IdentityService) DeleteSignupToken(token string) error {
	err := s.DeleteKey(userTokensPath, token)
	return trace.Wrap(err)
}

func (s *IdentityService) UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error {
	data, err := json.Marshal(u2fChallenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.UpsertVal(u2fRegChalPath, token, data, defaults.U2FChallengeTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FRegisterChallenge(token string) (*u2f.Challenge, error) {
	data, err := s.GetVal(u2fRegChalPath, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u2fChal := u2f.Challenge{}
	err = json.Unmarshal(data, &u2fChal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2fChal, nil
}

// u2f.Registration cannot be json marshalled due to the pointer in the public key so we have this marshallable version
type MarshallableU2FRegistration struct {
	Raw              []byte `json:"raw"`
	KeyHandle        []byte `json:"keyhandle"`
	MarshalledPubKey []byte `json:"marshalled_pubkey"`

	// AttestationCert is not needed for authentication so we don't need to store it
}

func (s *IdentityService) UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error {
	marshalledPubkey, err := x509.MarshalPKIXPublicKey(&u2fReg.PubKey)
	if err != nil {
		return trace.Wrap(err)
	}

	marshallableReg := MarshallableU2FRegistration{
		Raw:              u2fReg.Raw,
		KeyHandle:        u2fReg.KeyHandle,
		MarshalledPubKey: marshalledPubkey,
	}

	data, err := json.Marshal(marshallableReg)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal([]string{"web", "users", user}, "u2fregistration", data, backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FRegistration(user string) (*u2f.Registration, error) {
	data, err := s.GetVal([]string{"web", "users", user}, "u2fregistration")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	marshallableReg := MarshallableU2FRegistration{}
	err = json.Unmarshal(data, &marshallableReg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubkeyInterface, err := x509.ParsePKIXPublicKey(marshallableReg.MarshalledPubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubkey, ok := pubkeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.Wrap(errors.New("failed to convert crypto.PublicKey back to ecdsa.PublicKey"))
	}

	return &u2f.Registration{
		Raw:             marshallableReg.Raw,
		KeyHandle:       marshallableReg.KeyHandle,
		PubKey:          *pubkey,
		AttestationCert: nil,
	}, nil
}

type U2FRegistrationCounter struct {
	Counter uint32 `json:"counter"`
}

func (s *IdentityService) UpsertU2FRegistrationCounter(user string, counter uint32) error {
	data, err := json.Marshal(U2FRegistrationCounter{
		Counter: counter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertVal([]string{"web", "users", user}, "u2fregistrationcounter", data, backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FRegistrationCounter(user string) (counter uint32, e error) {
	data, err := s.GetVal([]string{"web", "users", user}, "u2fregistrationcounter")
	if err != nil {
		return 0, trace.Wrap(err)
	}

	u2fRegCounter := U2FRegistrationCounter{}
	err = json.Unmarshal(data, &u2fRegCounter)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return u2fRegCounter.Counter, nil
}

func (s *IdentityService) UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error {
	data, err := json.Marshal(u2fChallenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.UpsertVal([]string{"web", "users", user}, "u2fsignchallenge", data, defaults.U2FChallengeTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FSignChallenge(user string) (*u2f.Challenge, error) {
	data, err := s.GetVal([]string{"web", "users", user}, "u2fsignchallenge")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u2fChal := u2f.Challenge{}
	err = json.Unmarshal(data, &u2fChal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2fChal, nil
}

// UpsertOIDCConnector upserts OIDC Connector
func (s *IdentityService) UpsertOIDCConnector(connector services.OIDCConnector) error {
	if err := connector.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetOIDCConnectorMarshaler().MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), connector.Expiry())
	err = s.UpsertVal(oidcConnectorsPath, connector.GetName(), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteOIDCConnector deletes OIDC Connector
func (s *IdentityService) DeleteOIDCConnector(connectorID string) error {
	err := s.DeleteKey(oidcConnectorsPath, connectorID)
	return trace.Wrap(err)
}

// GetOIDCConnector returns OIDC connector data, , withSecrets adds or removes client secret from return results
func (s *IdentityService) GetOIDCConnector(id string, withSecrets bool) (services.OIDCConnector, error) {
	data, err := s.GetVal(oidcConnectorsPath, id)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("OpenID connector '%v' is not configured", id)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		conn.SetClientSecret("")
	}
	return conn, nil
}

// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (s *IdentityService) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	connectorIDs, err := s.GetKeys(oidcConnectorsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.OIDCConnector, 0, len(connectorIDs))
	for _, id := range connectorIDs {
		connector, err := s.GetOIDCConnector(id, withSecrets)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			// the record has expired
			continue
		}
		connectors = append(connectors, connector)
	}
	return connectors, nil
}

// CreateOIDCAuthRequest creates new auth request
func (s *IdentityService) CreateOIDCAuthRequest(req services.OIDCAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.CreateVal(oidcAuthRequestsPath, req.StateToken, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCAuthRequest returns OIDC auth request if found
func (s *IdentityService) GetOIDCAuthRequest(stateToken string) (*services.OIDCAuthRequest, error) {
	data, err := s.GetVal(oidcAuthRequestsPath, stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req *services.OIDCAuthRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// CreateSAMLConnector creates SAML Connector
func (s *IdentityService) CreateSAMLConnector(connector services.SAMLConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), connector.Expiry())
	err = s.CreateVal(samlConnectorsPath, connector.GetName(), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertSAMLConnector upserts SAML Connector
func (s *IdentityService) UpsertSAMLConnector(connector services.SAMLConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), connector.Expiry())
	err = s.UpsertVal(samlConnectorsPath, connector.GetName(), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSAMLConnector deletes OIDC Connector
func (s *IdentityService) DeleteSAMLConnector(connectorID string) error {
	err := s.DeleteKey(samlConnectorsPath, connectorID)
	return trace.Wrap(err)
}

// GetSAMLConnector returns OIDC connector data, withSecrets adds or removes secrets from return results
func (s *IdentityService) GetSAMLConnector(id string, withSecrets bool) (services.SAMLConnector, error) {
	data, err := s.GetVal(samlConnectorsPath, id)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("SAML connector '%v' is not configured", id)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		keyPair := conn.GetSigningKeyPair()
		if keyPair != nil {
			keyPair.PrivateKey = ""
			conn.SetSigningKeyPair(keyPair)
		}
	}
	return conn, nil
}

// GetSAMLConnectors returns registered connectors, withSecrets adds or removes secret from return results
func (s *IdentityService) GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error) {
	connectorIDs, err := s.GetKeys(samlConnectorsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.SAMLConnector, 0, len(connectorIDs))
	for _, id := range connectorIDs {
		connector, err := s.GetSAMLConnector(id, withSecrets)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			// the record has expired
			continue
		}
		connectors = append(connectors, connector)
	}
	return connectors, nil
}

// CreateSAMLAuthRequest creates new auth request
func (s *IdentityService) CreateSAMLAuthRequest(req services.SAMLAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.CreateVal(samlAuthRequestsPath, req.ID, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSAMLAuthRequest returns OSAML auth request if found
func (s *IdentityService) GetSAMLAuthRequest(id string) (*services.SAMLAuthRequest, error) {
	data, err := s.GetVal(samlAuthRequestsPath, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req *services.SAMLAuthRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// CreateGithubConnector creates a new Github connector
func (s *IdentityService) CreateGithubConnector(connector services.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	bytes, err := services.GetGithubConnectorMarshaler().Marshal(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), connector.Expiry())
	if err := s.CreateVal(githubConnectorsPath, connector.GetName(), bytes, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertGithubConnector creates or updates a Github connector
func (s *IdentityService) UpsertGithubConnector(connector services.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	bytes, err := services.GetGithubConnectorMarshaler().Marshal(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), connector.Expiry())
	if err := s.UpsertVal(githubConnectorsPath, connector.GetName(), bytes, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnectors returns all configured Github connectors
func (s *IdentityService) GetGithubConnectors(withSecrets bool) ([]services.GithubConnector, error) {
	ids, err := s.GetKeys(githubConnectorsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.GithubConnector, 0, len(ids))
	for _, id := range ids {
		connector, err := s.GetGithubConnector(id, withSecrets)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		connectors = append(connectors, connector)
	}
	return connectors, nil
}

// GetGithubConnectot returns a particular Github connector
func (s *IdentityService) GetGithubConnector(id string, withSecrets bool) (services.GithubConnector, error) {
	bytes, err := s.GetVal(githubConnectorsPath, id)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("Github connector %q is not configured", id)
		}
		return nil, trace.Wrap(err)
	}
	connector, err := services.GetGithubConnectorMarshaler().Unmarshal(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		connector.SetClientSecret("")
	}
	return connector, nil
}

// DeleteGithubConnector deletes the specified connector
func (s *IdentityService) DeleteGithubConnector(id string) error {
	return trace.Wrap(s.DeleteKey(githubConnectorsPath, id))
}

// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
func (s *IdentityService) CreateGithubAuthRequest(req services.GithubAuthRequest, ttl time.Duration) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.CreateVal(githubAuthRequestsPath, req.StateToken, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubAuthRequest retrieves Github auth request by the token
func (s *IdentityService) GetGithubAuthRequest(stateToken string) (*services.GithubAuthRequest, error) {
	data, err := s.GetVal(githubAuthRequestsPath, stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req services.GithubAuthRequest
	err = json.Unmarshal(data, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}
