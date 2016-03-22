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
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gokyle/hotp"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
)

// User is an optional user entry in the database
type User struct {
	// Name is a user name
	Name string `json:"name"`

	// AllowedLogins represents a list of OS users this teleport
	// user is allowed to login as
	AllowedLogins []string `json:"allowed_logins"`
}

// AuthorizedKey is a public key that is authorized to access SSH
// servers
type AuthorizedKey struct {
	// ID is a unique key id
	ID string `json:"id"`
	// Value is a value of the public key
	Value []byte `json:"value"`
}

// WebService is responsible for managing web users and currently
// user accounts as well
type WebService struct {
	backend     backend.Backend
	SignupMutex *sync.Mutex
}

// NewWebService returns new instance of WebService
func NewWebService(backend backend.Backend) *WebService {
	return &WebService{
		backend:     backend,
		SignupMutex: &sync.Mutex{},
	}
}

// GetUsers returns a list of users registered with the local auth server
func (s *WebService) GetUsers() ([]User, error) {
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
func (s *WebService) UpsertUser(user User) error {
	if !cstrings.IsValidUnixUser(user.Name) {
		return trace.Wrap(
			teleport.BadParameter("user.Name", fmt.Sprintf("'%v is not a valid unix username'", user.Name)))
	}
	data, err := json.Marshal(user.AllowedLogins)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, l := range user.AllowedLogins {
		if !cstrings.IsValidUnixUser(l) {
			return trace.Wrap(
				teleport.BadParameter("login", fmt.Sprintf("'%v is not a valid unix username'", l)))
		}
	}
	err = s.backend.UpsertVal([]string{"web", "users", user.Name}, "logins", []byte(data), backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUser returns a user by name
func (s *WebService) GetUser(user string) (*User, error) {
	u := User{Name: user}
	data, err := s.backend.GetVal([]string{"web", "users", user}, "logins")
	if err != nil {
		if teleport.IsNotFound(err) {
			return &u, nil
		}
		return nil, trace.Wrap(err)
	}
	if err := json.Unmarshal(data, &u.AllowedLogins); err != nil {
		return nil, trace.Wrap(err)
	}
	return &u, nil
}

// DeleteUser deletes a user with all the keys from the backend
func (s *WebService) DeleteUser(user string) error {
	err := s.backend.DeleteBucket([]string{"web", "users"}, user)
	if err != nil {
		if teleport.IsNotFound(err) {
			return trace.Wrap(teleport.NotFound(fmt.Sprintf("user '%v' is not found", user)))
		}
	}
	return trace.Wrap(err)
}

// UpsertPasswordHash upserts user password hash
func (s *WebService) UpsertPasswordHash(user string, hash []byte) error {
	err := s.backend.UpsertVal([]string{"web", "users", user}, "pwd", hash, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPasswordHash returns the password hash for a given user
func (s *WebService) GetPasswordHash(user string) ([]byte, error) {
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
func (s *WebService) UpsertHOTP(user string, otp *hotp.HOTP) error {
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
func (s *WebService) GetHOTP(user string) (*hotp.HOTP, error) {
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
func (s *WebService) UpsertWebSession(user, sid string, session WebSession, ttl time.Duration) error {
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
func (s *WebService) GetWebSession(user, sid string) (*WebSession, error) {
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
func (s *WebService) GetWebSessionsKeys(user string) ([]AuthorizedKey, error) {
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
func (s *WebService) DeleteWebSession(user, sid string) error {
	err := s.backend.DeleteKey(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	return err
}

// UpsertPassword upserts new password and HOTP token
func (s *WebService) UpsertPassword(user string,
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
func (s *WebService) CheckPassword(user string, password []byte, hotpToken string) error {
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
func (s *WebService) CheckPasswordWOToken(user string, password []byte) error {
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
	User            string   `json:"user"`
	Hotp            []byte   `json:"hotp"`
	HotpFirstValues []string `json:"hotp_first_values"`
	HotpQR          []byte   `json:"hotp_qr"`
	AllowedLogins   []string `json:"allowed_logins"`
}

var (
	userTokensPath = []string{"addusertokens"}
)

// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
func (s *WebService) UpsertSignupToken(token string, tokenData SignupToken, ttl time.Duration) error {
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
func (s *WebService) GetSignupToken(token string) (*SignupToken, error) {
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
func (s *WebService) DeleteSignupToken(token string) error {
	err := s.backend.DeleteKey(userTokensPath, token)
	return trace.Wrap(err)
}
