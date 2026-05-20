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

package usersv1_test

import (
	"context"
	"encoding/base32"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/users/usersv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	libutils "github.com/gravitational/teleport/lib/utils"
)

type fakeAuthorizer struct {
	authorize bool

	authzContext *authz.Context
}

// Authorize implements authz.Authorizer
func (a fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	identity, err := authz.UserFromContext(ctx)
	if err == nil {
		user, err := types.NewUser("alice")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &authz.Context{
			User: user,
			Checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
					},
				},
			},
			Identity:             identity,
			AdminActionAuthState: authz.AdminActionAuthNotRequired,
		}, nil
	}

	if a.authzContext != nil {
		return a.authzContext, nil
	}

	user, err := types.NewUser("alice")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authz.Context{
		User: user,
		Checker: &fakeChecker{
			rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
				},
			},
		},
		Identity: &authz.LocalUser{
			Username: "alice",
			Identity: tlsca.Identity{
				Groups:   []string{"dev"},
				Username: "alice",
			},
		},
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	rules  []types.Rule
	roles  []string
	checks []check
}

type check struct {
	kind, verb string
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, kind string, verb string) error {
	c := check{kind, verb}
	f.checks = append(f.checks, c)

	for _, r := range f.rules {
		if r.HasResource(kind) && r.HasVerb(verb) {
			return nil
		}
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", kind, verb)
}

// HasRole checks if the checker includes the role
func (f *fakeChecker) HasRole(target string) bool {
	return slices.Contains(f.roles, target)
}

type serviceOpt = func(config *usersv1.ServiceConfig)

func withAuthorizer(authz authz.Authorizer) serviceOpt {
	return func(config *usersv1.ServiceConfig) {
		config.Authorizer = authz
	}
}

func withEmitter(emitter apievents.Emitter) serviceOpt {
	return func(config *usersv1.ServiceConfig) {
		config.Emitter = emitter
	}
}

type testEnvBackend interface {
	usersv1.Backend
	services.Identity
}

type env struct {
	*usersv1.Service
	emitter *eventstest.ChannelEmitter
	backend testEnvBackend
	clock   *clockwork.FakeClock
}

func newTestEnv(opts ...serviceOpt) (*env, error) {
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock: clock,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating memory backend")
	}

	identityService, err := local.NewTestIdentityService(bk)
	if err != nil {
		return nil, trace.Wrap(err, "initializing identity service")
	}

	service := struct {
		services.Identity
		services.Access
	}{
		Identity: identityService,
		Access:   local.NewAccessService(bk),
	}

	emitter := eventstest.NewChannelEmitter(10)

	cfg := usersv1.ServiceConfig{
		Authorizer: fakeAuthorizer{authorize: true},
		Cache:      service,
		Backend:    service,
		Auth:       &fakeAuth{clock: clock},
		Emitter:    emitter,
		Reporter:   usagereporter.DiscardUsageReporter{},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	svc, err := usersv1.NewService(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating users service")
	}

	return &env{
		Service: svc,
		emitter: emitter,
		backend: service,
		clock:   clock,
	}, nil
}

type fakeAuth struct {
	clock clockwork.Clock
}

func (a *fakeAuth) NewUserToken(
	ctx context.Context, req authclient.CreateUserTokenRequest,
) (types.UserToken, error) {
	tokenID, err := libutils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := types.NewUserToken(tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token.SetSubKind(req.Type)
	token.SetExpiry(a.clock.Now().UTC().Add(req.TTL))
	token.SetUser(req.Name)
	token.SetCreated(a.clock.Now().UTC())
	token.SetURL(fmt.Sprintf("https://example.com:3080/web/reset/%v", tokenID))

	return token, nil
}
func (a *fakeAuth) DeleteUserTokens(ctx context.Context, username string) error {
	return nil
}

func TestCreateUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := t.Context()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	// Create a new user.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	// Validate that the user now exists.
	resp, err := env.GetUser(ctx, &userspb.GetUserRequest{Name: created.User.GetName()})
	require.NoError(t, err, "failed getting created user")
	require.Empty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Attempt to create a duplicate user
	created2, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	assert.Error(t, err, "duplicate user was created successfully")
	assert.Nil(t, created2, "received unexpected user ")
	require.True(t, trace.IsAlreadyExists(err), "creating duplicate user allowed")

	event := <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")
	createEvent, ok := event.(*apievents.UserCreate)
	require.True(t, ok, "expected a UserCreate event got %T", event)
	assert.Equal(t, "alice", createEvent.UserMetadata.User)

	user, err := types.NewUser("alpaca")
	require.NoError(t, err, "creating user alpaca")
	user.SetRoles([]string{uuid.NewString()})
	_, err = env.CreateUser(ctx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
	assert.True(t, trace.IsNotFound(err), "expected a not found error, got %T", err)
	require.Error(t, err, "user allowed to be created with a role that does not exist")
	createEvent, ok = event.(*apievents.UserCreate)
	require.True(t, ok, "expected a UserCreate event got %T", event)
	assert.Equal(t, "alice", createEvent.UserMetadata.User)
}

func TestCreateUserMaxLength(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	user, err := types.NewUser(strings.Repeat("A", 1001))
	require.NoError(t, err)
	user.AddRole("access")
	_, err = env.CreateUser(t.Context(), &userspb.CreateUserRequest{User: user.(*types.UserV2)})
	require.Error(t, err, "creating a user with a username too long should fail")
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := t.Context()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	// Create the user which will be deleted.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	event := <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")

	// Delete the user.
	_, err = env.DeleteUser(ctx, &userspb.DeleteUserRequest{Name: created.User.GetName()})
	require.NoError(t, err)

	event = <-env.emitter.C()
	assert.Equal(t, events.UserDeleteEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserDeleteCode, event.GetCode(), "unexpected event code")

	// Attempt to delete the user again, this time deletion should fail because
	// the user no longer exists.
	_, err = env.DeleteUser(ctx, &userspb.DeleteUserRequest{Name: created.User.GetName()})
	assert.Error(t, err, "deleting nonexistent user succeeded")
	require.True(t, trace.IsNotFound(err), "expected a not found error deleting nonexistent user got %T", err)
}

func TestGetUser(t *testing.T) {
	t.Parallel()

	// create an admin authz context to test listing users with secrets
	authzContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	}, &types.SessionRecordingConfigV2{})
	require.NoError(t, err, "creating authorization context")

	env, err := newTestEnv(withAuthorizer(fakeAuthorizer{authzContext: authzContext}))
	require.NoError(t, err, "creating test service")

	ctx := t.Context()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")
	require.NoError(t, generateUserSecrets(llama), "generating user secrets")

	// Validate that the user does not exist.
	resp, err := env.GetUser(ctx, &userspb.GetUserRequest{Name: llama.GetName()})
	assert.Error(t, err, "expected retrieving nonexistent user to fail")
	assert.Nil(t, resp, "non-nil response returned from error")
	assert.True(t, trace.IsNotFound(err), "expected not found error got %T", err)

	// Create a new user.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	// Validate that the user now exists and that querying by name takes precedence over
	// retrieving the current user.
	resp, err = env.GetUser(ctx, &userspb.GetUserRequest{Name: created.User.GetName(), CurrentUser: true})
	assert.NoError(t, err, "failed getting created user")
	assert.Empty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision"), cmpopts.IgnoreFields(types.UserSpecV2{}, "LocalAuth")))
	assert.Nil(t, resp.User.GetLocalAuth(), "user secrets were provided when not requested")

	// Validate that getting the current user returns "alice" and not "llama".
	resp, err = env.GetUser(authz.ContextWithUser(ctx, &authz.LocalUser{
		Username: "alice",
		Identity: tlsca.Identity{
			Groups: []string{"dev"},
		},
	}), &userspb.GetUserRequest{CurrentUser: true})
	assert.NoError(t, err, "failed getting created user")
	assert.NotEmpty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	assert.Equal(t, "alice", resp.User.GetName(), "expected current user to return alice")
	assert.Nil(t, resp.User.GetLocalAuth(), "secrets returned with current user")

	// Validate that requesting a users secrets returns them.
	resp, err = env.GetUser(ctx, &userspb.GetUserRequest{Name: created.User.GetName(), WithSecrets: true})
	assert.NoError(t, err, "failed getting created user")
	assert.Empty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	assert.NotNil(t, resp.User.GetLocalAuth(), "user secrets were not provided requested")
	assert.Empty(t, cmp.Diff(llama.GetLocalAuth(), resp.User.GetLocalAuth()), "user secrets do not match")

	// Validate that getting the current user never returns secrets
	resp, err = env.GetUser(authz.ContextWithUser(ctx, &authz.LocalUser{
		Username: "alice",
		Identity: tlsca.Identity{
			Groups: []string{"dev"},
		},
	}), &userspb.GetUserRequest{CurrentUser: true, WithSecrets: true})
	assert.NoError(t, err, "failed getting created user")
	assert.NotEmpty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	assert.Equal(t, "alice", resp.User.GetName(), "expected current user to return alice")
	assert.Nil(t, resp.User.GetLocalAuth(), "secrets returned with current user")
}

func TestUpdateUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := t.Context()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	// Attempt to update a nonexistent user.
	updated, err := env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: llama.(*types.UserV2)})
	assert.Error(t, err, "duplicate user was created successfully")
	assert.Nil(t, updated, "received unexpected user")
	require.True(t, trace.IsCompareFailed(err), "updated nonexistent user")

	// Create a new user.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	event := <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")
	createEvent, ok := event.(*apievents.UserCreate)
	require.True(t, ok, "expected a UserCreate event got %T", event)
	assert.Equal(t, "alice", createEvent.UserMetadata.User)

	// Attempt to update the user again.
	created.User.SetLogins([]string{"alpaca"})
	updated, err = env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: created.User})
	require.NoError(t, err, "failed updating user")
	require.Empty(t, cmp.Diff(created.User, updated.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	require.Equal(t, []string{"alpaca"}, updated.User.GetLogins(), "logins were not updated")

	event = <-env.emitter.C()
	assert.Equal(t, events.UserUpdatedEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserUpdateCode, event.GetCode(), "unexpected event code")
	createEvent, ok = event.(*apievents.UserCreate)
	require.True(t, ok, "expected a UserCreate event got %T", event)
	assert.Equal(t, "alice", createEvent.UserMetadata.User)

	// Attempt to update an existing user and set invalid roles
	updated.User.AddRole("does-not-exist")
	_, err = env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: updated.User})
	assert.True(t, trace.IsNotFound(err), "expected a not found error, got %T", err)
	require.Error(t, err, "user allowed to be updated with a role that does not exist")
}

func TestUpsertUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := t.Context()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	// Create a user via upsert.
	upserted, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "failed upserting user")

	// Validate that the user was created.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	assert.Error(t, err, "duplicate user was created successfully")
	assert.Nil(t, created, "received unexpected user ")
	require.True(t, trace.IsAlreadyExists(err), "creating duplicate user allowed")

	event := <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")
	createEvent, ok := event.(*apievents.UserCreate)
	require.True(t, ok, "expected a UserCreate event got %T", event)
	assert.Equal(t, "alice", createEvent.UserMetadata.User)

	// Attempt to update the user again.
	upserted.User.SetLogins([]string{"alpaca"})
	updated, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: upserted.User})
	require.NoError(t, err, "failed upserting user")
	require.Empty(t, cmp.Diff(upserted.User, updated.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	require.Equal(t, []string{"alpaca"}, updated.User.GetLogins(), "logins were not updated")

	event = <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")
	createEvent, ok = event.(*apievents.UserCreate)
	require.True(t, ok, "expected a UserCreate event got %T", event)
	assert.Equal(t, "alice", createEvent.UserMetadata.User)

	// Attempt to upsert a  user and set invalid roles
	updated.User.AddRole("does-not-exist")
	_, err = env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: updated.User})
	assert.True(t, trace.IsNotFound(err), "expected a not found error, got %T", err)
	require.Error(t, err, "user allowed to be upserted with a role that does not exist")
}

