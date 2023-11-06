// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usersv1

import (
	"context"
	"encoding/base32"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
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
			Identity: identity,
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
				Groups: []string{"dev"},
			},
		},
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

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, kind string, verb string, silent bool) error {
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
	for _, role := range f.roles {
		if role == target {
			return true
		}
	}

	return false
}

type serviceOpt = func(*Service)

func withAuthorizer(authz authz.Authorizer) serviceOpt {
	return func(service *Service) {
		service.authorizer = authz
	}
}

type env struct {
	*Service
	emitter *eventstest.ChannelEmitter
	backend Backend
}

func newTestEnv(opts ...serviceOpt) (*env, error) {
	bk, err := memory.New(memory.Config{})
	if err != nil {
		return nil, trace.Wrap(err, "creating memory backend")
	}

	service := struct {
		services.Identity
		services.Access
	}{
		Identity: local.NewIdentityService(bk),
		Access:   local.NewAccessService(bk),
	}

	emitter := eventstest.NewChannelEmitter(10)

	svc, err := NewService(ServiceConfig{
		Authorizer: fakeAuthorizer{authorize: true},
		Cache:      service,
		Backend:    service,
		Emitter:    emitter,
		Reporter:   usagereporter.DiscardUsageReporter{},
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating users service")
	}

	for _, opt := range opts {
		opt(svc)
	}

	return &env{
		Service: svc,
		emitter: emitter,
		backend: service,
	}, nil
}

func TestCreateUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := context.Background()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	// Create a new user.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	// Validate that the user now exists.
	resp, err := env.GetUser(ctx, &userspb.GetUserRequest{Name: created.User.GetName()})
	require.NoError(t, err, "failed getting created user")
	require.Empty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	// Attempt to create a duplicate user
	created2, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	assert.Error(t, err, "duplicate user was created successfully")
	assert.Nil(t, created2, "received unexpected user ")
	require.True(t, trace.IsAlreadyExists(err), "creating duplicate user allowed")

	event := <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := context.Background()

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

	ctx := context.Background()

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
	assert.Empty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"), cmpopts.IgnoreFields(types.UserSpecV2{}, "LocalAuth")))
	assert.Nil(t, resp.User.GetLocalAuth(), "user secrets were provided when not requested")

	// Validate that getting the current user returns "alice" and not "llama".
	resp, err = env.GetUser(authz.ContextWithUser(ctx, &authz.LocalUser{
		Username: "alice",
		Identity: tlsca.Identity{
			Groups: []string{"dev"},
		},
	}), &userspb.GetUserRequest{CurrentUser: true})
	assert.NoError(t, err, "failed getting created user")
	assert.NotEmpty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
	assert.Equal(t, "alice", resp.User.GetName(), "expected current user to return alice")
	assert.Nil(t, resp.User.GetLocalAuth(), "secrets returned with current user")

	// Validate that requesting a users secrets returns them.
	resp, err = env.GetUser(ctx, &userspb.GetUserRequest{Name: created.User.GetName(), WithSecrets: true})
	assert.NoError(t, err, "failed getting created user")
	assert.Empty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
	assert.NotEmpty(t, cmp.Diff(created.User, resp.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
	assert.Equal(t, "alice", resp.User.GetName(), "expected current user to return alice")
	assert.Nil(t, resp.User.GetLocalAuth(), "secrets returned with current user")
}

func TestUpdateUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := context.Background()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	// Attempt to update a nonexistent user.
	updated, err := env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: llama.(*types.UserV2)})
	assert.Error(t, err, "duplicate user was created successfully")
	assert.Nil(t, updated, "received unexpected user")
	require.True(t, trace.IsNotFound(err), "updated nonexistent user")

	// Create a new user.
	created, err := env.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
	require.NoError(t, err, "creating user llama")

	event := <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")

	// Attempt to update the user again.
	created.User.SetLogins([]string{"alpaca"})
	updated, err = env.UpdateUser(ctx, &userspb.UpdateUserRequest{User: created.User})
	require.NoError(t, err, "failed updating user")
	require.Empty(t, cmp.Diff(created.User, updated.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
	require.Equal(t, []string{"alpaca"}, updated.User.GetLogins(), "logins were not updated")

	event = <-env.emitter.C()
	assert.Equal(t, events.UserUpdatedEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserUpdateCode, event.GetCode(), "unexpected event code")
}

func TestUpsertUser(t *testing.T) {
	t.Parallel()
	env, err := newTestEnv()
	require.NoError(t, err, "creating test service")

	ctx := context.Background()

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

	// Attempt to update the user again.
	upserted.User.SetLogins([]string{"alpaca"})
	updated, err := env.UpsertUser(ctx, &userspb.UpsertUserRequest{User: upserted.User})
	require.NoError(t, err, "failed upserting user")
	require.Empty(t, cmp.Diff(upserted.User, updated.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
	require.Equal(t, []string{"alpaca"}, updated.User.GetLogins(), "logins were not updated")

	event = <-env.emitter.C()
	assert.Equal(t, events.UserCreateEvent, event.GetType(), "unexpected event type")
	assert.Equal(t, events.UserCreateCode, event.GetCode(), "unexpected event code")
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

	ctx := context.Background()

	llama, err := types.NewUser("llama")
	require.NoError(t, err, "creating new user llama")

	tests := []struct {
		desc         string
		f            func(t *testing.T, service *Service)
		checker      *fakeChecker
		expectChecks []check
	}{
		{
			desc: "get no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetUser(ctx, &userspb.GetUserRequest{Name: "alice"})
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
			f: func(t *testing.T, service *Service) {
				user, err := service.GetUser(ctx, &userspb.GetUserRequest{CurrentUser: true})
				assert.NoError(t, err, "expected RBAC to allow getting the current user")
				assert.Empty(t, cmp.Diff(llama, user.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
			f: func(t *testing.T, service *Service) {
				_, err := service.GetUser(ctx, &userspb.GetUserRequest{Name: "alice", WithSecrets: true})
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
			desc: "create no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.CreateUser(ctx, &userspb.CreateUserRequest{User: llama.(*types.UserV2)})
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
			f: func(t *testing.T, service *Service) {
				u := utils.CloneProtoMsg(llama.(*types.UserV2))
				u.SetName("alpaca")
				created, err := service.CreateUser(ctx, &userspb.CreateUserRequest{User: u})
				assert.NoError(t, err, "expected RBAC to allow creating user")
				assert.Empty(t, cmp.Diff(u, created.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
			f: func(t *testing.T, service *Service) {
				_, err := service.UpdateUser(ctx, &userspb.UpdateUserRequest{User: llama.(*types.UserV2)})
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
			f: func(t *testing.T, service *Service) {
				u := utils.CloneProtoMsg(llama.(*types.UserV2))
				u.SetLogins([]string{"alpaca"})
				updated, err := service.UpdateUser(ctx, &userspb.UpdateUserRequest{User: u})
				assert.NoError(t, err, "expected RBAC to allow updating user")
				assert.Empty(t, cmp.Diff(u, updated.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
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
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
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
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
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
			f: func(t *testing.T, service *Service) {
				upserted, err := service.UpsertUser(ctx, &userspb.UpsertUserRequest{User: llama.(*types.UserV2)})
				assert.NoError(t, err, "expected RBAC to allow updating user")
				assert.Empty(t, cmp.Diff(llama, upserted.User, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
			f: func(t *testing.T, service *Service) {
				_, err := service.DeleteUser(ctx, &userspb.DeleteUserRequest{Name: llama.GetName()})
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
			f: func(t *testing.T, service *Service) {
				_, err := service.DeleteUser(ctx, &userspb.DeleteUserRequest{Name: llama.GetName()})
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
			}}))
			require.NoError(t, err, "creating test service")

			// Create the user directly on the backend to bypass RBAC enforced by the test cases.
			_, err = env.backend.CreateUser(ctx, llama.(*types.UserV2))
			require.NoError(t, err, "creating test user")

			// Validate RBAC is enforced.
			test.f(t, env.Service)
			require.ElementsMatch(t, test.expectChecks, test.checker.checks)
		})
	}

}
