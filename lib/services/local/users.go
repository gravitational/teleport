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
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/types"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// GlobalSessionDataMaxEntries represents the maximum number of in-flight
// global WebAuthn challenges for a given scope.
// Attempting to write more instances than the max limit causes an error.
// The limit is enforced separately by Auth Server instances.
var GlobalSessionDataMaxEntries = 5000 // arbitrary

// IdentityService is responsible for managing web users and currently
// user accounts as well
type IdentityService struct {
	backend.Backend
	log        logrus.FieldLogger
	bcryptCost int
}

// NewIdentityService returns a new instance of IdentityService object
func NewIdentityService(backend backend.Backend) *IdentityService {
	return &IdentityService{
		Backend:    backend,
		log:        logrus.WithField(trace.Component, "identity"),
		bcryptCost: bcrypt.DefaultCost,
	}
}

// NewTestIdentityService returns a new instance of IdentityService object to be
// used in tests. It will use weaker cryptography to minimize the time it takes
// to perform flakiness tests and decrease the probability of timeouts.
func NewTestIdentityService(backend backend.Backend) *IdentityService {
	if !testing.Testing() {
		// Don't allow using weak cryptography in production.
		panic("Attempted to create a test identity service outside of a test")
	}
	s := NewIdentityService(backend)
	s.bcryptCost = bcrypt.MinCost
	return s
}

