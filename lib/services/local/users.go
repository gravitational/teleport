/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
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
	logger           *slog.Logger
	bcryptCost       int
	notificationsSvc *NotificationsService
}

// NewIdentityService returns a new instance of IdentityService object
func NewIdentityService(backend backend.Backend) (*IdentityService, error) {
	notificationsSvc, err := NewNotificationsService(backend, backend.Clock())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &IdentityService{
		Backend:          backend,
		logger:           slog.With(teleport.ComponentKey, "identity"),
		bcryptCost:       bcrypt.DefaultCost,
		notificationsSvc: notificationsSvc,
	}, nil
}

// NewTestIdentityService returns a new instance of IdentityService object to be
// used in tests. It will use weaker cryptography to minimize the time it takes
// to perform flakiness tests and decrease the probability of timeouts.
func NewTestIdentityService(backend backend.Backend) (*IdentityService, error) {
	if !testing.Testing() {
		// Don't allow using weak cryptography in production.
		panic("Attempted to create a test identity service outside of a test")
	}

	s, err := NewIdentityService(backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.bcryptCost = bcrypt.MinCost
	return s, nil
}

// DeleteAllUsers deletes all users
func (s *IdentityService) DeleteAllUsers(ctx context.Context) error {
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	return trace.Wrap(s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// ListUsers returns a page of users.
func (s *IdentityService) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	rangeStart := backend.NewKey(webPrefix, usersPrefix).AppendKey(backend.KeyFromString(req.PageToken))
	rangeEnd := backend.RangeEnd(backend.ExactKey(webPrefix, usersPrefix))
	pageSize := req.PageSize

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	// Artificially inflate the limit to account for user secrets
	// which have the same prefix.
	limit := int(pageSize) * 4

	itemStream := backend.StreamRange(ctx, s.Backend, rangeStart, rangeEnd, limit)

	var userStream stream.Stream[*types.UserV2]
	if req.WithSecrets {
		userStream = s.streamUsersWithSecrets(itemStream)
	} else {
		userStream = s.streamUsersWithoutSecrets(itemStream)
	}

	if req.Filter != nil {
		userStream = stream.FilterMap(userStream, func(user *types.UserV2) (*types.UserV2, bool) {
			if !req.Filter.Match(user) {
				return nil, false
			}

			return user, true
		})
	}

	users, full := stream.Take(userStream, int(pageSize))

	var nextToken string
	if full && userStream.Next() {
		nextToken = nextUserToken(users[len(users)-1])
	}

	if err := userStream.Done(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &userspb.ListUsersResponse{
		Users:         users,
		NextPageToken: nextToken,
	}, nil
}

// nextUserToken returns the last token for the given user. This
// allows the listing operation to provide a token which doesn't divulge
// the next user in the list while still allowing listing to operate
// without missing any users.
func nextUserToken(user types.User) string {
	key := backend.RangeEnd(backend.ExactKey(user.GetName())).String()
	return strings.Trim(key, string(backend.Separator))
}

// streamUsersWithSecrets is a helper that converts a stream of backend items over the user key range into a stream
// of users along with their associated secrets.
func (s *IdentityService) streamUsersWithSecrets(itemStream stream.Stream[backend.Item]) stream.Stream[*types.UserV2] {
	type collector struct {
		items userItems
		name  string
	}

	var current collector

	collectorStream := stream.FilterMap(itemStream, func(item backend.Item) (collector, bool) {
		name, suffix, err := splitUsernameAndSuffix(item.Key)
		if err != nil {
			s.logger.WarnContext(context.Background(), "Failed to extract name/suffix for user item",
				"key", item.Key,
				"error", err,
			)
			return collector{}, false
		}

		if name == current.name {
			// we're already in the process of aggregating the items for this user, so just
			// store this item and continue on to the next one.
			current.items.Set(suffix, item)
			return collector{}, false
		}

		// we've reached a new user range, so take local ownership of the previous aggregator and
		// set up a new one to aggregate this new range.
		prev := current
		current = collector{
			name: name,
		}
		current.items.Set(suffix, item)

		if !prev.items.complete() {
			// previous aggregator was empty or malformed and can be discarded.
			return collector{}, false
		}

		return prev, true

	})

	// since a collector for a given user isn't yielded until the above stream reaches the *next*
	// user, that means the last user's collector is never yielded. we need to append a single
	// additional check to the stream that decides if it should yield the final collector.
	collectorStream = stream.Chain(collectorStream, stream.OnceFunc(func() (collector, error) {
		if !current.items.complete() {
			return collector{}, io.EOF
		}
		return current, nil
	}))

	userStream := stream.FilterMap(collectorStream, func(c collector) (*types.UserV2, bool) {
		user, err := userFromUserItems(c.name, c.items)
		if err != nil {
			s.logger.WarnContext(context.Background(), "Failed to build user from user item aggregator",
				"user", c.name,
				"error", err,
			)
			return nil, false
		}

		return user, true
	})

	return userStream
}

// streamUsersWithoutSecrets is a helper that converts a stream of backend items over the user range into a stream of
// user resources without any included secrets.
func (s *IdentityService) streamUsersWithoutSecrets(itemStream stream.Stream[backend.Item]) stream.Stream[*types.UserV2] {
	suffix := backend.NewKey(paramsPrefix)
	userStream := stream.FilterMap(itemStream, func(item backend.Item) (*types.UserV2, bool) {
		if !item.Key.HasSuffix(suffix) {
			return nil, false
		}

		user, err := services.UnmarshalUser(item.Value, services.WithRevision(item.Revision))
		if err != nil {
			s.logger.WarnContext(context.Background(), "Failed to unmarshal user",
				"key", item.Key,
				"error", err,
			)
			return nil, false
		}

		return user, true
	})

	return userStream
}

// GetUsers returns a list of users registered with the local auth server
func (s *IdentityService) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	if withSecrets {
		return s.getUsersWithSecrets(ctx)
	}
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []types.User
	for _, item := range result.Items {
		if !item.Key.HasSuffix(backend.NewKey(paramsPrefix)) {
			continue
		}
		u, err := services.UnmarshalUser(
			item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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

func (s *IdentityService) getUsersWithSecrets(ctx context.Context) ([]types.User, error) {
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
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
func (s *IdentityService) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}

	// Confirm user doesn't exist before creating.
	_, err := s.GetUser(ctx, user.GetName(), false)
	if !trace.IsNotFound(err) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.AlreadyExists("user %q already registered", user.GetName())
	}

	// In a typical case, we create users without passwords. However, it is
	// technically possible to create a user along with a password using a direct
	// RPC call or `tctl create`, so we need to support this case, too.
	auth := user.GetLocalAuth()
	if auth != nil {
		if len(auth.PasswordHash) > 0 {
			user.SetPasswordState(types.PasswordState_PASSWORD_STATE_SET)
		}
	} else {
		user.SetPasswordState(types.PasswordState_PASSWORD_STATE_UNSET)
	}

	s.buildAndSetWeakestMFADeviceKind(ctx, user, auth)

	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:   value,
		Expires: user.Expiry(),
	}

	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if auth != nil {
		if err = s.upsertLocalAuthSecrets(ctx, user.GetName(), *auth); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user.SetRevision(lease.Revision)
	return user, nil
}

// LegacyUpdateUser blindly updates an existing user. [IdentityService.UpdateUser] should be
// used instead so that optimistic locking prevents concurrent resource updates.
func (s *IdentityService) LegacyUpdateUser(ctx context.Context, user types.User) (types.User, error) {
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}

	// Confirm user exists before updating.
	if _, err := s.GetUser(ctx, user.GetName(), false); err != nil {
		return nil, trace.Wrap(err)
	}

	rev := user.GetRevision()

	// if the user has no local auth, we need to check if the user
	// was previously created and enrolled an MFA device.
	s.buildAndSetWeakestMFADeviceKind(ctx, user, user.GetLocalAuth())

	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:    value,
		Expires:  user.Expiry(),
		Revision: rev,
	}
	lease, err := s.Update(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if auth := user.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(ctx, user.GetName(), *auth); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user.SetRevision(lease.Revision)
	return user, nil
}

// UpdateUser updates an existing user if the revisions match.
func (s *IdentityService) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}

	s.buildAndSetWeakestMFADeviceKind(ctx, user, user.GetLocalAuth())

	rev := user.GetRevision()
	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:    value,
		Expires:  user.Expiry(),
		Revision: rev,
	}
	lease, err := s.Backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if auth := user.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(ctx, user.GetName(), *auth); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user.SetRevision(lease.Revision)
	return user, nil
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
func (s *IdentityService) UpsertUser(ctx context.Context, user types.User) (types.User, error) {
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}

	s.buildAndSetWeakestMFADeviceKind(ctx, user, user.GetLocalAuth())

	rev := user.GetRevision()
	value, err := services.MarshalUser(user.WithoutSecrets().(types.User))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:    value,
		Expires:  user.Expiry(),
		Revision: rev,
	}
	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if auth := user.GetLocalAuth(); auth != nil {
		if err = s.upsertLocalAuthSecrets(ctx, user.GetName(), *auth); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user.SetRevision(lease.Revision)
	return user, nil
}

