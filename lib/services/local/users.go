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
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gokyle/hotp"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
)

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
func (s *IdentityService) GetUsers() ([]services.User, error) {
	keys, err := s.backend.GetKeys([]string{"web", "users"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.User, len(keys))
	for i, name := range keys {
		u, err := s.GetUser(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = u
	}
	return out, nil
}

// UpsertUser updates parameters about user
func (s *IdentityService) UpsertUser(user services.User) error {
	if !cstrings.IsValidUnixUser(user.GetName()) {
		return trace.Wrap(
			teleport.BadParameter("user.Name", fmt.Sprintf("'%v is not a valid unix username'", user.GetName())))
	}

	for _, l := range user.GetAllowedLogins() {
		if !cstrings.IsValidUnixUser(l) {
			return trace.Wrap(
				teleport.BadParameter("login", fmt.Sprintf("'%v is not a valid unix username'", l)))
		}
	}
	for _, i := range user.GetIdentities() {
		if err := i.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	data, err := json.Marshal(user)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal([]string{"web", "users", user.GetName()}, "params", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUser returns a user by name
func (s *IdentityService) GetUser(user string) (services.User, error) {
	u := services.TeleportUser{Name: user}
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
func (s *IdentityService) GetUserByOIDCIdentity(id services.OIDCIdentity) (services.User, error) {
	users, err := s.GetUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetIdentities() {
			if uid.Equals(&id) {
				return u, nil
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
func (s *IdentityService) UpsertWebSession(user, sid string, session services.WebSession, ttl time.Duration) error {
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
func (s *IdentityService) GetWebSession(user, sid string) (*services.WebSession, error) {
	val, err := s.backend.GetVal(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var session services.WebSession
	err = json.Unmarshal(val, &session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session, nil
}

// GetWebSessionsKeys returns public keys associated with the session
func (s *IdentityService) GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error) {
	keys, err := s.backend.GetKeys([]string{"web", "users", user, "sessions"})
	if err != nil {
		return nil, err
	}

	values := make([]services.AuthorizedKey, len(keys))
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

	if err := services.VerifyPassword(password); err != nil {
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
	if err := services.VerifyPassword(password); err != nil {
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
	if err := services.VerifyPassword(password); err != nil {
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

var (
	userTokensPath   = []string{"addusertokens"}
	connectorsPath   = []string{"web", "connectors", "oidc", "connectors"}
	authRequestsPath = []string{"web", "connectors", "oidc", "requests"}
)

// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
func (s *IdentityService) UpsertSignupToken(token string, tokenData services.SignupToken, ttl time.Duration) error {
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
func (s *IdentityService) GetSignupToken(token string) (*services.SignupToken, error) {
	out, err := s.backend.GetVal(userTokensPath, token)
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

// DeleteSignupToken deletes signup token from the storage
func (s *IdentityService) DeleteSignupToken(token string) error {
	err := s.backend.DeleteKey(userTokensPath, token)
	return trace.Wrap(err)
}

// UpsertOIDCConnector upserts OIDC Connector
func (s *IdentityService) UpsertOIDCConnector(connector services.OIDCConnector, ttl time.Duration) error {
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
func (s *IdentityService) GetOIDCConnector(id string, withSecrets bool) (*services.OIDCConnector, error) {
	out, err := s.backend.GetVal(connectorsPath, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data *services.OIDCConnector
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
func (s *IdentityService) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	connectorIDs, err := s.backend.GetKeys(connectorsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.OIDCConnector, 0, len(connectorIDs))
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
func (s *IdentityService) CreateOIDCAuthRequest(req services.OIDCAuthRequest, ttl time.Duration) error {
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
func (s *IdentityService) GetOIDCAuthRequest(stateToken string) (*services.OIDCAuthRequest, error) {
	data, err := s.backend.GetVal(authRequestsPath, stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req *services.OIDCAuthRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}