func TestListUsers(t *testing.T) {
	t.Parallel()

	// create an admin authz context to test listing users with secrets
	authzContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	}, &types.SessionRecordingConfigV2{})
	require.NoError(t, err, "creating authorization context")

	env, err := newTestEnv(
		withAuthorizer(fakeAuthorizer{authzContext: authzContext}),
		withEmitter(&events.DiscardEmitter{}),
	)
	require.NoError(t, err, "creating test service")

	ctx := t.Context()

	// Create a role to assign to users for search testing.
	accessSvc := env.backend.(interface {
		UpsertRole(context.Context, types.Role) (types.Role, error)
	})
	role, err := types.NewRole("test-role", types.RoleSpecV6{})
	require.NoError(t, err, "creating role")
	_, err = accessSvc.UpsertRole(ctx, role)
	require.NoError(t, err, "upserting role")

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")
	require.NoError(t, generateUserSecrets(llama), "generating user secrets")
	llama.SetRoles([]string{"test-role"})
	llama.SetTraits(map[string][]string{"logins": {"test-login"}})

	// Validate that the user does not exist.
	resp, err := env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 10})
	assert.NoError(t, err, "expected list to return empty response when no users exist")
	assert.Empty(t, resp.Users, "expected no users to be returned got %d", len(resp.Users))
	assert.Empty(t, resp.NextPageToken, "expected next page token to be empty")

	// Create a new user.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	// Validate that the user now exists.
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 10})
	assert.NoError(t, err, "failed listing created user")
	assert.Len(t, resp.Users, 1, "expected one user to be returned got %d", len(resp.Users))
	assert.Empty(t, resp.NextPageToken, "expected next page token to be empty")
	assert.Empty(t, cmp.Diff(created.User, resp.Users[0], cmpopts.IgnoreFields(types.Metadata{}, "Revision"), cmpopts.IgnoreFields(types.UserSpecV2{}, "LocalAuth")))
	assert.Nil(t, resp.Users[0].GetLocalAuth(), "user secrets were provided when not requested")

	// Validate that requesting a users secrets returns them.
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 10, WithSecrets: true})
	assert.NoError(t, err, "failed listing created user")
	assert.Len(t, resp.Users, 1, "expected one user to be returned got %d", len(resp.Users))
	assert.Empty(t, resp.NextPageToken, "expected next page token to be empty")
	assert.Empty(t, cmp.Diff(created.User, resp.Users[0], cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	assert.Empty(t, cmp.Diff(llama.GetLocalAuth(), resp.Users[0].GetLocalAuth()), "user secrets do not match")

	// Validate that searching by role returns matching users.
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{
		PageSize: 10,
		Filter:   &types.UserFilter{SearchKeywords: []string{"test-role"}},
	})
	require.NoError(t, err, "listing users with role filter")
	require.Len(t, resp.Users, 1, "expected one user with test-role")
	assert.Equal(t, "llama", resp.Users[0].GetName(), "expected llama to match role search")

	// Validate that searching by trait returns matching users (SearchKeywords).
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{
		PageSize: 10,
		Filter:   &types.UserFilter{SearchKeywords: []string{"test-login"}},
	})
	require.NoError(t, err, "listing users with trait filter (SearchKeywords)")
	require.Len(t, resp.Users, 1, "expected one user with trait logins: test-login")
	assert.Equal(t, "llama", resp.Users[0].GetName(), "expected llama to match trait search")

	// Validate that searching by trait returns matching users (Traits).
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{
		PageSize: 10,
		Filter:   &types.UserFilter{Traits: map[string][]string{"logins": {"test-login"}}},
	})
	require.NoError(t, err, "listing users with trait filter (Traits)")
	require.Len(t, resp.Users, 1, "expected one user with trait logins: test-login")
	assert.Equal(t, "llama", resp.Users[0].GetName(), "expected llama to match trait search")

	// Create addition users to test pagination
	createdUsers := []*types.UserV2{llama.(*types.UserV2)}
	for i := range 22 {
		user, err := types.NewUser(fmt.Sprintf("user_%d", i))
		require.NoError(t, err, "creating new user %d", i)
		require.NoError(t, generateUserSecrets(user), "generating user secrets")

		// Create a new user.
		created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
		require.NoError(t, err, "creating user %d", i)

		createdUsers = append(createdUsers, created.User)
	}

	// List all users across multiple pages without secrets.
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 3})
	require.NoError(t, err, "unexpected error listing users")

	listedUsers := resp.Users
	for next := resp.NextPageToken; next != ""; {
		resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 3, PageToken: next})
		require.NoError(t, err, "unexpected error listing users")
		listedUsers = append(listedUsers, resp.Users...)
		next = resp.NextPageToken
	}

	assert.Len(t, createdUsers, len(listedUsers), "expected to eventually retrieve all users from listing")
	assert.Empty(t, cmp.Diff(createdUsers, listedUsers,
		cmpopts.SortSlices(func(a, b *types.UserV2) bool { return a.GetName() < b.GetName() }),
		cmpopts.IgnoreFields(types.UserSpecV2{}, "LocalAuth"),
	))

	// List all users across multiple pages with secrets.
	resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 3, WithSecrets: true})
	require.NoError(t, err, "unexpected error listing users")

	listedUsersWithSecrets := resp.Users
	for next := resp.NextPageToken; next != ""; {
		resp, err = env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 3, PageToken: next, WithSecrets: true})
		require.NoError(t, err, "unexpected error listing users")
		listedUsersWithSecrets = append(listedUsersWithSecrets, resp.Users...)
		next = resp.NextPageToken
	}

	assert.Len(t, createdUsers, len(listedUsersWithSecrets), "expected to eventually retrieve all users from listing")
	assert.Empty(t, cmp.Diff(createdUsers, listedUsersWithSecrets,
		cmpopts.SortSlices(func(a, b *types.UserV2) bool { return a.GetName() < b.GetName() }),
	))
}

func generateUserSecrets(u types.User) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("insecure"), bcrypt.MinCost)
	if err != nil {
		return trace.Wrap(err)
	}

	dev, err := services.NewTOTPDevice("otp", base32.StdEncoding.EncodeToString([]byte("abc123")), time.Now())
	if err != nil {
		return trace.Wrap(err)
	}

	u.SetLocalAuth(&types.LocalAuthSecrets{
		PasswordHash: hash,
		MFA:          []*types.MFADevice{dev},
	})
	return nil
}