// CompareAndSwapUser updates a user, but fails if the user (as exists in the
// backend) differs from the provided `existing` user. If the existing user
// matches, returns no error, otherwise returns `trace.CompareFailed`.
func (s *IdentityService) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	if new.GetName() != existing.GetName() {
		return trace.BadParameter("name mismatch between new and existing user")
	}
	if err := services.ValidateUser(new); err != nil {
		return trace.Wrap(err)
	}

	newWithoutSecrets, ok := new.WithoutSecrets().(types.User)
	if !ok {
		return trace.BadParameter("invalid new user type %T (this is a bug)", new)
	}

	existingWithoutSecrets, ok := existing.WithoutSecrets().(types.User)
	if !ok {
		return trace.BadParameter("invalid existing user type %T (this is a bug)", existing)
	}

	item := backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, new.GetName(), paramsPrefix),
		Value:    nil, // avoid marshaling new until we pass one comparison
		Expires:  new.Expiry(),
		Revision: "",
	}

	// one retry because ConditionalUpdate could occasionally spuriously fail,
	// another retry because a single retry would be weird
	const iterationLimit = 3
	for i := 0; i < iterationLimit; i++ {
		const withoutSecrets = false
		currentWithoutSecrets, err := s.GetUser(ctx, new.GetName(), withoutSecrets)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.CompareFailed("user %v did not match expected existing value", new.GetName())
			}
			return trace.Wrap(err)
		}

		if !services.UsersEquals(existingWithoutSecrets, currentWithoutSecrets) {
			return trace.CompareFailed("user %v did not match expected existing value", new.GetName())
		}

		s.buildAndSetWeakestMFADeviceKind(ctx, newWithoutSecrets, new.GetLocalAuth())

		if item.Value == nil {
			v, err := services.MarshalUser(newWithoutSecrets)
			if err != nil {
				return trace.Wrap(err)
			}
			item.Value = v
		}

		item.Revision = currentWithoutSecrets.GetRevision()

		if _, err = s.Backend.ConditionalUpdate(ctx, item); err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return trace.Wrap(err)
		}

		if auth := new.GetLocalAuth(); auth != nil {
			if err = s.upsertLocalAuthSecrets(ctx, new.GetName(), *auth); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}

	return trace.LimitExceeded("failed to update user within %v iterations", iterationLimit)
}