// DeleteAllUsers deletes all users
func (s *IdentityService) DeleteAllUsers() error {
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// GetUsers returns a list of users registered with the local auth server
func (s *IdentityService) GetUsers(withSecrets bool) ([]types.User, error) {
	if withSecrets {
		return s.getUsersWithSecrets()
	}
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []types.User
	for _, item := range result.Items {
		if !item.Key.HasSuffix(backend.Key(paramsPrefix)) {
			continue
		}
		u, err := services.UnmarshalUser(
			item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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

func (s *IdentityService) getUsersWithSecrets() ([]types.User, error) {
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	result, err := s.GetRange(context.TODO(), startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	collected, _, err := collectUserItems(result.Items)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	users := make([]types.User, 0, len(collected))
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
func (s *IdentityService) CreateUser(user types.User) error {
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

	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
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
func (s *IdentityService) UpdateUser(ctx context.Context, user types.User) error {
	if err := services.ValidateUser(user); err != nil {
		return trace.Wrap(err)
	}

	// Confirm user exists before updating.
	if _, err := s.GetUser(user.GetName(), false); err != nil {
		return trace.Wrap(err)
	}

	rev := user.GetRevision()
	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:    value,
		Expires:  user.Expiry(),
		ID:       user.GetResourceID(),
		Revision: rev,
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

// UpdateAndSwapUser reads an existing user, runs `fn` against it and writes the
// result to storage. Return `false` from `fn` to avoid storage changes.
// Roughly equivalent to [GetUser] followed by [CompareAndSwapUser].
// Returns the storage user.
func (s *IdentityService) UpdateAndSwapUser(ctx context.Context, user string, withSecrets bool, fn func(types.User) (changed bool, err error)) (types.User, error) {
	u, items, err := s.getUser(ctx, user, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Take a "copy" of `u`. It is never modified.
	existing, err := userFromUserItems(user, *items)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch changed, err := fn(u); {
	case err != nil:
		return nil, trace.Wrap(err)
	case !changed:
		// Return user before modifications.
		return existing, nil
	}

	// Don't write secrets if we didn't read secrets.
	if !withSecrets {
		u = u.WithoutSecrets().(types.User)
	}

	if err := s.CompareAndSwapUser(ctx, u, existing); err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// UpsertUser updates parameters about user, or creates an entry if not exist.
func (s *IdentityService) UpsertUser(user types.User) error {
	if err := services.ValidateUser(user); err != nil {
		return trace.Wrap(err)
	}
	rev := user.GetRevision()
	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:    value,
		Expires:  user.Expiry(),
		ID:       user.GetResourceID(),
		Revision: rev,
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

// CompareAndSwapUser updates a user, but fails if the value (as exists in the
// backend) differs from the provided `existing` value. If the existing value
// matches, returns no error, otherwise returns `trace.CompareFailed`.
func (s *IdentityService) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	if err := services.ValidateUser(new); err != nil {
		return trace.Wrap(err)
	}

	newRaw, ok := new.WithoutSecrets().(types.User)
	if !ok {
		return trace.BadParameter("Invalid user type %T", new)
	}
	rev := new.GetRevision()
	newValue, err := services.MarshalUser(newRaw)
	if err != nil {
		return trace.Wrap(err)
	}
	newItem := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, new.GetName(), paramsPrefix),
		Value:    newValue,
		Expires:  new.Expiry(),
		ID:       new.GetResourceID(),
		Revision: rev,
	}

	existingRaw, ok := existing.WithoutSecrets().(types.User)
	if !ok {
		return trace.BadParameter("Invalid user type %T", existing)
	}
	existingValue, err := services.MarshalUser(existingRaw)
	if err != nil {
		return trace.Wrap(err)
	}
	existingItem := backend.Item{
		Key:     backend.NewKey(webPrefix, usersPrefix, existing.GetName(), paramsPrefix),
		Value:   existingValue,
		Expires: existing.Expiry(),
		ID:      existing.GetResourceID(),
	}

	_, err = s.CompareAndSwap(ctx, existingItem, newItem)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("user %v did not match expected existing value", new.GetName())
		}
		return trace.Wrap(err)
	}

	if auth := new.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(new.GetName(), *auth); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetUser returns a user by name
func (s *IdentityService) GetUser(user string, withSecrets bool) (types.User, error) {
	u, _, err := s.getUser(context.TODO(), user, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

func (s *IdentityService) getUser(ctx context.Context, user string, withSecrets bool) (types.User, *userItems, error) {
	if user == "" {
		return nil, nil, trace.BadParameter("missing user name")
	}

	if withSecrets {
		u, items, err := s.getUserWithSecrets(ctx, user)
		return u, items, trace.Wrap(err)
	}

	item, err := s.Get(ctx, backend.NewKey(webPrefix, usersPrefix, user, paramsPrefix))
	if err != nil {
		return nil, nil, trace.NotFound("user %q not found", user)
	}

	u, err := services.UnmarshalUser(
		item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if !withSecrets {
		u.SetLocalAuth(nil)
	}
	return u, &userItems{params: item}, nil
}

func (s *IdentityService) getUserWithSecrets(ctx context.Context, user string) (types.User, *userItems, error) {
	startKey := backend.ExactKey(webPrefix, usersPrefix, user)
	endKey := backend.RangeEnd(startKey)
	result, err := s.GetRange(ctx, startKey, endKey, backend.NoLimit)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var items userItems
	for _, item := range result.Items {
		suffix := item.Key.TrimPrefix(startKey)
		items.Set(suffix.String(), item) // Result of Set i
	}

	u, err := userFromUserItems(user, items)
	return u, &items, trace.Wrap(err)
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
	if auth.Webauthn != nil {
		if err := s.UpsertWebauthnLocalAuth(context.TODO(), user, auth.Webauthn); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserByOIDCIdentity(id types.ExternalIdentity) (types.User, error) {
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

// GetUserBySAMLIdentity returns a user by it's specified OIDC Identity, returns
// first user specified with this identity.
func (s *IdentityService) GetUserBySAMLIdentity(id types.ExternalIdentity) (types.User, error) {
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
func (s *IdentityService) GetUserByGithubIdentity(id types.ExternalIdentity) (types.User, error) {
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
	// each user has multiple related entries in the backend,
	// so use DeleteRange to make sure we get them all
	startKey := backend.ExactKey(webPrefix, usersPrefix, user)
	err = s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	return trace.Wrap(err)
}

// UpsertPasswordHash upserts user password hash
func (s *IdentityService) UpsertPasswordHash(username string, hash []byte) error {
	userPrototype, err := types.NewUser(username)
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
		Key:   backend.NewKey(webPrefix, usersPrefix, username, pwdPrefix),
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
	item, err := s.Get(context.TODO(), backend.NewKey(webPrefix, usersPrefix, user, pwdPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user %q is not found", user)
		}
		return nil, trace.Wrap(err)
	}
	return item.Value, nil
}

// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
// during the 30 second window it's valid.
func (s *IdentityService) UpsertUsedTOTPToken(user string, otpToken string) error {
	if user == "" {
		return trace.BadParameter("missing user name")
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, usersPrefix, user, usedTOTPPrefix),
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
	item, err := s.Get(context.TODO(), backend.NewKey(webPrefix, usersPrefix, user, usedTOTPPrefix))
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
	return s.Delete(context.TODO(), backend.NewKey(webPrefix, usersPrefix, user, usedTOTPPrefix))
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
		Key:     backend.NewKey(webPrefix, usersPrefix, user, attemptsPrefix, uuid.New().String()),
		Value:   value,
		Expires: backend.Expiry(s.Clock(), ttl),
	}
	_, err = s.Put(context.TODO(), item)
	return trace.Wrap(err)
}

// GetUserLoginAttempts returns user login attempts
func (s *IdentityService) GetUserLoginAttempts(user string) ([]services.LoginAttempt, error) {
	startKey := backend.ExactKey(webPrefix, usersPrefix, user, attemptsPrefix)
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
	startKey := backend.ExactKey(webPrefix, usersPrefix, user, attemptsPrefix)
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
	hash, err := utils.BcryptFromPassword(password, s.bcryptCost)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertPasswordHash(user, hash)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *IdentityService) UpsertWebauthnLocalAuth(ctx context.Context, user string, wla *types.WebauthnLocalAuth) error {
	switch {
	case user == "":
		return trace.BadParameter("missing parameter user")
	case wla == nil:
		return trace.BadParameter("missing parameter webauthn local auth")
	}
	if err := wla.Check(); err != nil {
		return trace.Wrap(err)
	}

	// Marshal both values before writing, we want to minimize the chances of
	// having to "undo" a write below.
	wlaJSON, err := json.Marshal(wla)
	if err != nil {
		return trace.Wrap(err, "marshal webauthn local auth")
	}
	userJSON, err := json.Marshal(&wanpb.User{
		TeleportUser: user,
	})
	if err != nil {
		return trace.Wrap(err, "marshal webauthn user")
	}

	// Write WebauthnLocalAuth.
	wlaKey := webauthnLocalAuthKey(user)
	if _, err = s.Put(ctx, backend.Item{
		Key:   wlaKey,
		Value: wlaJSON,
	}); err != nil {
		return trace.Wrap(err, "writing webauthn local auth")
	}

	// Write wla.UserID->user mapping, used for usernameless logins.
	if _, err = s.Put(ctx, backend.Item{
		Key:   webauthnUserKey(wla.UserID),
		Value: userJSON,
	}); err != nil {
		// Undo the first write if the one below fails.
		// This is a best-effort attempt, as both the 2nd write and the delete may
		// fail (it's even likely that both do, depending on the error).
		// lib/auth/webauthn is prepared to deal with eventual inconsistencies
		// between "web/users/.../webauthnlocalauth" and "webauthn/users/" keys.
		if err := s.Delete(ctx, wlaKey); err != nil {
			s.log.WithError(err).Warn("Failed to undo WebauthnLocalAuth update")
		}
		return trace.Wrap(err, "writing webauthn user")
	}

	return nil
}

func (s *IdentityService) GetWebauthnLocalAuth(ctx context.Context, user string) (*types.WebauthnLocalAuth, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	item, err := s.Get(ctx, webauthnLocalAuthKey(user))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wal := &types.WebauthnLocalAuth{}
	return wal, trace.Wrap(json.Unmarshal(item.Value, wal))
}

func (s *IdentityService) GetTeleportUserByWebauthnID(ctx context.Context, webID []byte) (string, error) {
	if len(webID) == 0 {
		return "", trace.BadParameter("missing parameter webID")
	}

	item, err := s.Get(ctx, webauthnUserKey(webID))
	if err != nil {
		return "", trace.Wrap(err)
	}
	user := &wanpb.User{}
	if err := json.Unmarshal(item.Value, user); err != nil {
		return "", trace.Wrap(err)
	}
	return user.TeleportUser, nil
}

func webauthnLocalAuthKey(user string) backend.Key {
	return backend.NewKey(webPrefix, usersPrefix, user, webauthnLocalAuthPrefix)
}

func webauthnUserKey(id []byte) backend.Key {
	key := base64.RawURLEncoding.EncodeToString(id)
	return backend.NewKey(webauthnPrefix, usersPrefix, key)
}

func (s *IdentityService) UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wanpb.SessionData) error {
	switch {
	case user == "":
		return trace.BadParameter("missing parameter user")
	case sessionID == "":
		return trace.BadParameter("missing parameter sessionID")
	case sd == nil:
		return trace.BadParameter("missing parameter sd")
	}

	value, err := json.Marshal(sd)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(ctx, backend.Item{
		Key:     sessionDataKey(user, sessionID),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(defaults.WebauthnChallengeTimeout),
	})
	return trace.Wrap(err)
}

func (s *IdentityService) GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wanpb.SessionData, error) {
	switch {
	case user == "":
		return nil, trace.BadParameter("missing parameter user")
	case sessionID == "":
		return nil, trace.BadParameter("missing parameter sessionID")
	}

	item, err := s.Get(ctx, sessionDataKey(user, sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sd := &wanpb.SessionData{}
	return sd, trace.Wrap(json.Unmarshal(item.Value, sd))
}

func (s *IdentityService) DeleteWebauthnSessionData(ctx context.Context, user, sessionID string) error {
	switch {
	case user == "":
		return trace.BadParameter("missing parameter user")
	case sessionID == "":
		return trace.BadParameter("missing parameter sessionID")
	}

	return trace.Wrap(s.Delete(ctx, sessionDataKey(user, sessionID)))
}

func sessionDataKey(user, sessionID string) backend.Key {
	return backend.NewKey(webPrefix, usersPrefix, user, webauthnSessionData, sessionID)
}

// globalSessionDataLimiter keeps a count of in-flight session data challenges
// over a period of time.
type globalSessionDataLimiter struct {
	// Clock is public so it may be overwritten by tests.
	Clock clockwork.Clock
	// ResetPeriod is public so it may be overwritten by tests.
	ResetPeriod time.Duration
	// mu guards the fields below it.
	mu         sync.Mutex
	scopeCount map[string]int
	lastReset  time.Time
}

func (l *globalSessionDataLimiter) add(scope string, n int) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Reset counters to account for key expiration.
	now := l.Clock.Now()
	if now.Sub(l.lastReset) >= l.ResetPeriod {
		for k := range l.scopeCount {
			l.scopeCount[k] = 0
		}
		l.lastReset = now
	}

	v := l.scopeCount[scope] + n
	if v < 0 {
		v = 0
	}
	l.scopeCount[scope] = v
	return v
}

var sdLimiter = &globalSessionDataLimiter{
	Clock: clockwork.NewRealClock(),
	// Make ResetPeriod larger than the challenge expiration, so we are a bit
	// more conservative than storage.
	ResetPeriod: defaults.WebauthnGlobalChallengeTimeout + 10*time.Second,
	scopeCount:  make(map[string]int),
}

func (s *IdentityService) UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wanpb.SessionData) error {
	switch {
	case scope == "":
		return trace.BadParameter("missing parameter scope")
	case id == "":
		return trace.BadParameter("missing parameter id")
	case sd == nil:
		return trace.BadParameter("missing parameter sd")
	}

	// Marshal before checking limiter, in case this fails.
	value, err := json.Marshal(sd)
	if err != nil {
		return trace.Wrap(err)
	}

	// Are we within the limits for the current time window?
	if entries := sdLimiter.add(scope, 1); entries > GlobalSessionDataMaxEntries {
		sdLimiter.add(scope, -1) // Request denied, adjust accordingly
		return trace.LimitExceeded("too many in-flight challenges")
	}

	if _, err = s.Put(ctx, backend.Item{
		Key:     globalSessionDataKey(scope, id),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(defaults.WebauthnGlobalChallengeTimeout),
	}); err != nil {
		sdLimiter.add(scope, -1) // Don't count eventual write failures
		return trace.Wrap(err)
	}
	return nil
}

func (s *IdentityService) GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wanpb.SessionData, error) {
	switch {
	case scope == "":
		return nil, trace.BadParameter("missing parameter scope")
	case id == "":
		return nil, trace.BadParameter("missing parameter id")
	}

	item, err := s.Get(ctx, globalSessionDataKey(scope, id))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sd := &wanpb.SessionData{}
	return sd, trace.Wrap(json.Unmarshal(item.Value, sd))
}

func (s *IdentityService) DeleteGlobalWebauthnSessionData(ctx context.Context, scope, id string) error {
	switch {
	case scope == "":
		return trace.BadParameter("missing parameter scope")
	case id == "":
		return trace.BadParameter("missing parameter id")
	}

	if err := s.Delete(ctx, globalSessionDataKey(scope, id)); err != nil {
		return trace.Wrap(err)
	}

	sdLimiter.add(scope, -1)
	return nil
}

func globalSessionDataKey(scope, id string) backend.Key {
	return backend.NewKey(webauthnPrefix, webauthnGlobalSessionData, scope, id)
}

func (s *IdentityService) UpsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}
	if err := d.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	devs, err := s.GetMFADevices(ctx, user, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, dd := range devs {
		switch {
		case d.Metadata.Name == dd.Metadata.Name && d.Id == dd.Id:
			// OK. Same Name and ID means we are doing an update.
			continue
		case d.Metadata.Name == dd.Metadata.Name && d.Id != dd.Id:
			// NOK. Same Name but different ID means it's a duplicate device.
			return trace.AlreadyExists("MFA device with name %q already exists", d.Metadata.Name)
		}

		// Disallow duplicate credential IDs if the new device is Webauthn.
		if d.GetWebauthn() == nil {
			continue
		}
		id1, ok := getCredentialID(d)
		if !ok {
			continue
		}
		id2, ok := getCredentialID(dd)
		if !ok {
			continue
		}
		if bytes.Equal(id1, id2) {
			return trace.AlreadyExists("credential ID already in use by device %q", dd.Metadata.Name)
		}
	}

	rev := d.GetRevision()
	value, err := json.Marshal(d)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user, mfaDevicePrefix, d.Id),
		Value:    value,
		Revision: rev,
	}

	if _, err := s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getCredentialID(d *types.MFADevice) ([]byte, bool) {
	switch d := d.Device.(type) {
	case *types.MFADevice_U2F:
		return d.U2F.KeyHandle, true
	case *types.MFADevice_Webauthn:
		return d.Webauthn.CredentialId, true
	}
	return nil, false
}

func (s *IdentityService) DeleteMFADevice(ctx context.Context, user, id string) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}
	if id == "" {
		return trace.BadParameter("missing parameter id")
	}

	err := s.Delete(ctx, backend.NewKey(webPrefix, usersPrefix, user, mfaDevicePrefix, id))
	return trace.Wrap(err)
}

func (s *IdentityService) GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	startKey := backend.ExactKey(webPrefix, usersPrefix, user, mfaDevicePrefix)
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
		if !withSecrets {
			devWithoutSensitiveData, err := d.WithoutSensitiveData()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			d = *devWithoutSensitiveData
		}
		devices = append(devices, &d)
	}
	return devices, nil
}

// UpsertOIDCConnector upserts OIDC Connector
func (s *IdentityService) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error {
	rev := connector.GetRevision()
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		ID:       connector.GetResourceID(),
		Revision: rev,
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
	err := s.Delete(ctx, backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, name))
	return trace.Wrap(err)
}

// GetOIDCConnector returns OIDC connector data, parameter 'withSecrets'
// includes or excludes client secret from return results
func (s *IdentityService) GetOIDCConnector(ctx context.Context, name string, withSecrets bool) (types.OIDCConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, name))
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
func (s *IdentityService) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error) {
	startKey := backend.ExactKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var connectors []types.OIDCConnector
	for _, item := range result.Items {
		conn, err := services.UnmarshalOIDCConnector(
			item.Value, services.WithExpires(item.Expires))
		if err != nil {
			logrus.
				WithError(err).
				WithField("key", item.Key).
				Errorf("Error unmarshaling OIDC Connector")
			continue
		}
		if !withSecrets {
			conn.SetClientSecret("")
			conn.SetGoogleServiceAccount("")
		}
		connectors = append(connectors, conn)
	}
	return connectors, nil
}

// CreateOIDCAuthRequest creates new auth request
func (s *IdentityService) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	buf := new(bytes.Buffer)
	if err := (&jsonpb.Marshaler{}).Marshal(buf, &req); err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, requestsPrefix, req.StateToken),
		Value:   buf.Bytes(),
		Expires: backend.Expiry(s.Clock(), ttl),
	}
	if _, err := s.Create(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCAuthRequest returns OIDC auth request
func (s *IdentityService) GetOIDCAuthRequest(ctx context.Context, stateToken string) (*types.OIDCAuthRequest, error) {
	if stateToken == "" {
		return nil, trace.BadParameter("missing parameter stateToken")
	}
	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, requestsPrefix, stateToken))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := new(types.OIDCAuthRequest)
	if err := jsonpb.Unmarshal(bytes.NewReader(item.Value), req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// UpsertSAMLConnector upserts SAML Connector
func (s *IdentityService) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if err := services.ValidateSAMLConnector(connector, nil); err != nil {
		return trace.Wrap(err)
	}
	rev := connector.GetRevision()
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: rev,
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
	err := s.Delete(ctx, backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, name))
	return trace.Wrap(err)
}

