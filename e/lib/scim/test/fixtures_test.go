package test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/provider/okta/oktahandler"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type builtinRoleAuthorizer struct{}

func (builtinRoleAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	userI, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if role, ok := userI.(authz.BuiltinRole); ok {
		return authz.ContextForBuiltinRole(role, nil)
	}
	return nil, trace.AccessDenied("nope")
}

type testFixture struct {
	users           mockUserService
	roles           mockRoleService
	accesslists     mockAccessListService
	jwtSigner       mockJwtSigner
	locks           mockLocksService
	plugins         mockPluginsService
	creds           mockCredentialsService
	shim            *mockProviderShim
	clock           clocki.FakeClock
	userCtx         context.Context
	identityService mockIdentityService
	authorityGetter mockAuthorityGetter
	assignments     mockAssignmentsService
}

func (tf *testFixture) AssertExpectations(t *testing.T) {
	tf.users.AssertExpectations(t)
	tf.roles.AssertExpectations(t)
	tf.accesslists.AssertExpectations(t)
	tf.locks.AssertExpectations(t)
	tf.plugins.AssertExpectations(t)
	tf.creds.AssertExpectations(t)
	tf.shim.AssertExpectations(t)
	tf.identityService.AssertExpectations(t)
	tf.assignments.AssertExpectations(t)
}

// testBackend is a composite struct that satisfies the common.Backend
// interface by embedding both AccessLists and Assignments implementations.
type testBackend struct {
	common.AccessLists
	common.Assignments
	common.Locks
}

func (tf *testFixture) CheckAndSetDefaults(t *testing.T) {
	if tf.shim == nil {
		tf.shim = &mockProviderShim{}
	}

	if tf.clock == nil {
		tf.clock = clockwork.NewFakeClock()
	}

	if tf.userCtx == nil {
		tf.userCtx = authz.ContextWithUser(
			context.Background(),
			authz.BuiltinRole{
				Role:     types.RoleProxy,
				Username: string(types.RoleProxy),
			})
	}
}

func newTestServiceWith(t *testing.T, fix *testFixture) (*service.Service, *testFixture) {
	fix.CheckAndSetDefaults(t)

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	scimSvc, err := service.NewService(&common.Config{
		Authorizer: builtinRoleAuthorizer{},
		AccessPoint: &testServices{
			mockUserService:        &fix.users,
			mockRoleService:        &fix.roles,
			mockIdentityService:    &fix.identityService,
			mockPluginsService:     &fix.plugins,
			mockCredentialsService: &fix.creds,
			mockJwtSigner:          &fix.jwtSigner,
			mockAuthorityGetter:    &fix.authorityGetter,
			mockAccessListService:  &fix.accesslists,
			mockLocksService:       &fix.locks,
			Semaphores:             local.NewPresenceService(bk),
		},
		Backend: &testBackend{
			AccessLists: &fix.accesslists,
			Assignments: &fix.assignments,
			Locks:       &fix.locks,
		},
		Clock:       fix.clock,
		ClusterName: "test-cluster",
	})
	require.NoError(t, err, "creating test harness")

	// patch the services shim factory map to return our mock shim when asked to
	// create one for the test plugin type
	t.Cleanup(func() {
		// No sense in cluttering the output with missed expectations if the
		// test has already failed.
		if t.Failed() {
			return
		}
		fix.AssertExpectations(t)
	})

	scimSvc.CreatePluginHandler = func(plugin types.Plugin, config common.Config, resourceType string) (common.ResourceHandler, error) {
		switch resourceType {
		case "Users":
			return &oktahandler.UserHandler{
				Config:       config,
				ProviderUser: fix.shim,
			}, nil
		case "Groups":
			return &oktahandler.GroupHandler{
				Config:        config,
				ProviderGroup: fix.shim,
			}, nil
		default:
			return nil, trace.BadParameter("unsupported resource type %q", resourceType)
		}
	}
	return scimSvc, fix
}

func newTestService(t *testing.T) (*service.Service, *testFixture) {
	return newTestServiceWith(t, &testFixture{})
}