// GetUser returns a user by name
func (s *IdentityService) GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error) {
	u, _, err := s.getUser(ctx, user, withSecrets)
	return u, trace.Wrap(err)
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
		item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
		items.Set(suffix.Components(), item) // Result of Set i
	}

	u, err := userFromUserItems(user, items)
	return u, &items, trace.Wrap(err)
}

func (s *IdentityService) upsertLocalAuthSecrets(ctx context.Context, user string, auth types.LocalAuthSecrets) error {
	if len(auth.PasswordHash) > 0 {
		if err := s.upsertPasswordHash(user, auth.PasswordHash); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, d := range auth.MFA {
		if err := s.upsertMFADevice(ctx, user, d); err != nil {
			return trace.Wrap(err)
		}
	}
	if auth.Webauthn != nil {
		if err := s.UpsertWebauthnLocalAuth(ctx, user, auth.Webauthn); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetUserByOIDCIdentity returns a user by its specified OIDC Identity, returns first
// user specified with this identity
func (s *IdentityService) GetUserByOIDCIdentity(id types.ExternalIdentity) (types.User, error) {
	users, err := s.GetUsers(context.TODO(), false)
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

// GetUserBySAMLIdentity returns a user by its specified OIDC Identity, returns
// first user specified with this identity.
func (s *IdentityService) GetUserBySAMLIdentity(id types.ExternalIdentity) (types.User, error) {
	users, err := s.GetUsers(context.TODO(), false)
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
	users, err := s.GetUsers(context.TODO(), false)
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
	_, err := s.GetUser(ctx, user, false)
	if err != nil {
		return trace.Wrap(err)
	}

	// each user has multiple related entries in the backend,
	// so use DeleteRange to make sure we get them all
	startKey := backend.ExactKey(webPrefix, usersPrefix, user)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}

	// Delete notification objects associated with this user.
	var notifErrors []error
	// Delete all user-specific notifications for this user.
	if err := s.notificationsSvc.DeleteAllUserNotificationsForUser(ctx, user); err != nil {
		notifErrors = append(notifErrors, trace.Wrap(err, "failed to delete notifications for user %s", user))
	}
	// Delete all user notification states for this user.
	if err := s.notificationsSvc.DeleteAllUserNotificationStatesForUser(ctx, user); err != nil {
		notifErrors = append(notifErrors, trace.Wrap(err, "failed to delete notification states for user %s", user))
	}

	return trace.NewAggregate(notifErrors...)
}

func (s *IdentityService) upsertPasswordHash(username string, hash []byte) error {
	userPrototype, err := types.NewUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.CreateUser(context.TODO(), userPrototype)
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

// UpsertPassword upserts a new password. It also sets the user's
// `PasswordState` status flag accordingly. Returns an error if the user doesn't
// exist.
func (s *IdentityService) UpsertPassword(user string, password []byte) error {
	ctx := context.TODO()
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

	if err := s.upsertPasswordHash(user, hash); err != nil {
		return trace.Wrap(err)
	}

	_, err = s.UpdateAndSwapUser(
		ctx,
		user,
		false, /*withSecrets*/
		func(u types.User) (bool, error) {
			u.SetPasswordState(types.PasswordState_PASSWORD_STATE_SET)
			return true, nil
		})
	if err != nil {
		// Don't let the password state flag change fail the entire operation.
		s.logger.WarnContext(ctx, "Failed to set password state",
			"user", user,
			"error", err,
		)
	}

	return nil
}

// DeletePassword deletes user's password and sets the `PasswordState` status
// flag accordingly.
func (s *IdentityService) DeletePassword(ctx context.Context, user string) error {
	if user == "" {
		return trace.BadParameter("missing username")
	}

	delErr := s.Delete(ctx, backend.NewKey(webPrefix, usersPrefix, user, pwdPrefix))
	// Don't bail out just yet if the error is "not found"; the password state
	// flag may still be unspecified, and we want to make it UNSET.
	if delErr != nil && !trace.IsNotFound(delErr) {
		return trace.Wrap(delErr)
	}

	if _, err := s.UpdateAndSwapUser(
		ctx,
		user,
		false, /*withSecrets*/
		func(u types.User) (bool, error) {
			u.SetPasswordState(types.PasswordState_PASSWORD_STATE_UNSET)
			return true, nil
		},
	); err != nil {
		// Don't let the password state flag change fail the entire operation.
		s.logger.WarnContext(ctx, "Failed to set password state",
			"user", user,
			"error", err,
		)
	}

	// Now is the time to return the delete operation, if any.
	return trace.Wrap(delErr)
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
	userJSON, err := json.Marshal(&webauthnUser{
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
			s.logger.WarnContext(ctx, "Failed to undo WebauthnLocalAuth update", "error", err)
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
	user := &webauthnUser{}
	if err := json.Unmarshal(item.Value, user); err != nil {
		return "", trace.Wrap(err)
	}
	return user.TeleportUser, nil
}

// webauthnUser represents a WebAuthn user stored under [webauthnUserKey].
// Looked up during passwordless logins.
type webauthnUser struct {
	TeleportUser string `json:"teleport_user"`
}

func webauthnLocalAuthKey(user string) backend.Key {
	return backend.NewKey(webPrefix, usersPrefix, user, webauthnLocalAuthPrefix)
}

func webauthnUserKey(id []byte) backend.Key {
	key := base64.RawURLEncoding.EncodeToString(id)
	return backend.NewKey(webauthnPrefix, usersPrefix, key)
}

func (s *IdentityService) UpsertWebauthnSessionData(ctx context.Context, user, sessionID string, sd *wantypes.SessionData) error {
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

func (s *IdentityService) GetWebauthnSessionData(ctx context.Context, user, sessionID string) (*wantypes.SessionData, error) {
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
	sd := &wantypes.SessionData{}
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

func (s *IdentityService) UpsertGlobalWebauthnSessionData(ctx context.Context, scope, id string, sd *wantypes.SessionData) error {
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

func (s *IdentityService) GetGlobalWebauthnSessionData(ctx context.Context, scope, id string) (*wantypes.SessionData, error) {
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
	sd := &wantypes.SessionData{}
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
	if err := s.upsertMFADevice(ctx, user, d); err != nil {
		return trace.Wrap(err)
	}
	if err := s.upsertUserStatusMFADevice(ctx, user); err != nil {
		s.logger.WarnContext(ctx, "Unable to update user status after adding MFA device", "error", err)
	}
	return nil
}
func (s *IdentityService) upsertMFADevice(ctx context.Context, user string, d *types.MFADevice) error {
	if user == "" {
		return trace.BadParameter("missing parameter user")
	}
	if err := d.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if _, ok := d.Device.(*types.MFADevice_Sso); ok {
		return trace.BadParameter("cannot create SSO MFA device")
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

// upsertUserStatusMFADevice updates the user's MFA state based on the devices
// they have.
// It's called after adding or removing an MFA device and ensures the user's
// MFA state is up-to-date and reflects the weakest MFA device they have.
func (s *IdentityService) upsertUserStatusMFADevice(ctx context.Context, user string) error {
	devs, err := s.GetMFADevices(ctx, user, false)
	if err != nil {
		return trace.Wrap(err)
	}
	mfaState := GetWeakestMFADeviceKind(devs)

	_, err = s.UpdateAndSwapUser(
		ctx,
		user,
		false, /*withSecrets*/
		func(u types.User) (bool, error) {
			// If the user already has the weakest device, don't update.
			if u.GetWeakestDevice() == mfaState {
				return false, nil
			}
			u.SetWeakestDevice(mfaState)
			return true, nil
		})

	return trace.Wrap(err)
}

// buildAndSetWeakestMFADeviceKind builds the MFA state for a user and sets it on the user if successful.
func (s *IdentityService) buildAndSetWeakestMFADeviceKind(ctx context.Context, user types.User, localAuthSecrets *types.LocalAuthSecrets) {
	// upsertingMFA is the list of MFA devices that are being upserted when updating the user.
	var upsertingMFA []*types.MFADevice
	if localAuthSecrets != nil {
		upsertingMFA = localAuthSecrets.MFA
	}
	state, err := s.buildWeakestMFADeviceKind(ctx, user.GetName(), upsertingMFA...)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to determine weakest mfa device kind for user", "error", err)
		return
	}
	user.SetWeakestDevice(state)
}

func (s *IdentityService) buildWeakestMFADeviceKind(ctx context.Context, user string, upsertingMFA ...*types.MFADevice) (types.MFADeviceKind, error) {
	devs, err := s.GetMFADevices(ctx, user, false)
	if err != nil {
		return types.MFADeviceKind_MFA_DEVICE_KIND_UNSET, trace.Wrap(err)
	}
	return GetWeakestMFADeviceKind(append(devs, upsertingMFA...)), nil
}

// GetWeakestMFADeviceKind returns the weakest MFA state based on the devices the user
// has.
// When a user has no MFA device, it's set to `MFADeviceKind_MFA_DEVICE_KIND_UNSET`.
// When a user has at least one TOTP device, it's set to `MFADeviceKind_MFA_DEVICE_KIND_TOTP`.
// When a user ONLY has webauthn devices, it's set to `MFADeviceKind_MFA_DEVICE_KIND_WEBAUTHN`.
func GetWeakestMFADeviceKind(devs []*types.MFADevice) types.MFADeviceKind {
	mfaState := types.MFADeviceKind_MFA_DEVICE_KIND_UNSET
	for _, d := range devs {
		if (d.GetWebauthn() != nil || d.GetU2F() != nil) && mfaState == types.MFADeviceKind_MFA_DEVICE_KIND_UNSET {
			mfaState = types.MFADeviceKind_MFA_DEVICE_KIND_WEBAUTHN
		}
		if d.GetTotp() != nil {
			mfaState = types.MFADeviceKind_MFA_DEVICE_KIND_TOTP
			break
		}
	}
	return mfaState
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
	if trace.IsNotFound(err) {
		if _, err := s.getSSOMFADevice(ctx, user); err == nil {
			return trace.BadParameter("cannot delete ephemeral SSO MFA device")
		}
		return trace.Wrap(err)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.upsertUserStatusMFADevice(ctx, user); err != nil {
		s.logger.WarnContext(ctx, "Unable to update user status after deleting MFA device", "error", err)
	}
	return nil
}

func (s *IdentityService) GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	// get normal MFA devices and SSO mfa device concurrently, returning the first error we get.
	eg, egCtx := errgroup.WithContext(ctx)

	var devices []*types.MFADevice
	eg.Go(func() error {
		var err error
		devices, err = s.getMFADevices(egCtx, user, withSecrets)
		return trace.Wrap(err)
	})

	var ssoDev *types.MFADevice
	eg.Go(func() error {
		var err error
		ssoDev, err = s.getSSOMFADevice(egCtx, user)
		if trace.IsNotFound(err) {
			return nil // OK, SSO device may not exist.
		}
		return trace.Wrap(err)
	})

	if err := eg.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	if ssoDev != nil {
		devices = append(devices, ssoDev)
	}

	return devices, nil
}

// getMFADevices reads devices from storage. Devices from other sources, such as
// the ephemeral SSO devices, are not returned by it.
// See getSSOMFADevice and GetMFADevices (which returns all devices).
func (s *IdentityService) getMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error) {
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

// getSSOMFADevice returns the user's SSO MFA device. This device is ephemeral, meaning it
// does not actually appear in the backend under the user's mfa key. Instead it is fetched
// by checking related user and cluster configuration settings.
func (s *IdentityService) getSSOMFADevice(ctx context.Context, user string) (*types.MFADevice, error) {
	if user == "" {
		return nil, trace.BadParameter("missing parameter user")
	}

	u, err := s.GetUser(ctx, user, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cb := u.GetCreatedBy()
	if cb.Connector == nil {
		return nil, trace.NotFound("no SSO MFA device found; user was not created by an auth connector")
	}

	var mfaConnector interface {
		IsMFAEnabled() bool
		GetDisplay() string
	}

	const ssoMFADisabledErr = "no SSO MFA device found; user's auth connector does not have MFA enabled"
	switch cb.Connector.Type {
	case constants.SAML:
		mfaConnector, err = s.GetSAMLConnector(ctx, cb.Connector.ID, false /* withSecrets */)
	case constants.OIDC:
		mfaConnector, err = s.GetOIDCConnector(ctx, cb.Connector.ID, false /* withSecrets */)
	case constants.Github:
		// Github connectors do not support SSO MFA.
		return nil, trace.NotFound(ssoMFADisabledErr)
	default:
		return nil, trace.NotFound("user created by unknown auth connector type %v", cb.Connector.Type)
	}
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("user created by unknown %v auth connector %v", cb.Connector.Type, cb.Connector.ID)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !mfaConnector.IsMFAEnabled() {
		return nil, trace.NotFound(ssoMFADisabledErr)
	}

	return types.NewMFADevice(mfaConnector.GetDisplay(), cb.Connector.ID, cb.Time.UTC(), &types.MFADevice_Sso{
		Sso: &types.SSOMFADevice{
			ConnectorId:   cb.Connector.ID,
			ConnectorType: cb.Connector.Type,
			DisplayName:   mfaConnector.GetDisplay(),
		},
	})
}

// UpsertOIDCConnector upserts OIDC Connector
func (s *IdentityService) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	if err := connector.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := connector.GetRevision()
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: rev,
	}
	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
}

// CreateOIDCConnector creates a new OIDC connector.
func (s *IdentityService) CreateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	if err := connector.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
	}
	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
}

// UpdateOIDCConnector updates an existing OIDC connector.
func (s *IdentityService) UpdateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	if err := connector.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: connector.GetRevision(),
	}
	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
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
	conn, err := services.UnmarshalOIDCConnector(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
		conn, err := services.UnmarshalOIDCConnector(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			s.logger.ErrorContext(ctx, "Error unmarshaling OIDC Connector",
				"key", item.Key,
				"error", err,
			)
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
func (s *IdentityService) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	if err := services.ValidateSAMLConnector(connector, nil); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := connector.GetRevision()
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: rev,
	}
	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
}

// UpdateSAMLConnector updates an existing SAML connector
func (s *IdentityService) UpdateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	if err := services.ValidateSAMLConnector(connector, nil); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: connector.GetRevision(),
	}
	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
}

// CreateSAMLConnector creates a new SAML connector.
func (s *IdentityService) CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	if err := services.ValidateSAMLConnector(connector, nil); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
	}
	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
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
	conn, err := services.UnmarshalSAMLConnector(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
		conn, err := services.UnmarshalSAMLConnector(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			s.logger.ErrorContext(ctx, "Error unmarshaling SAML Connector",
				"key", item.Key,
				"error", err,
			)
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

func (s *IdentityService) UpsertSSOMFASessionData(ctx context.Context, sd *services.SSOMFASessionData) error {
	switch {
	case sd == nil:
		return trace.BadParameter("missing parameter sd")
	case sd.RequestID == "":
		return trace.BadParameter("missing parameter RequestID")
	case sd.ConnectorID == "":
		return trace.BadParameter("missing parameter ConnectorID")
	case sd.ConnectorType == "":
		return trace.BadParameter("missing parameter ConnectorType")
	case sd.Username == "":
		return trace.BadParameter("missing parameter Username")
	}

	value, err := json.Marshal(sd)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(ctx, backend.Item{
		Key:     ssoMFASessionDataKey(sd.RequestID),
		Value:   value,
		Expires: s.Clock().Now().UTC().Add(defaults.WebauthnChallengeTimeout),
	})
	return trace.Wrap(err)
}

func (s *IdentityService) GetSSOMFASessionData(ctx context.Context, sessionID string) (*services.SSOMFASessionData, error) {
	if sessionID == "" {
		return nil, trace.BadParameter("missing parameter sessionID")
	}

	item, err := s.Get(ctx, ssoMFASessionDataKey(sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sd := &services.SSOMFASessionData{}
	return sd, trace.Wrap(json.Unmarshal(item.Value, sd))
}

func (s *IdentityService) DeleteSSOMFASessionData(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return trace.BadParameter("missing parameter sessionID")
	}

	return trace.Wrap(s.Delete(ctx, ssoMFASessionDataKey(sessionID)))
}

func ssoMFASessionDataKey(sessionID string) backend.Key {
	return backend.NewKey(webPrefix, ssoMFASessionData, sessionID)
}

// UpsertGithubConnector creates or updates a Github connector
func (s *IdentityService) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error) {
	if err := services.CheckAndSetDefaults(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := connector.GetRevision()
	value, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: rev,
	}
	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
}

// UpdateGithubConnector updates an existing Github connector.
func (s *IdentityService) UpdateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error) {
	if err := services.CheckAndSetDefaults(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: connector.GetRevision(),
	}
	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
}

// CreateGithubConnector creates a new Github connector.
func (s *IdentityService) CreateGithubConnector(ctx context.Context, connector types.GithubConnector) (types.GithubConnector, error) {
	if err := services.CheckAndSetDefaults(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
	}
	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector.SetRevision(lease.Revision)
	return connector, nil
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
		connector, err := services.UnmarshalGithubConnector(item.Value, services.WithRevision(item.Revision))
		if err != nil {
			s.logger.ErrorContext(ctx, "Error unmarshaling GitHub Connector",
				"key", item.Key,
				"error", err,
			)
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
	connector, err := services.UnmarshalGithubConnector(item.Value, services.WithRevision(item.Revision))
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
	webPrefix                 = "web"
	usersPrefix               = "users"
	sessionsPrefix            = "sessions"
	attemptsPrefix            = "attempts"
	pwdPrefix                 = "pwd"
	connectorsPrefix          = "connectors"
	oidcPrefix                = "oidc"
	samlPrefix                = "saml"
	githubPrefix              = "github"
	requestsPrefix            = "requests"
	requestsTracePrefix       = "requestsTrace"
	usedTOTPPrefix            = "used_totp"
	usedTOTPTTL               = 30 * time.Second
	mfaDevicePrefix           = "mfa"
	webauthnPrefix            = "webauthn"
	webauthnGlobalSessionData = "sessionData"
	webauthnLocalAuthPrefix   = "webauthnlocalauth"
	webauthnSessionData       = "webauthnsessiondata"
	ssoMFASessionData         = "ssomfasessiondata"
	recoveryCodesPrefix       = "recoverycodes"
	attestationsPrefix        = "key_attestations"
	userPreferencesPrefix     = "user_preferences"
)