// GetSAMLConnector returns SAML connector data,
// withSecrets includes or excludes secrets from return results
func (s *IdentityService) GetSAMLConnector(ctx context.Context, name string, withSecrets bool) (types.SAMLConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, name))
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
func (s *IdentityService) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error) {
	startKey := backend.ExactKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var connectors []types.SAMLConnector
	for _, item := range result.Items {
		conn, err := services.UnmarshalSAMLConnector(
			item.Value, services.WithExpires(item.Expires))
		if err != nil {
			logrus.
				WithError(err).
				WithField("key", item.Key).
				Errorf("Error unmarshaling SAML Connector")
			continue
		}
		if !withSecrets {
			keyPair := conn.GetSigningKeyPair()
			if keyPair != nil {
				keyPair.PrivateKey = ""
				conn.SetSigningKeyPair(keyPair)
			}
		}
		connectors = append(connectors, conn)
	}
	return connectors, nil
}

// CreateSAMLAuthRequest creates new auth request
func (s *IdentityService) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	buf := new(bytes.Buffer)
	if err := (&jsonpb.Marshaler{}).Marshal(buf, &req); err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, requestsPrefix, req.ID),
		Value:   buf.Bytes(),
		Expires: backend.Expiry(s.Clock(), ttl),
	}
	if _, err := s.Create(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSAMLAuthRequest returns SAML auth request if found
func (s *IdentityService) GetSAMLAuthRequest(ctx context.Context, id string) (*types.SAMLAuthRequest, error) {
	if id == "" {
		return nil, trace.BadParameter("missing parameter id")
	}
	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, requestsPrefix, id))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := new(types.SAMLAuthRequest)
	if err := jsonpb.Unmarshal(bytes.NewReader(item.Value), req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// CreateSSODiagnosticInfo creates new SAML diagnostic info record.
func (s *IdentityService) CreateSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string, entry types.SSODiagnosticInfo) error {
	if authRequestID == "" {
		return trace.BadParameter("missing parameter authRequestID")
	}

	switch authKind {
	case types.KindSAML, types.KindGithub, types.KindOIDC:
		// nothing to do
	default:
		return trace.BadParameter("unsupported authKind %q", authKind)
	}

	jsonValue, err := json.Marshal(entry)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, authKind, requestsTracePrefix, authRequestID),
		Value:   jsonValue,
		Expires: backend.Expiry(s.Clock(), time.Minute*15),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSSODiagnosticInfo returns SSO diagnostic info records.