func TestRBAC(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	tests := []struct {
		desc         string
		f            func(t *testing.T, env *env)
		checker      *fakeChecker
		expectChecks []check
	}{
		{
			desc: "get no access",
			f: func(t *testing.T, env *env) {
				_, err := env.GetUser(ctx, &userspb.GetUserRequest{Name: "alice"})
				assert.Error(t, err, "expected RBAC to prevent getting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
				{kind: types.KindUser, verb: types.VerbRead},
			},
		},
		{
			desc: "get current users when no access",
			f: func(t *testing.T, env *env) {
				user, err := env.GetUser(ctx, &userspb.GetUserRequest{CurrentUser: true})
				assert.NoError(t, err, "expected RBAC to allow getting the current user")
				assert.Empty(t, cmp.Diff(llama, user.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
		},
		{
			desc: "get with secrets no access",
			f: func(t *testing.T, env *env) {
				_, err := env.GetUser(ctx, &userspb.GetUserRequest{Name: "alice", WithSecrets: true})
				assert.Error(t, err, "expected RBAC to prevent getting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbList},
					},
				},
			},
			expectChecks: []check{},
		},
		{
			desc: "get",
			f: func(t *testing.T, env *env) {
				resp, err := env.GetUser(ctx, &userspb.GetUserRequest{Name: "llama"})
				assert.NoError(t, err, "expected RBAC to allow getting user")
				assert.Empty(t, cmp.Diff(llama, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbRead},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
				{kind: types.KindUser, verb: types.VerbRead},
			},
		},
		{
			desc: "create no access",
			f: func(t *testing.T, env *env) {
				_, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
				assert.Error(t, err, "expected RBAC to prevent creating user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
			},
		},
		{
			desc: "create",
			f: func(t *testing.T, env *env) {
				u := utils.CloneProtoMsg(llama.(*types.UserV2))
				u.SetName("alpaca")
				created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: u})
				assert.NoError(t, err, "expected RBAC to allow creating user")
				assert.Empty(t, cmp.Diff(u, created.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
			},
		},
		{
			desc: "update no access",
			f: func(t *testing.T, env *env) {
				_, err := env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: llama.(*types.UserV2)})
				assert.Error(t, err, "expected RBAC to prevent updating user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "update",
			f: func(t *testing.T, env *env) {
				u := utils.CloneProtoMsg(llama.(*types.UserV2))
				u.SetLogins([]string{"alpaca"})
				updated, err := env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: u})
				assert.NoError(t, err, "expected RBAC to allow updating user")
				assert.Empty(t, cmp.Diff(u, updated.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbUpdate},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "upsert no access",
			f: func(t *testing.T, env *env) {
				_, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
				assert.Error(t, err, "expected RBAC to prevent upserting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "upsert without create",
			f: func(t *testing.T, env *env) {
				_, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
				assert.Error(t, err, "expected RBAC to prevent upserting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbUpdate},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "upsert without update",
			f: func(t *testing.T, env *env) {
				_, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
				assert.Error(t, err, "expected RBAC to prevent upserting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "upsert",
			f: func(t *testing.T, env *env) {
				upserted, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
				assert.NoError(t, err, "expected RBAC to allow updating user")
				assert.Empty(t, cmp.Diff(llama, upserted.User, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate, types.VerbUpdate},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbCreate},
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "delete no access",
			f: func(t *testing.T, env *env) {
				_, err := env.DeleteUser(ctx, &userspb.DeleteUserRequest{Name: llama.GetName()})
				assert.Error(t, err, "expected RBAC to prevent deleting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbDelete},
			},
		},
		{
			desc: "delete",
			f: func(t *testing.T, env *env) {
				_, err := env.DeleteUser(ctx, &userspb.DeleteUserRequest{Name: llama.GetName()})
				assert.NoError(t, err, "expected RBAC to allow deleting user")
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbDelete},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbDelete},
			},
		},
		{
			desc: "list no access",
			f: func(t *testing.T, env *env) {
				_, err := env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 1})
				assert.Error(t, err, "expected RBAC to prevent listing users")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error got %T", err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbList},
				{kind: types.KindUser, verb: types.VerbRead},
			},
		},
		{
			desc: "list",
			f: func(t *testing.T, env *env) {
				resp, err := env.ListUsers(ctx, &userspb.ListUsersRequest{PageSize: 1})
				assert.NoError(t, err, "expected RBAC to prevent deleting user")
				require.Len(t, resp.Users, 1, "expected list to return a single user got %d", len(resp.Users))
				assert.Empty(t, cmp.Diff(llama, resp.Users[0], cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbRead, types.VerbList},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbList},
				{kind: types.KindUser, verb: types.VerbRead},
			},
		},
		{
			desc: "reset no access",
			f: func(t *testing.T, env *env) {
				_, err := env.ResetUser(ctx, &userspb.ResetUserRequest{Name: llama.GetName()})
				assert.Error(t, err, "eexpected RBAC to prevent resetting user")
				assert.True(t, trace.IsAccessDenied(err), "expected access denied, got %T %v", err, err)

				evts := getAllEvents(env.emitter.C())
				require.GreaterOrEqual(t, len(evts), 1)

				e, ok := evts[len(evts)-1].(*apievents.UserReset)
				require.True(t, ok)
				assert.Equal(t, events.UserResetFailureEvent, e.GetType())
				assert.Equal(t, events.UserResetFailureCode, e.Code)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbList},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
		{
			desc: "reset",
			f: func(t *testing.T, env *env) {
				_, err := env.ResetUser(ctx, &userspb.ResetUserRequest{Name: llama.GetName()})
				assert.NoError(t, err)
			},
			checker: &fakeChecker{
				rules: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbUpdate},
					},
				},
			},
			expectChecks: []check{
				{kind: types.KindUser, verb: types.VerbUpdate},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			env, err := newTestEnv(withAuthorizer(&fakeAuthorizer{authzContext: &authz.Context{
				User:    llama,
				Checker: test.checker,
				Identity: authz.LocalUser{
					Username: "alice",
					Identity: tlsca.Identity{
						Groups: []string{"dev"},
					},
				},
				AdminActionAuthState: authz.AdminActionAuthNotRequired,
			}}))
			require.NoError(t, err, "creating test service")

			// Create the user directly on the backend to bypass RBAC enforced by the test cases.
			_, err = env.backend.CreateUser(ctx, llama.(*types.UserV2))
			require.NoError(t, err, "creating test user")

			// Validate RBAC is enforced.
			test.f(t, env)
			require.ElementsMatch(t, test.expectChecks, test.checker.checks)
		})
	}
}

func TestResetUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	assertUserResetEvent := func(t *testing.T, e apievents.AuditEvent, username string) {
		ure, ok := e.(*apievents.UserReset)
		require.True(t, ok)
		assert.Equal(t, events.UserResetEvent, ure.GetType())
		assert.Equal(t, username, ure.Name)
		assert.Equal(t, events.UserResetCode, ure.Code)
		assert.Equal(t, teleport.UserSystem, ure.User)
	}

	testCases := []struct {
		name              string
		userCreatedBy     types.CreatedBy
		setupUser         func(t *testing.T, env *env, user types.User)
		numMFAsAfterSetup int
		numMFAsAfterReset int
		userType          types.UserType
		expectToken       bool
		assert            func(t *testing.T, env *env, user types.User, res *userspb.ResetUserResponse)
	}{
		{
			name: "local user",
			setupUser: func(t *testing.T, env *env, user types.User) {
				err := env.backend.UpsertPassword(user.GetName(), []byte("dummypasswordhash"))
				require.NoError(t, err)

				// Double-check the opposite condition to the assertion that will check
				// if the password is later removed.
				_, err = env.backend.GetPasswordHash(user.GetName())
				require.NoError(t, err)
			},
			numMFAsAfterSetup: 2,
			userType:          types.UserTypeLocal,
			expectToken:       true,
			assert: func(t *testing.T, env *env, user types.User, res *userspb.ResetUserResponse) {
				assert.NotNil(t, res.PasswordResetToken)
				assert.Equal(t, authclient.UserTokenTypeResetPassword, res.PasswordResetToken.SubKind)
				assert.Equal(t, user.GetName(), res.PasswordResetToken.GetUser(), user.GetName())

				evts := getAllEvents(env.emitter.C())
				require.GreaterOrEqual(t, len(evts), 2)
				evts = evts[len(evts)-2:]

				e, ok := evts[0].(*apievents.UserTokenCreate)
				require.True(t, ok)
				assert.Equal(t, events.ResetPasswordTokenCreateEvent, e.GetType())
				assert.Equal(t, user.GetName(), e.Name)
				assert.Equal(t, events.ResetPasswordTokenCreateCode, e.Code)
				assert.Equal(t, teleport.UserSystem, e.User)

				assertUserResetEvent(t, evts[1], user.GetName())
			},
		},
		{
			name: "SSO user, active",
			userCreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Connector: &types.ConnectorRef{
					Type: "dummy",
				},
			},
			setupUser:         func(t *testing.T, env *env, user types.User) {},
			numMFAsAfterSetup: 2,
			userType:          types.UserTypeSSO,
			assert: func(t *testing.T, env *env, user types.User, res *userspb.ResetUserResponse) {
				assert.Nil(t, res.PasswordResetToken)

				evts := getAllEvents(env.emitter.C())
				require.GreaterOrEqual(t, len(evts), 1)
				assertUserResetEvent(t, evts[len(evts)-1], user.GetName())
			},
		},
		{
			name: "SSO user, active, with SSO MFA",
			userCreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Connector: &types.ConnectorRef{
					Type: constants.SAML,
					ID:   "some-connector",
				},
			},
			setupUser: func(t *testing.T, env *env, user types.User) {
				u := user.(*types.UserV2)
				cb := u.GetCreatedBy()
				cb.Time = env.clock.Now().UTC()
				u.SetCreatedBy(cb)

				_, err := env.backend.UpdateUser(t.Context(), u)
				require.NoError(t, err)

				conn, err := types.NewSAMLConnector(
					"some-connector",
					types.SAMLConnectorSpecV2{
						AssertionConsumerService: "http://localhost:65535/acs", // not called
						Issuer:                   "test",
						SSO:                      "https://localhost:65535/sso", // not called
						AttributesToRoles: []types.AttributeMapping{
							{Name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", Value: "admin", Roles: []string{"access"}},
						},
						MFASettings: &types.SAMLConnectorMFASettings{
							Enabled: true,
							Issuer:  "test",
							Sso:     "http://localhost:65535/sso",
						},
					})
				require.NoError(t, err)

				_, err = env.backend.CreateSAMLConnector(ctx, conn)
				require.NoError(t, err)

				// Sanity check
				devs, err := env.backend.GetMFADevices(ctx, user.GetName(), false /* withSecrets */)
				require.NoError(t, err)
				require.Len(t, devs, 1)
			},
			numMFAsAfterSetup: 3,
			numMFAsAfterReset: 1,
			userType:          types.UserTypeSSO,
			assert: func(t *testing.T, env *env, user types.User, res *userspb.ResetUserResponse) {
				assert.Nil(t, res.PasswordResetToken)

				evts := getAllEvents(env.emitter.C())
				require.GreaterOrEqual(t, len(evts), 1)
				assertUserResetEvent(t, evts[len(evts)-1], user.GetName())
			},
		},
		{
			name: "SSO user, expired",
			userCreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Connector: &types.ConnectorRef{
					Type: "dummy",
				},
			},
			setupUser: func(t *testing.T, env *env, user types.User) {
				expiration := time.Hour
				now := env.clock.Now()
				user.SetExpiry(now.Add(expiration))
				_, err := env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: user.(*types.UserV2)})
				require.NoError(t, err)

				env.clock.Advance(user.Expiry().Sub(env.clock.Now()))
				_, err = env.GetUser(ctx, &userspb.GetUserRequest{Name: user.GetName()})
				require.True(t, trace.IsNotFound(err), "expected NotFound, got %T", err)
			},
			numMFAsAfterSetup: 2,
			userType:          types.UserTypeSSO,
			assert: func(t *testing.T, env *env, user types.User, res *userspb.ResetUserResponse) {
				assert.Nil(t, res.PasswordResetToken)

				evts := getAllEvents(env.emitter.C())
				require.GreaterOrEqual(t, len(evts), 1)
				assertUserResetEvent(t, evts[len(evts)-1], user.GetName())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := newTestEnv()
			require.NoError(t, err, "creating test service")

			user, err := types.NewUser("capybara")
			require.NoError(t, err)
			user.SetCreatedBy(tc.userCreatedBy)

			cuResp, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
			require.NoError(t, err)
			// The rules for GetUserType are non-obvious, so make sure that what we
			// created is really the kind of user we want to have.
			assert.Equal(t, tc.userType, cuResp.User.GetUserType())

			tc.setupUser(t, env, cuResp.User)

			// Add two MFA devices.
			devs := []*types.MFADevice{}
			devId := uuid.NewString()
			dev, err := types.NewMFADevice(
				"webauthn-key", devId, env.clock.Now(),
				&types.MFADevice_Webauthn{
					Webauthn: &types.WebauthnDevice{
						CredentialId:             []byte("cred-" + devId),
						PublicKeyCbor:            []byte("cbor-" + devId),
						AttestationType:          "none",
						Aaguid:                   []byte("aaguid"),
						SignatureCounter:         10,
						AttestationObject:        []byte("att-obj"),
						ResidentKey:              true,
						CredentialRpId:           "example.com",
						CredentialBackupEligible: &gogotypes.BoolValue{Value: true},
						CredentialBackedUp:       &gogotypes.BoolValue{Value: false},
					},
				})
			require.NoError(t, err)
			devs = append(devs, dev)

			devId = uuid.NewString()
			dev, err = types.NewMFADevice(
				"auth-app", devId, env.clock.Now(),
				&types.MFADevice_Totp{
					Totp: &types.TOTPDevice{Key: "totp-key-" + devId},
				},
			)
			require.NoError(t, err)
			devs = append(devs, dev)

			for _, d := range devs {
				err = env.backend.UpsertMFADevice(ctx, user.GetName(), d)
				require.NoError(t, err, "unable to upsert device %q", d.GetName())
			}

			// Double-check the opposite condition to the assertion that will check
			// if the MFAs are later removed.
			gotDevs, err := env.backend.GetMFADevices(ctx, user.GetName(), false)
			require.NoError(t, err)
			require.Len(t, gotDevs, tc.numMFAsAfterSetup)

			ruResp, err := env.ResetUser(ctx, &userspb.ResetUserRequest{
				Name: user.GetName(),
				Type: authclient.UserTokenTypeResetPassword,
			})
			require.NoError(t, err)

			tc.assert(t, env, user, ruResp)

			// verify that user has no MFA devices
			gotDevs, err = env.backend.GetMFADevices(ctx, user.GetName(), false)
			require.NoError(t, err)
			assert.Len(t, gotDevs, tc.numMFAsAfterReset)

			// verify that password was reset
			_, err = env.backend.GetPasswordHash(user.GetName())
			require.True(t, trace.IsNotFound(err), "expected trace.NotFound, got %T %v", err, err)
		})
	}
}

func TestResetUser_NotFound(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	_, err = env.ResetUser(ctx, &userspb.ResetUserRequest{
		Name: "foobar",
		Type: authclient.UserTokenTypeResetPassword,
	})
	assert.True(t, trace.IsNotFound(err), "expected trace.NotFound, got %T %v", err, err)
}

func TestResetUser_Bot(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	user, err := types.NewUser("capybara")
	require.NoError(t, err)

	metadata := user.GetMetadata()
	if metadata.Labels == nil {
		metadata.Labels = map[string]string{}
	}
	metadata.Labels[types.BotLabel] = "R2D2"
	user.SetMetadata(metadata)

	resp, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
	require.NoError(t, err)

	// Sanity check.
	assert.True(t, resp.User.IsBot())

	_, err = env.ResetUser(ctx, &userspb.ResetUserRequest{
		Name: "capybara",
		Type: authclient.UserTokenTypeResetPassword,
	})
	assert.True(t, trace.IsBadParameter(err), "expected trace.BadParameter, got %T %v", err, err)
}

func TestResetUser_AdminMFARequired(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	authzContext, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	}, &types.SessionRecordingConfigV2{})
	require.NoError(t, err)

	env, err := newTestEnv(withAuthorizer(fakeAuthorizer{
		authorize:    true,
		authzContext: authzContext,
	}))
	require.NoError(t, err, "creating test service")

	user, err := types.NewUser("dave")
	require.NoError(t, err)
	_, err = env.CreateUser(ctx, &userspb.CreateUserRequest{User: user.(*types.UserV2)})
	require.NoError(t, err)
	err = env.backend.UpsertPassword(user.GetName(), []byte("dummypasswordhash"))
	require.NoError(t, err)

	authzContext.AdminActionAuthState = authz.AdminActionAuthUnauthorized
	_, err = env.ResetUser(ctx, &userspb.ResetUserRequest{
		Name: user.GetName(),
		Type: authclient.UserTokenTypeResetPassword,
	})
	assert.True(t, trace.IsAccessDenied(err), "expected trace.AccessDenied, got %T %v", err, err)

	// The password should remain intact.
	hash, err := env.backend.GetPasswordHash(user.GetName())
	require.NoError(t, err)
	assert.Equal(t, "dummypasswordhash", string(hash))
}

