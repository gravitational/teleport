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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
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
	startKey := backend.Key(webPrefix, usersPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// GetUsers returns a list of users registered with the local auth server
func (s *IdentityService) GetUsers(withSecrets bool) ([]services.User, error) {
	if withSecrets {
		return s.getUsersWithSecrets()
	}
	startKey := backend.Key(webPrefix, usersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []services.User
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			continue
		}
		u, err := services.UnmarshalUser(
			item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !withSecrets {
			u.SetLocalAuth(nil)
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *IdentityService) getUsersWithSecrets() ([]services.User, error) {
	startKey := backend.Key(webPrefix, usersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	collected, _, err := collectUserItems(result.Items)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	users := make([]services.User, 0, len(collected))
	for uname, uitems := range collected {
		user, err := userFromUserItems(uname, uitems)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		users = append(users, user)
	}
	return users, nil
}

// CreateUser creates user if it does not exist.
func (s *IdentityService) CreateUser(user services.User) error {
	if err := user.Check(); err != nil {
		return trace.Wrap(err)
	}

	// Confirm user doesn't exist before creating.
	_, err := s.GetUser(user.GetName(), false)
	if !trace.IsNotFound(err) {
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.AlreadyExists("user %q already registered", user.GetName())
	}

	value, err := services.MarshalUser(user.WithoutSecrets().(services.User))
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:   value,
		Expires: user.Expiry(),
	}

	if _, err = s.Create(context.TODO(), item); err != nil {
		return trace.Wrap(err)
	}

	if auth := user.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(user.GetName(), *auth); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UpdateUser updates an existing user.
func (s *IdentityService) UpdateUser(ctx context.Context, user services.User) error {
	if err := user.Check(); err != nil {
		return trace.Wrap(err)
	}

	// Confirm user exists before updating.
	if _, err := s.GetUser(user.GetName(), false); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalUser(user.WithoutSecrets().(services.User))
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:   value,
		Expires: user.Expiry(),
		ID:      user.GetResourceID(),
	}
	_, err = s.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	if auth := user.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(user.GetName(), *auth); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UpsertUser updates parameters about user, or creates an entry if not exist.
func (s *IdentityService) UpsertUser(user services.User) error {
	if err := user.Check(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalUser(user.WithoutSecrets().(services.User))
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:   value,
		Expires: user.Expiry(),
		ID:      user.GetResourceID(),
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	if auth := user.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(user.GetName(), *auth); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetUser returns a user by name
func (s *IdentityService) GetUser(user string, withSecrets bool) (services.User, error) {
	if withSecrets {
		return s.getUserWithSecrets(user)
	}
	if user == "" {
		return nil, trace.BadParameter("missing user name")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, paramsPrefix))
	if err != nil {
		return nil, trace.NotFound("user %q is not found", user)
	}
	u, err := services.UnmarshalUser(
		item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		u.SetLocalAuth(nil)
	}
	return u, nil
}

func (s *IdentityService) getUserWithSecrets(user string) (services.User, error) {
	if user == "" {
		return nil, trace.BadParameter("missing user name")
	}
	startKey := backend.Key(webPrefix, usersPrefix, user)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var uitems userItems
	for _, item := range result.Items {
		suffix := trimToSuffix(string(item.Key))
		uitems.Set(suffix, item) // Result of Set i
	}
	u, err := userFromUserItems(user, uitems)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

func (s *IdentityService) upsertLocalAuthSecrets(user string, auth services.LocalAuthSecrets) error {
	if len(auth.PasswordHash) > 0 {
		err := s.UpsertPasswordHash(user, auth.PasswordHash)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if len(auth.TOTPKey) > 0 {
		err := s.UpsertTOTP(user, auth.TOTPKey)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if auth.U2FRegistration != nil {
		reg, err := services.GetLocalAuthSecretsU2FRegistration(&auth)
		if err != nil {
			return trace.Wrap(err)
		}
		err = s.UpsertU2FRegistration(user, reg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if auth.U2FCounter > 0 || auth.U2FRegistration != nil {
		err := s.UpsertU2FRegistrationCounter(user, auth.U2FCounter)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserByOIDCIdentity(id services.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers(false)
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
	return nil, trace.NotFound("user with identity %q not found", &id)
}

// GetUserBySAMLCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserBySAMLIdentity(id services.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers(false)
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
	return nil, trace.NotFound("user with identity %q not found", &id)
}

// GetUserByGithubIdentity returns the first found user with specified Github identity
func (s *IdentityService) GetUserByGithubIdentity(id services.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers(false)
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
func (s *IdentityService) DeleteUser(ctx context.Context, user string) error {
	_, err := s.GetUser(user, false)
	if err != nil {
		return trace.Wrap(err)
	}
	startKey := backend.Key(webPrefix, usersPrefix, user)
	err = s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// UpsertPasswordHash upserts user password hash
func (s *IdentityService) UpsertPasswordHash(username string, hash []byte) error {
	userPrototype, err := services.NewUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.CreateUser(userPrototype)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	item := backend.Item{
		Key:   backend.Key(webPrefix, usersPrefix, username, pwdPrefix),
		Value: hash,
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPasswordHash returns the password hash for a given user
func (s *IdentityService) GetPasswordHash(user string) ([]byte, error) {
	if user == "" {
		return nil, trace.BadParameter("missing user name")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, pwdPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user %q is not found", user)
		}
		return nil, trace.Wrap(err)
	}
	return item.Value, nil
}

// UpsertHOTP upserts HOTP state for user
// Deprecated: HOTP use is deprecated, use UpsertTOTP instead.
func (s *IdentityService) UpsertHOTP(user string, otp *hotp.HOTP) error {
	if user == "" {
		return trace.BadParameter("missing user name")
	}
	bytes, err := hotp.Marshal(otp)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(webPrefix, usersPrefix, user, hotpPrefix),
		Value: bytes,
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetHOTP gets HOTP token state for a user
// Deprecated: HOTP use is deprecated, use GetTOTP instead.
func (s *IdentityService) GetHOTP(user string) (*hotp.HOTP, error) {
	if user == "" {
		return nil, trace.BadParameter("missing user name")
	}

	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, hotpPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user %q is not found", user)
		}
		return nil, trace.Wrap(err)
	}

	otp, err := hotp.Unmarshal(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return otp, nil
}

// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
func (s *IdentityService) UpsertTOTP(user string, secretKey string) error {
	if user == "" {
		return trace.BadParameter("missing user name")
	}

	item := backend.Item{
		Key:   backend.Key(webPrefix, usersPrefix, user, totpPrefix),
		Value: []byte(secretKey),
	}

	_, err := s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens
func (s *IdentityService) GetTOTP(user string) (string, error) {
	if user == "" {
		return "", trace.BadParameter("missing user name")
	}

	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, totpPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return "", trace.NotFound("OTP key for user(%q) is not found", user)
		}
		return "", trace.Wrap(err)
	}

	return string(item.Value), nil
}

// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
// during the 30 second window it's valid.
func (s *IdentityService) UpsertUsedTOTPToken(user string, otpToken string) error {
	if user == "" {
		return trace.BadParameter("missing user name")
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user, usedTOTPPrefix),
		Value:   []byte(otpToken),
		Expires: s.Clock().Now().UTC().Add(usedTOTPTTL),
	}
	_, err := s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUsedTOTPToken returns the last successfully used TOTP token. If no token is found zero is returned.
func (s *IdentityService) GetUsedTOTPToken(user string) (string, error) {
	if user == "" {
		return "", trace.BadParameter("missing user name")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, usedTOTPPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return "0", nil
		}
		return "", trace.Wrap(err)
	}

	return string(item.Value), nil
}

// DeleteUsedTOTPToken removes the used token from the backend. This should only
// be used during tests.
func (s *IdentityService) DeleteUsedTOTPToken(user string) error {
	if user == "" {
		return trace.BadParameter("missing user name")
	}
	return s.Delete(context.TODO(), backend.Key(webPrefix, usersPrefix, user, usedTOTPPrefix))
}

// UpsertWebSession updates or inserts a web session for a user and session id
// the session will be created with bearer token expiry time TTL, because
// it is expected to be extended by the client before then
func (s *IdentityService) UpsertWebSession(user, sid string, session services.WebSession) error {
	session.SetUser(user)
	session.SetName(sid)
	value, err := services.MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user, sessionsPrefix, sid),
		Value:   value,
		Expires: backend.EarliestExpiry(session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry()),
	}
	_, err = s.Put(context.TODO(), item)
	return trace.Wrap(err)
}

// AddUserLoginAttempt logs user login attempt
func (s *IdentityService) AddUserLoginAttempt(user string, attempt services.LoginAttempt, ttl time.Duration) error {
	if err := attempt.Check(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(attempt)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user, attemptsPrefix, uuid.New()),
		Value:   value,
		Expires: backend.Expiry(s.Clock(), ttl),
	}
	_, err = s.Put(context.TODO(), item)
	return trace.Wrap(err)
}

// GetUserLoginAttempts returns user login attempts
func (s *IdentityService) GetUserLoginAttempts(user string) ([]services.LoginAttempt, error) {
	startKey := backend.Key(webPrefix, usersPrefix, user, attemptsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.LoginAttempt, len(result.Items))
	for i, item := range result.Items {
		var a services.LoginAttempt
		if err := json.Unmarshal(item.Value, &a); err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = a
	}
	sort.Sort(services.SortedLoginAttempts(out))
	return out, nil
}

// DeleteUserLoginAttempts removes all login attempts of a user. Should be
// called after successful login.
func (s *IdentityService) DeleteUserLoginAttempts(user string) error {
	if user == "" {
		return trace.BadParameter("missing username")
	}
	startKey := backend.Key(webPrefix, usersPrefix, user, attemptsPrefix)
	err := s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetWebSession returns a web session state for a given user and session id
func (s *IdentityService) GetWebSession(user, sid string) (services.WebSession, error) {
	if user == "" {
		return nil, trace.BadParameter("missing username")
	}
	if sid == "" {
		return nil, trace.BadParameter("missing session id")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, sessionsPrefix, sid))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := services.UnmarshalWebSession(item.Value)
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
	if user == "" {
		return trace.BadParameter("missing username")
	}
	if sid == "" {
		return trace.BadParameter("missing session id")
	}
	err := s.Delete(context.TODO(), backend.Key(webPrefix, usersPrefix, user, sessionsPrefix, sid))
	return trace.Wrap(err)
}

// UpsertPassword upserts new password hash into a backend.
func (s *IdentityService) UpsertPassword(user string, password []byte) error {
	if user == "" {
		return trace.BadParameter("missing username")
	}
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

func (s *IdentityService) UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error {
	if token == "" {
		return trace.BadParameter("missing parmeter token")
	}
	value, err := json.Marshal(u2fChallenge)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(u2fRegChalPrefix, token),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(defaults.U2FChallengeTimeout),
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FRegisterChallenge(token string) (*u2f.Challenge, error) {
	if token == "" {
		return nil, trace.BadParameter("missing parameter token")
	}
	item, err := s.Get(context.TODO(), backend.Key(u2fRegChalPrefix, token))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var u2fChal u2f.Challenge
	err = json.Unmarshal(item.Value, &u2fChal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2fChal, nil
}

// u2fRegistration is a marshallable version of u2f.Registration that cannot be
// json marshalled due to the pointer in the public key
type u2fRegistration struct {
	Raw              []byte `json:"raw"`
	KeyHandle        []byte `json:"keyhandle"`
	MarshalledPubKey []byte `json:"marshalled_pubkey"`
	// AttestationCert is not needed for authentication so we don't need to store it
}

func (s *IdentityService) UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}

	pubKeyValue, err := x509.MarshalPKIXPublicKey(&u2fReg.PubKey)
	if err != nil {
		return trace.Wrap(err)
	}

	value, err := json.Marshal(u2fRegistration{
		Raw:              u2fReg.Raw,
		KeyHandle:        u2fReg.KeyHandle,
		MarshalledPubKey: pubKeyValue,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.Key(webPrefix, usersPrefix, user, u2fRegistrationPrefix),
		Value: value,
	}

	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FRegistration(user string) (*u2f.Registration, error) {
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, u2fRegistrationPrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var reg u2fRegistration
	err = json.Unmarshal(item.Value, &reg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubKeyI, err := x509.ParsePKIXPublicKey(reg.MarshalledPubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubKey, ok := pubKeyI.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("failed to convert crypto.PublicKey back to ecdsa.PublicKey")
	}

	return &u2f.Registration{
		Raw:       reg.Raw,
		KeyHandle: reg.KeyHandle,
		PubKey:    *pubKey,
	}, nil
}

type u2fRegistrationCounter struct {
	Counter uint32 `json:"counter"`
}

func (s *IdentityService) UpsertU2FRegistrationCounter(user string, counter uint32) error {
	if user == "" {
		return trace.BadParameter("missing parameter")
	}
	value, err := json.Marshal(u2fRegistrationCounter{
		Counter: counter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.Key(webPrefix, usersPrefix, user, u2fRegistrationCounterPrefix),
		Value: value,
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FRegistrationCounter(user string) (uint32, error) {
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, u2fRegistrationCounterPrefix))
	if err != nil {
		return 0, trace.Wrap(err)
	}
	var counter u2fRegistrationCounter
	err = json.Unmarshal(item.Value, &counter)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return counter.Counter, nil
}

func (s *IdentityService) UpsertU2FSignChallenge(user string, challenge *u2f.Challenge) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}
	value, err := json.Marshal(challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user, u2fSignChallengePrefix),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(defaults.U2FChallengeTimeout),
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetU2FSignChallenge(user string) (*u2f.Challenge, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, usersPrefix, user, u2fSignChallengePrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var signChallenge u2f.Challenge
	err = json.Unmarshal(item.Value, &signChallenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &signChallenge, nil
}

// UpsertOIDCConnector upserts OIDC Connector
func (s *IdentityService) UpsertOIDCConnector(connector services.OIDCConnector) error {
	if err := connector.Check(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
		ID:      connector.GetResourceID(),
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteOIDCConnector deletes OIDC Connector by name
func (s *IdentityService) DeleteOIDCConnector(name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	err := s.Delete(context.TODO(), backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, name))
	return trace.Wrap(err)
}

// GetOIDCConnector returns OIDC connector data, parameter 'withSecrets'
// includes or excludes client secret from return results
func (s *IdentityService) GetOIDCConnector(name string, withSecrets bool) (services.OIDCConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("OpenID connector '%v' is not configured", name)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := services.UnmarshalOIDCConnector(item.Value,
		services.WithExpires(item.Expires))
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
	startKey := backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.OIDCConnector, len(result.Items))
	for i, item := range result.Items {
		conn, err := services.UnmarshalOIDCConnector(
			item.Value, services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !withSecrets {
			conn.SetClientSecret("")
		}
		connectors[i] = conn
	}
	return connectors, nil
}

// CreateOIDCAuthRequest creates new auth request
func (s *IdentityService) CreateOIDCAuthRequest(req services.OIDCAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, oidcPrefix, requestsPrefix, req.StateToken),
		Value:   value,
		Expires: backend.Expiry(s.Clock(), ttl),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCAuthRequest returns OIDC auth request
func (s *IdentityService) GetOIDCAuthRequest(stateToken string) (*services.OIDCAuthRequest, error) {
	if stateToken == "" {
		return nil, trace.BadParameter("missing parameter stateToken")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, connectorsPrefix, oidcPrefix, requestsPrefix, stateToken))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req services.OIDCAuthRequest
	if err := json.Unmarshal(item.Value, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// CreateSAMLConnector creates SAML Connector
func (s *IdentityService) CreateSAMLConnector(connector services.SAMLConnector) error {
	if err := services.ValidateSAMLConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertSAMLConnector upserts SAML Connector
func (s *IdentityService) UpsertSAMLConnector(connector services.SAMLConnector) error {
	if err := services.ValidateSAMLConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSAMLConnector deletes SAML Connector by name
func (s *IdentityService) DeleteSAMLConnector(name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	err := s.Delete(context.TODO(), backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, name))
	return trace.Wrap(err)
}

// GetSAMLConnector returns SAML connector data,
// withSecrets includes or excludes secrets from return results
func (s *IdentityService) GetSAMLConnector(name string, withSecrets bool) (services.SAMLConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("SAML connector %q is not configured", name)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := services.UnmarshalSAMLConnector(
		item.Value, services.WithExpires(item.Expires))
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

// GetSAMLConnectors returns registered connectors
// withSecrets includes or excludes private key values from return results
func (s *IdentityService) GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error) {
	startKey := backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.SAMLConnector, len(result.Items))
	for i, item := range result.Items {
		conn, err := services.UnmarshalSAMLConnector(
			item.Value, services.WithExpires(item.Expires))
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
		connectors[i] = conn
	}
	return connectors, nil
}

// CreateSAMLAuthRequest creates new auth request
func (s *IdentityService) CreateSAMLAuthRequest(req services.SAMLAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, samlPrefix, requestsPrefix, req.ID),
		Value:   value,
		Expires: backend.Expiry(s.Clock(), ttl),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSAMLAuthRequest returns SAML auth request if found
func (s *IdentityService) GetSAMLAuthRequest(id string) (*services.SAMLAuthRequest, error) {
	if id == "" {
		return nil, trace.BadParameter("missing parameter id")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, connectorsPrefix, samlPrefix, requestsPrefix, id))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req services.SAMLAuthRequest
	if err := json.Unmarshal(item.Value, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// CreateGithubConnector creates a new Github connector
func (s *IdentityService) CreateGithubConnector(connector services.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertGithubConnector creates or updates a Github connector
func (s *IdentityService) UpsertGithubConnector(connector services.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
		ID:      connector.GetResourceID(),
	}
	_, err = s.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnectors returns all configured Github connectors
func (s *IdentityService) GetGithubConnectors(withSecrets bool) ([]services.GithubConnector, error) {
	startKey := backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.GithubConnector, len(result.Items))
	for i, item := range result.Items {
		connector, err := services.UnmarshalGithubConnector(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !withSecrets {
			connector.SetClientSecret("")
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// GetGithubConnectot returns a particular Github connector
func (s *IdentityService) GetGithubConnector(name string, withSecrets bool) (services.GithubConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("github connector %q is not configured", name)
		}
		return nil, trace.Wrap(err)
	}
	connector, err := services.UnmarshalGithubConnector(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		connector.SetClientSecret("")
	}
	return connector, nil
}

// DeleteGithubConnector deletes the specified connector
func (s *IdentityService) DeleteGithubConnector(name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	return trace.Wrap(s.Delete(context.TODO(), backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, name)))
}

// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
func (s *IdentityService) CreateGithubAuthRequest(req services.GithubAuthRequest) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(req)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, githubPrefix, requestsPrefix, req.StateToken),
		Value:   value,
		Expires: req.Expiry(),
	}
	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubAuthRequest retrieves Github auth request by the token
func (s *IdentityService) GetGithubAuthRequest(stateToken string) (*services.GithubAuthRequest, error) {
	if stateToken == "" {
		return nil, trace.BadParameter("missing parameter stateToken")
	}
	item, err := s.Get(context.TODO(), backend.Key(webPrefix, connectorsPrefix, githubPrefix, requestsPrefix, stateToken))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req services.GithubAuthRequest
	err = json.Unmarshal(item.Value, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

const (
	webPrefix                    = "web"
	usersPrefix                  = "users"
	sessionsPrefix               = "sessions"
	attemptsPrefix               = "attempts"
	pwdPrefix                    = "pwd"
	hotpPrefix                   = "hotp"
	totpPrefix                   = "totp"
	connectorsPrefix             = "connectors"
	oidcPrefix                   = "oidc"
	samlPrefix                   = "saml"
	githubPrefix                 = "github"
	requestsPrefix               = "requests"
	u2fRegChalPrefix             = "adduseru2fchallenges"
	usedTOTPPrefix               = "used_totp"
	usedTOTPTTL                  = 30 * time.Second
	u2fRegistrationPrefix        = "u2fregistration"
	u2fRegistrationCounterPrefix = "u2fregistrationcounter"
	u2fSignChallengePrefix       = "u2fsignchallenge"
)