func (s *IdentityService) GetSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string) (*types.SSODiagnosticInfo, error) {
	if authRequestID == "" {
		return nil, trace.BadParameter("missing parameter authRequestID")
	}

	switch authKind {
	case types.KindSAML, types.KindGithub, types.KindOIDC:
		// nothing to do
	default:
		return nil, trace.BadParameter("unsupported authKind %q", authKind)
	}

	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, authKind, requestsTracePrefix, authRequestID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req types.SSODiagnosticInfo
	if err := json.Unmarshal(item.Value, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &req, nil
}

// UpsertGithubConnector creates or updates a Github connector
func (s *IdentityService) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	rev := connector.GetRevision()
	value, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		ID:       connector.GetResourceID(),
		Revision: rev,
	}
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnectors returns all configured Github connectors
func (s *IdentityService) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	startKey := backend.ExactKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var connectors []types.GithubConnector
	for _, item := range result.Items {
		connector, err := services.UnmarshalGithubConnector(item.Value)
		if err != nil {
			logrus.
				WithError(err).
				WithField("key", item.Key).
				Errorf("Error unmarshaling GitHub Connector")
			continue
		}
		if !withSecrets {
			connector.SetClientSecret("")
		}
		connectors = append(connectors, connector)
	}
	return connectors, nil
}

