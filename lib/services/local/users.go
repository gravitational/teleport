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
	"encoding/json"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/go-cmp/cmp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

// IdentityService is responsible for managing web users and currently
// user accounts as well
type IdentityService struct {
	backend.Backend
	log logrus.FieldLogger
}

// NewIdentityService returns a new instance of IdentityService object
func NewIdentityService(backend backend.Backend) *IdentityService {
	return &IdentityService{
		Backend: backend,
		log:     logrus.WithField(trace.Component, "identity"),
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
	if err := services.ValidateUser(user); err != nil {
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
	if err := services.ValidateUser(user); err != nil {
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
	if err := services.ValidateUser(user); err != nil {
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
		suffix := bytes.TrimPrefix(item.Key, append(startKey, byte(backend.Separator)))
		uitems.Set(string(suffix), item) // Result of Set i
	}
	u, err := userFromUserItems(user, uitems)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

func (s *IdentityService) upsertLocalAuthSecrets(user string, auth types.LocalAuthSecrets) error {
	if len(auth.PasswordHash) > 0 {
		err := s.UpsertPasswordHash(user, auth.PasswordHash)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for _, d := range auth.MFA {
		if err := s.UpsertMFADevice(context.TODO(), user, d); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserByOIDCIdentity(id types.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetOIDCIdentities() {
			if cmp.Equal(uid, &id) {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with identity %q not found", &id)
}

// GetUserBySAMLCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserBySAMLIdentity(id types.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetSAMLIdentities() {
			if cmp.Equal(uid, &id) {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with identity %q not found", &id)
}

// GetUserByGithubIdentity returns the first found user with specified Github identity
func (s *IdentityService) GetUserByGithubIdentity(id types.ExternalIdentity) (services.User, error) {
	users, err := s.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, uid := range u.GetGithubIdentities() {
			if cmp.Equal(uid, &id) {
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
// Deprecated: HOTP use is deprecated, use UpsertMFADevice instead.
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
// Deprecated: HOTP use is deprecated, use GetMFADevices instead.
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

func (s *IdentityService) UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}
	if err := d.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Check device Name for uniqueness.
	devs, err := s.GetMFADevices(ctx, user)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, dd := range devs {
		// Same ID and Name is OK - it means update existing resource.
		// Different Id and same Name is not OK - it means a duplicate device.
		if d.Metadata.Name == dd.Metadata.Name && d.Id != dd.Id {
			return trace.AlreadyExists("MFA device with name %q already exists with ID %q", dd.Metadata.Name, dd.Id)
		}
	}

	value, err := json.Marshal(d)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.Key(webPrefix, usersPrefix, user, mfaDevicePrefix, d.Id),
		Value: value,
	}

	if _, err := s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) DeleteMFADevice(ctx context.Context, user, id string) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}
	if id == "" {
		return trace.BadParameter("missing parameter id")
	}

	err := s.Delete(ctx, backend.Key(webPrefix, usersPrefix, user, mfaDevicePrefix, id))
	return trace.Wrap(err)
}

func (s *IdentityService) GetMFADevices(ctx context.Context, user string) ([]*types.MFADevice, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	startKey := backend.Key(webPrefix, usersPrefix, user, mfaDevicePrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	devices := make([]*types.MFADevice, 0, len(result.Items))
	for _, item := range result.Items {
		var d types.MFADevice
		if err := json.Unmarshal(item.Value, &d); err != nil {
			return nil, trace.Wrap(err)
		}
		devices = append(devices, &d)
	}
	return devices, nil
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
func (s *IdentityService) UpsertOIDCConnector(ctx context.Context, connector services.OIDCConnector) error {
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
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteOIDCConnector deletes OIDC Connector by name
func (s *IdentityService) DeleteOIDCConnector(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	err := s.Delete(ctx, backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, name))
	return trace.Wrap(err)
}

// GetOIDCConnector returns OIDC connector data, parameter 'withSecrets'
// includes or excludes client secret from return results
func (s *IdentityService) GetOIDCConnector(ctx context.Context, name string, withSecrets bool) (services.OIDCConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, name))
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
		conn.SetGoogleServiceAccount("")
	}
	return conn, nil
}

// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (s *IdentityService) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]services.OIDCConnector, error) {
	startKey := backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
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
			conn.SetGoogleServiceAccount("")
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
func (s *IdentityService) UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
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
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSAMLConnector deletes SAML Connector by name
func (s *IdentityService) DeleteSAMLConnector(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	err := s.Delete(ctx, backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, name))
	return trace.Wrap(err)
}

// GetSAMLConnector returns SAML connector data,
// withSecrets includes or excludes secrets from return results
func (s *IdentityService) GetSAMLConnector(ctx context.Context, name string, withSecrets bool) (services.SAMLConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, name))
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
func (s *IdentityService) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]services.SAMLConnector, error) {
	startKey := backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
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
func (s *IdentityService) UpsertGithubConnector(ctx context.Context, connector services.GithubConnector) error {
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
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnectors returns all configured Github connectors
func (s *IdentityService) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]services.GithubConnector, error) {
	startKey := backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
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
func (s *IdentityService) GetGithubConnector(ctx context.Context, name string, withSecrets bool) (services.GithubConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, name))
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
func (s *IdentityService) DeleteGithubConnector(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	return trace.Wrap(s.Delete(ctx, backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, name)))
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
	webPrefix              = "web"
	usersPrefix            = "users"
	sessionsPrefix         = "sessions"
	attemptsPrefix         = "attempts"
	pwdPrefix              = "pwd"
	hotpPrefix             = "hotp"
	connectorsPrefix       = "connectors"
	oidcPrefix             = "oidc"
	samlPrefix             = "saml"
	githubPrefix           = "github"
	requestsPrefix         = "requests"
	u2fRegChalPrefix       = "adduseru2fchallenges"
	usedTOTPPrefix         = "used_totp"
	usedTOTPTTL            = 30 * time.Second
	mfaDevicePrefix        = "mfa"
	u2fSignChallengePrefix = "u2fsignchallenge"

	// DELETE IN 7.0: these prefixes are migrated to mfaDevicePrefix in 6.0 on
	// first startup.
	totpPrefix                   = "totp"
	u2fRegistrationPrefix        = "u2fregistration"
	u2fRegistrationCounterPrefix = "u2fregistrationcounter"
)