func getAllEvents(c <-chan apievents.AuditEvent) []apievents.AuditEvent {
	var events []apievents.AuditEvent
	for {
		select {
		case e := <-c:
			events = append(events, e)
		default:
			return events
		}
	}
}

func TestResetCredentials_ResetsPassword(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	dave, err := types.NewUser("dave")
	require.NoError(t, err, "creating new user dave")

	_, err = env.CreateUser(ctx, &userspb.CreateUserRequest{User: dave.(*types.UserV2)})
	require.NoError(t, err, "creating user dave")

	// Using the Identity service makes it easier to set up the test case.
	err = env.backend.UpsertPassword("dave", []byte("it's full of stars!"))
	require.NoError(t, err)

	// Reset password.
	_, err = env.ResetCredentials(ctx, "dave")
	require.NoError(t, err)

	// Make sure that the password has been reset.
	u, err := env.backend.GetUser(ctx, "dave", true /* withSecrets */)
	require.NoError(t, err)
	assert.Nil(t, u.GetLocalAuth(), "user LocalAuth not nil")
	assert.Equal(t, types.PasswordState_PASSWORD_STATE_UNSET, u.GetPasswordState())

	// Make sure that we can reset once again (i.e. we don't complain if there's
	// no password).
	_, err = env.ResetCredentials(ctx, "dave")
	require.NoError(t, err)
}