// GetGithubConnector returns a particular Github connector.
func (s *IdentityService) GetGithubConnector(ctx context.Context, name string, withSecrets bool) (types.GithubConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, name))
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
	return trace.Wrap(s.Delete(ctx, backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, name)))
}

// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
func (s *IdentityService) CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	buf := new(bytes.Buffer)
	if err := (&jsonpb.Marshaler{}).Marshal(buf, &req); err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, requestsPrefix, req.StateToken),
		Value:   buf.Bytes(),
		Expires: req.Expiry(),
	}
	if _, err := s.Create(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubAuthRequest retrieves Github auth request by the token
func (s *IdentityService) GetGithubAuthRequest(ctx context.Context, stateToken string) (*types.GithubAuthRequest, error) {
	if stateToken == "" {
		return nil, trace.BadParameter("missing parameter stateToken")
	}
	item, err := s.Get(ctx, backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, requestsPrefix, stateToken))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := new(types.GithubAuthRequest)
	if err := jsonpb.Unmarshal(bytes.NewReader(item.Value), req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// GetRecoveryCodes returns user's recovery codes.
func (s *IdentityService) GetRecoveryCodes(ctx context.Context, user string, withSecrets bool) (*types.RecoveryCodesV1, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	item, err := s.Get(ctx, backend.NewKey(webPrefix, usersPrefix, user, recoveryCodesPrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var rc types.RecoveryCodesV1
	if err := json.Unmarshal(item.Value, &rc); err != nil {
		return nil, trace.Wrap(err)
	}

	if !withSecrets {
		rc.Spec.Codes = nil
	}

	return &rc, nil
}

// UpsertRecoveryCodes creates or updates user's account recovery codes.
// Each recovery code are hashed before upsert.
func (s *IdentityService) UpsertRecoveryCodes(ctx context.Context, user string, recovery *types.RecoveryCodesV1) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}

	if err := recovery.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := json.Marshal(recovery)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:   backend.NewKey(webPrefix, usersPrefix, user, recoveryCodesPrefix),
		Value: value,
	}

	_, err = s.Put(ctx, item)
	return trace.Wrap(err)
}

// CreateUserRecoveryAttempt creates new user recovery attempt.
func (s *IdentityService) CreateUserRecoveryAttempt(ctx context.Context, user string, attempt *types.RecoveryAttempt) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}

	if err := attempt.Check(); err != nil {
		return trace.Wrap(err)
	}

	value, err := json.Marshal(attempt)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(webPrefix, usersPrefix, user, recoveryAttemptsPrefix, uuid.New().String()),
		Value:   value,
		Expires: attempt.Expires,
	}

	_, err = s.Create(ctx, item)
	return trace.Wrap(err)
}

// GetUserRecoveryAttempts returns users recovery attempts.
func (s *IdentityService) GetUserRecoveryAttempts(ctx context.Context, user string) ([]*types.RecoveryAttempt, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	startKey := backend.ExactKey(webPrefix, usersPrefix, user, recoveryAttemptsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]*types.RecoveryAttempt, len(result.Items))
	for i, item := range result.Items {
		var a types.RecoveryAttempt
		if err := json.Unmarshal(item.Value, &a); err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = &a
	}

	sort.Sort(recoveryAttemptsChronologically(out))

	return out, nil
}

// DeleteUserRecoveryAttempts removes all recovery attempts of a user.
func (s *IdentityService) DeleteUserRecoveryAttempts(ctx context.Context, user string) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}

	startKey := backend.ExactKey(webPrefix, usersPrefix, user, recoveryAttemptsPrefix)
	return trace.Wrap(s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// recoveryAttemptsChronologically sorts recovery attempts by oldest to latest time.
type recoveryAttemptsChronologically []*types.RecoveryAttempt

func (s recoveryAttemptsChronologically) Len() int {
	return len(s)
}

// Less stacks latest attempts to the end of the list.
func (s recoveryAttemptsChronologically) Less(i, j int) bool {
	return s[i].Time.Before(s[j].Time)
}

func (s recoveryAttemptsChronologically) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// UpsertKeyAttestationData upserts a verified public key attestation response.
func (s *IdentityService) UpsertKeyAttestationData(ctx context.Context, attestationData *keys.AttestationData, ttl time.Duration) error {
	value, err := json.Marshal(attestationData)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate public key.
	if _, err := x509.ParsePKIXPublicKey(attestationData.PublicKeyDER); err != nil {
		return trace.Wrap(err)
	}

	key := keyAttestationDataFingerprint(attestationData.PublicKeyDER)
	item := backend.Item{
		Key:     backend.NewKey(attestationsPrefix, key),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(ttl),
	}
	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetKeyAttestationData gets a verified public key attestation response.
func (s *IdentityService) GetKeyAttestationData(ctx context.Context, pubDER []byte) (*keys.AttestationData, error) {
	if pubDER == nil {
		return nil, trace.BadParameter("missing parameter pubDER")
	}

	key := keyAttestationDataFingerprint(pubDER)
	item, err := s.Get(ctx, backend.NewKey(attestationsPrefix, key))

	if trace.IsNotFound(err) {
		return nil, trace.NotFound("hardware key attestation not found")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	var resp keys.AttestationData
	if err := json.Unmarshal(item.Value, &resp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resp, nil
}

func keyAttestationDataFingerprint(pubDER []byte) string {
	sha256sum := sha256.Sum256(pubDER)
	encodedSHA := base64.RawURLEncoding.EncodeToString(sha256sum[:])
	return encodedSHA
}

const (
	webPrefix                   = "web"
	usersPrefix                 = "users"
	sessionsPrefix              = "sessions"
	attemptsPrefix              = "attempts"
	pwdPrefix                   = "pwd"
	connectorsPrefix            = "connectors"
	oidcPrefix                  = "oidc"
	samlPrefix                  = "saml"
	githubPrefix                = "github"
	requestsPrefix              = "requests"
	requestsTracePrefix         = "requestsTrace"
	usedTOTPPrefix              = "used_totp"
	usedTOTPTTL                 = 30 * time.Second
	mfaDevicePrefix             = "mfa"
	webauthnPrefix              = "webauthn"
	webauthnGlobalSessionData   = "sessionData"
	webauthnLocalAuthPrefix     = "webauthnlocalauth"
	webauthnSessionData         = "webauthnsessiondata"
	recoveryCodesPrefix         = "recoverycodes"
	recoveryAttemptsPrefix      = "recoveryattempts"
	attestationsPrefix          = "key_attestations"
	assistantMessagePrefix      = "assistant_messages"
	assistantConversationPrefix = "assistant_conversations"
	userPreferencesPrefix       = "user_preferences"
)
