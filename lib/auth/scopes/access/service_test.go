// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package accesscontrol

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedaccesscache "github.com/gravitational/teleport/lib/scopes/cache/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

const testClusterName = "test-cluster"

// fakeSplitAuthorizer is a mock implementation of ScopedAuthorizer that provides a hard-coded context.
type fakeSplitAuthorizer struct {
	ctx *authz.ScopedContext
}

func (a *fakeSplitAuthorizer) AuthorizeScoped(ctx context.Context) (*authz.ScopedContext, error) {
	return a.ctx, nil
}

// newFakeScopedAuthorizer builds a fake split authorizer with a hard-coded context based on the provided scoped access info and reader.
// this means that while the identity/assignments can be fake, the underlying reader must contain the expected scoped
// roles in order for the context to be built successfully.
func newFakeScopedAuthorizer(t *testing.T, accessInfo *services.AccessInfo, reader services.ScopedRoleReader) *fakeSplitAuthorizer {
	t.Helper()

	scopedCtx, err := services.NewScopedAccessCheckerContext(t.Context(), accessInfo, testClusterName, reader)
	require.NoError(t, err)

	return &fakeSplitAuthorizer{
		ctx: &authz.ScopedContext{
			User: &types.UserV2{
				Metadata: types.Metadata{
					Name: accessInfo.Username,
				},
			},
			CheckerContext: scopedCtx,
		},
	}
}

// newFakeUnscopedAuthorizer builds a fake split authorizer with a hard-coded context based on the provided unscoped access info and reader.
// this means that while the identity can be fake, the underlying reader must contain the expected classic roles in order for the context to
// be built successfully.
func newFakeUnscopedAuthorizer(t *testing.T, accessInfo *services.AccessInfo, reader services.RoleGetter) *fakeSplitAuthorizer {
	t.Helper()

	checker, err := services.NewAccessChecker(accessInfo, testClusterName, reader)
	require.NoError(t, err)

	return &fakeSplitAuthorizer{
		ctx: &authz.ScopedContext{
			User: &types.UserV2{
				Metadata: types.Metadata{
					Name: accessInfo.Username,
				},
			},
			CheckerContext: services.NewScopedAccessCheckerContextFromUnscoped(checker),
		},
	}
}

// newServerForIdentity builds a server with an access checker that is hard-coded to the provided access info. The backend pack
// much be pre-seeded with the relevant scoped/unscoped roles, but assignments are drawn from the access info (as they would be
// if the access info was being taken from a certificate).
func newServerForIdentity(t *testing.T, bk *backendPack, accessInfo *services.AccessInfo) *Server {
	t.Helper()

	var authz authz.ScopedAuthorizer
	if accessInfo.ScopePin != nil {
		authz = newFakeScopedAuthorizer(t, accessInfo, bk.cache)
	} else {
		authz = newFakeUnscopedAuthorizer(t, accessInfo, bk.classicService)
	}

	srv, err := New(Config{
		ScopedAuthorizer: authz,
		Reader:           bk.cache,
		Writer:           bk.service,
		BackendReader:    bk.service,
	})
	require.NoError(t, err)

	return srv
}

type backendPack struct {
	backend        backend.Backend
	service        *local.ScopedAccessService
	classicService *local.AccessService
	cache          *scopedaccesscache.Cache
}

func (p *backendPack) Close() {
	p.cache.Close()
	p.backend.Close()
}

// newBackendPack creates a scoped access service and populates it with the provided scoped roles.
func newBackendPack(t *testing.T) *backendPack {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { backend.Close() })

	service := local.NewScopedAccessService(backend)
	classicService := local.NewAccessService(backend)
	events := local.NewEventsService(backend)

	cache, err := scopedaccesscache.NewCache(scopedaccesscache.CacheConfig{
		Events: events,
		Reader: service,
	})
	require.NoError(t, err)

	select {
	case <-cache.Init():
	case <-time.After(30 * time.Second):
		require.FailNow(t, "timed out waiting for scoped access cache to initialize")
	}

	return &backendPack{
		backend:        backend,
		service:        service,
		classicService: classicService,
		cache:          cache,
	}
}

// TestRoleBasics verifies that basic CRUD operations on scoped roles work as expected, with a focus on ensuring that
// pinned scopes and role permissions are being properly enforced.
func TestRoleBasics(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newBackendPack(t)
	defer bk.Close()

	initialRoles := []*scopedaccessv1.ScopedRole{
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-admin",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "prod-admin",
			},
			Scope: "/prod",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/prod"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
					},
				},
			},
			Version: types.V1,
		},
	}

	for _, role := range initialRoles {
		// bootstrap in an initial role so that we can start using scoped permissions for our tests
		_, err := bk.service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
	}

	// wait for roles to be populated into cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		return len(roles) == 2
	})

	// set up server pinned to a staging admin identity
	srv := newServerForIdentity(t, bk, &services.AccessInfo{
		ScopePin: &scopesv1.Pin{
			Scope: "/staging",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/staging": {"/staging": {"staging-admin"}},
			}),
		},
		Username: "alice",
	})

	// verify expected successful read
	rrsp, err := srv.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "staging-admin",
	})
	require.NoError(t, err)
	require.Equal(t, "staging-admin", rrsp.GetRole().GetMetadata().GetName())
	require.Equal(t, "/staging", rrsp.GetRole().GetScope())

	// verify expected denied read
	_, err = srv.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "prod-admin",
	})
	require.Error(t, err)
	// within the scopes model, getting a disallowed resource by its name is always considered an access denied,
	// this is a divergence from our traditional RBAC model where a not found might be returned instead. scopes don't
	// "hide" primary keys in the same manner if the caller has scoped read permissions.
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify expected successful list request includes only allowed roles
	lrsp, err := srv.ListScopedRoles(ctx, &scopedaccessv1.ListScopedRolesRequest{})
	require.NoError(t, err)
	require.Empty(t, lrsp.GetNextPageToken())
	require.Len(t, lrsp.GetRoles(), 1)
	require.Equal(t, "staging-admin", lrsp.GetRoles()[0].GetMetadata().GetName())

	// verify expected successful create
	crsp, err := srv.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-user",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
					},
				},
			},
			Version: types.V1,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "staging-user", crsp.GetRole().GetMetadata().GetName())

	// wait for roles to be populated into cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		return len(roles) == 3
	})

	// verify expected denied create (out of scope)
	_, err = srv.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "prod-user",
			},
			Scope: "/prod",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/prod"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
					},
				},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that denied create really didn't create the role (requires using backend service
	// directly to avoid false positive due to cache replication)
	rrsp, err = bk.service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "prod-user",
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected not found error, got: %v", err)
	require.Nil(t, rrsp)

	// verify expected successful update

	// start by getting the existing role
	rrsp, err = srv.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "staging-user",
	})
	require.NoError(t, err)

	// modify the role
	rrsp.Role.Metadata.Labels = map[string]string{
		"key": "val",
	}

	// update the role
	ursp, err := srv.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: rrsp.GetRole(),
	})
	require.NoError(t, err)
	require.Equal(t, "staging-user", ursp.GetRole().GetMetadata().GetName())
	require.NotEqual(t, rrsp.GetRole().GetMetadata().GetRevision(), ursp.GetRole().GetMetadata().GetRevision())

	// observe change in cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		for _, role := range roles {
			if role.GetMetadata().GetName() == "staging-user" {
				if val, ok := role.GetMetadata().GetLabels()["key"]; ok && val == "val" {
					return true
				}
			}
		}
		return false
	})

	// verify expected denied update (out of scope)

	// start by getting the existing role (requires using backend service
	// directly since our server is using a scoped identity)
	rrsp, err = bk.service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "prod-admin",
	})
	require.NoError(t, err)

	// modify the role
	rrsp.Role.Metadata.Labels = map[string]string{
		"key": "val",
	}

	// attempt to update the role
	ursp, err = srv.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: rrsp.GetRole(),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
	require.Nil(t, ursp)

	// verify that denied update really didn't update the role (requires using backend service
	// directly to avoid false positive due to cache replication)
	rrsp, err = bk.service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "prod-admin",
	})
	require.NoError(t, err)
	require.Nil(t, rrsp.GetRole().GetMetadata().GetLabels())

	// verify expected successful delete
	_, err = srv.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: "staging-user",
	})
	require.NoError(t, err)

	// wait for deletion to be populated into cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		for _, role := range roles {
			if role.GetMetadata().GetName() == "staging-user" {
				return false
			}
		}
		return true
	})

	// verify expected denied delete (out of scope)
	_, err = srv.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: "prod-admin",
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that denied delete really didn't delete the role (requires using backend service
	// directly to avoid false positive due to cache replication)
	rrsp, err = bk.service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "prod-admin",
	})
	require.NoError(t, err)
	require.Equal(t, "prod-admin", rrsp.GetRole().GetMetadata().GetName())

	// verify expected successful upsert (creates new role)
	ursp2, err := srv.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-upserted",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
			},
			Version: types.V1,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "staging-upserted", ursp2.GetRole().GetMetadata().GetName())

	// wait for upserted role to appear in cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		for _, role := range roles {
			if role.GetMetadata().GetName() == "staging-upserted" {
				return true
			}
		}
		return false
	})

	// verify expected successful upsert (updates existing role)
	ursp2.Role.Metadata.Labels = map[string]string{"upserted": "true"}
	ursp3, err := srv.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: ursp2.GetRole(),
	})
	require.NoError(t, err)
	require.Equal(t, "staging-upserted", ursp3.GetRole().GetMetadata().GetName())
	require.Equal(t, "true", ursp3.GetRole().GetMetadata().GetLabels()["upserted"])

	// verify expected denied upsert (out of scope)
	_, err = srv.UpsertScopedRole(ctx, &scopedaccessv1.UpsertScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "prod-upserted",
			},
			Scope: "/prod",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/prod"},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
}

// TestAssignmentBasics verifies that basic CRUD operations on scoped role assignments work as expected, with a focus on ensuring that
// pinned scopes and role permissions are being properly enforced.
func TestAssignmentBasics(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newBackendPack(t)
	defer bk.Close()

	initialRoles := []*scopedaccessv1.ScopedRole{
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "staging-admin",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
					},
				},
			},
			Version: types.V1,
		},
		{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "prod-admin",
			},
			Scope: "/prod",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/prod"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
					},
				},
			},
			Version: types.V1,
		},
	}

	for _, role := range initialRoles {
		// bootstrap in an initial role so that we can start using scoped permissions for our tests
		_, err := bk.service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
			Role: role,
		})
		require.NoError(t, err)
	}

	// wait for roles to be populated into cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		return len(roles) == 2
	})

	// set up some initial assignments
	initialAssignments := []*scopedaccessv1.ScopedRoleAssignment{
		newScopedRoleAssignmentAtScope("staging-admin", "/staging"),
		newScopedRoleAssignmentAtScope("prod-admin", "/prod"),
	}

	for _, assignment := range initialAssignments {
		_, err := bk.service.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: assignment,
		})
		require.NoError(t, err)
	}

	// wait for assignments to be populated into cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		return len(assignments) == 2
	})

	// set up server pinned to a staging admin identity
	srv := newServerForIdentity(t, bk, &services.AccessInfo{
		ScopePin: &scopesv1.Pin{
			Scope: "/staging",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/staging": {"/staging": {"staging-admin"}},
			}),
		},
		Username: "alice",
	})

	// verify expected successful read
	rasp, err := srv.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: initialAssignments[0].GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.Equal(t, initialAssignments[0].GetMetadata().GetName(), rasp.GetAssignment().GetMetadata().GetName())
	require.Equal(t, "/staging", rasp.GetAssignment().GetScope())

	// verify expected denied read
	rasp, err = srv.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: initialAssignments[1].GetMetadata().GetName(),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
	require.Nil(t, rasp)

	// verify expected successful list request includes only allowed assignments
	lasp, err := srv.ListScopedRoleAssignments(ctx, &scopedaccessv1.ListScopedRoleAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, lasp.GetNextPageToken())
	require.Len(t, lasp.GetAssignments(), 1)
	require.Equal(t, initialAssignments[0].GetMetadata().GetName(), lasp.GetAssignments()[0].GetMetadata().GetName())

	// verify expected successful create
	a1 := newScopedRoleAssignmentAtScope("staging-admin", "/staging")
	carsp, err := srv.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: a1,
	})
	require.NoError(t, err)
	require.Equal(t, a1.GetMetadata().GetName(), carsp.GetAssignment().GetMetadata().GetName())

	// wait for assignments to be populated into cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		return len(assignments) == 3
	})

	// verify expected denied create (out of scope)
	a2 := newScopedRoleAssignmentAtScope("prod-admin", "/prod")
	carsp, err = srv.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: a2,
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
	require.Nil(t, carsp)

	// verify that denied create really didn't create the assignment (requires using backend service
	// directly to avoid false positive due to cache replication)
	garsp, err := bk.service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: a2.GetMetadata().GetName(),
	})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "expected not found error, got: %v", err)
	require.Nil(t, garsp)

	// verify expected successful delete
	_, err = srv.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: a1.GetMetadata().GetName(),
	})
	require.NoError(t, err)

	// wait for deletion to be populated into cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		for _, assignment := range assignments {
			if assignment.GetMetadata().GetName() == a1.GetMetadata().GetName() {
				return false
			}
		}
		return true
	})

	// verify expected denied delete (out of scope)
	_, err = srv.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: initialAssignments[1].GetMetadata().GetName(),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that denied delete really didn't delete the assignment (requires using backend service
	// directly to avoid false positive due to cache replication)
	rasp, err = bk.service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: initialAssignments[1].GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.Equal(t, initialAssignments[1].GetMetadata().GetName(), rasp.GetAssignment().GetMetadata().GetName())

	// verify expected successful update

	// create an assignment to update
	a3 := newScopedRoleAssignmentAtScope("staging-admin", "/staging")
	ca3rsp, err := srv.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: a3,
	})
	require.NoError(t, err)

	// wait for assignment to be populated into cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		for _, a := range assignments {
			if a.GetMetadata().GetName() == a3.GetMetadata().GetName() {
				return true
			}
		}
		return false
	})

	// add a label and update
	ca3rsp.Assignment.Metadata.Labels = map[string]string{"key": "val"}
	ua3rsp, err := srv.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: ca3rsp.GetAssignment(),
	})
	require.NoError(t, err)
	require.Equal(t, a3.GetMetadata().GetName(), ua3rsp.GetAssignment().GetMetadata().GetName())
	require.NotEqual(t, ca3rsp.GetAssignment().GetMetadata().GetRevision(), ua3rsp.GetAssignment().GetMetadata().GetRevision())

	// observe change in cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		for _, a := range assignments {
			if a.GetMetadata().GetName() == a3.GetMetadata().GetName() {
				if val, ok := a.GetMetadata().GetLabels()["key"]; ok && val == "val" {
					return true
				}
			}
		}
		return false
	})

	// verify expected denied update (out of scope)

	// start by getting the existing assignment (requires using backend service
	// directly since our server is using a scoped identity)
	garsp, err = bk.service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: initialAssignments[1].GetMetadata().GetName(),
	})
	require.NoError(t, err)

	// attempt to update the out-of-scope assignment
	garsp.Assignment.Metadata.Labels = map[string]string{"key": "val"}
	uarsp, err := srv.UpdateScopedRoleAssignment(ctx, &scopedaccessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: garsp.GetAssignment(),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
	require.Nil(t, uarsp)

	// verify that denied update really didn't update the assignment
	garsp, err = bk.service.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: initialAssignments[1].GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.Nil(t, garsp.GetAssignment().GetMetadata().GetLabels())

	// verify expected successful upsert (creates new assignment)
	a4 := newScopedRoleAssignmentAtScope("staging-admin", "/staging")
	ua4rsp, err := srv.UpsertScopedRoleAssignment(ctx, &scopedaccessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: a4,
	})
	require.NoError(t, err)
	require.Equal(t, a4.GetMetadata().GetName(), ua4rsp.GetAssignment().GetMetadata().GetName())

	// wait for upserted assignment to appear in cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		for _, a := range assignments {
			if a.GetMetadata().GetName() == a4.GetMetadata().GetName() {
				return true
			}
		}
		return false
	})

	// verify expected successful upsert (updates existing assignment)
	ua4rsp.Assignment.Metadata.Labels = map[string]string{"upserted": "true"}
	ua4rsp2, err := srv.UpsertScopedRoleAssignment(ctx, &scopedaccessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: ua4rsp.GetAssignment(),
	})
	require.NoError(t, err)
	require.Equal(t, a4.GetMetadata().GetName(), ua4rsp2.GetAssignment().GetMetadata().GetName())
	require.Equal(t, "true", ua4rsp2.GetAssignment().GetMetadata().GetLabels()["upserted"])

	// verify expected denied upsert (out of scope)
	a5 := newScopedRoleAssignmentAtScope("prod-admin", "/prod")
	_, err = srv.UpsertScopedRoleAssignment(ctx, &scopedaccessv1.UpsertScopedRoleAssignmentRequest{
		Assignment: a5,
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
}

func newScopedRoleAssignmentAtScope(roleName string, scope string) *scopedaccessv1.ScopedRoleAssignment {
	return &scopedaccessv1.ScopedRoleAssignment{
		Kind: scopedaccess.KindScopedRoleAssignment,
		Metadata: &headerv1.Metadata{
			Name: uuid.New().String(),
		},
		Scope: scope,
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User: "bob",
			Assignments: []*scopedaccessv1.Assignment{
				{
					Role:  roleName,
					Scope: scope,
				},
			},
		},
		Version: types.V1,
	}
}

// TestUnscopedBasics verifies that unscoped access control works as expected.
func TestUnscopedBasics(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newBackendPack(t)
	defer bk.Close()

	classicRoles := []*types.RoleV6{
		{
			Metadata: types.Metadata{
				Name: "unscoped-auditor",
			},
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Rules: []types.Rule{
						{
							Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
							Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
						},
					},
				},
			},
			Version: types.V8,
		},
		{
			Metadata: types.Metadata{
				Name: "unscoped-admin",
			},
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Rules: []types.Rule{
						{
							Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
							Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
						},
					},
				},
			},
			Version: types.V8,
		},
	}

	for _, role := range classicRoles {
		_, err := bk.classicService.CreateRole(ctx, role)
		require.NoError(t, err)
	}

	// set up server pinned to unscoped admin identity
	srvAlice := newServerForIdentity(t, bk, &services.AccessInfo{
		Username: "alice",
		Roles:    []string{"unscoped-admin"},
	})

	// set up server pinned to unscoped auditor identity
	srvBob := newServerForIdentity(t, bk, &services.AccessInfo{
		Username: "bob",
		Roles:    []string{"unscoped-auditor"},
	})

	// verify that admin can create a role
	crsp, err := srvAlice.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "some-role",
			},
			Scope: "/some-scope",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/some-scope"},
			},
			Version: types.V1,
		},
	})
	require.NoError(t, err)

	// wait for roles to be populated into cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		return len(roles) == 1
	})

	// verify that admin can read the role
	rrsp, err := srvAlice.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "some-role",
	})
	require.NoError(t, err)
	require.Equal(t, "some-role", rrsp.GetRole().GetMetadata().GetName())

	// verify that admin can list the role
	lrsp, err := srvAlice.ListScopedRoles(ctx, &scopedaccessv1.ListScopedRolesRequest{})
	require.NoError(t, err)
	require.Empty(t, lrsp.GetNextPageToken())
	require.Len(t, lrsp.GetRoles(), 1)
	require.Equal(t, "some-role", lrsp.GetRoles()[0].GetMetadata().GetName())

	// verify that auditor cannot create a role
	_, err = srvBob.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind: scopedaccess.KindScopedRole,
			Metadata: &headerv1.Metadata{
				Name: "some-other-role",
			},
			Scope: "/some-scope",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/some-scope"},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that auditor can read the admin-created role
	rrsp, err = srvBob.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "some-role",
	})
	require.NoError(t, err)
	require.Equal(t, crsp.GetRole().GetMetadata().GetName(), rrsp.GetRole().GetMetadata().GetName())

	// verify that auditor can list the admin-created role
	lrsp, err = srvBob.ListScopedRoles(ctx, &scopedaccessv1.ListScopedRolesRequest{})
	require.NoError(t, err)
	require.Empty(t, lrsp.GetNextPageToken())
	require.Len(t, lrsp.GetRoles(), 1)
	require.Equal(t, crsp.GetRole().GetMetadata().GetName(), lrsp.GetRoles()[0].GetMetadata().GetName())

	// verify that admin can create an assignment
	acrsp, err := srvAlice.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerv1.Metadata{
				Name: uuid.New().String(),
			},
			Scope: "/some-scope",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "some-role",
						Scope: "/some-scope",
					},
				},
			},
			Version: types.V1,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/some-scope", acrsp.GetAssignment().GetScope())

	// wait for assignments to be populated into cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		return len(assignments) == 1
	})

	// verify that admin can read the assignment
	rasp, err := srvAlice.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: acrsp.GetAssignment().GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.Equal(t, acrsp.GetAssignment().GetMetadata().GetName(), rasp.GetAssignment().GetMetadata().GetName())

	// verify that admin can list the assignment
	lasp, err := srvAlice.ListScopedRoleAssignments(ctx, &scopedaccessv1.ListScopedRoleAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, lasp.GetNextPageToken())
	require.Len(t, lasp.GetAssignments(), 1)
	require.Equal(t, acrsp.GetAssignment().GetMetadata().GetName(), lasp.GetAssignments()[0].GetMetadata().GetName())

	// verify that auditor cannot create an assignment
	_, err = srvBob.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind: scopedaccess.KindScopedRoleAssignment,
			Metadata: &headerv1.Metadata{
				Name: uuid.New().String(),
			},
			Scope: "/some-scope",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{
					{
						Role:  "some-role",
						Scope: "/some-scope",
					},
				},
			},
			Version: types.V1,
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that auditor can read the admin-created assignment
	rasp, err = srvBob.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
		Name: acrsp.GetAssignment().GetMetadata().GetName(),
	})
	require.NoError(t, err)
	require.Equal(t, acrsp.GetAssignment().GetMetadata().GetName(), rasp.GetAssignment().GetMetadata().GetName())

	// verify that auditor can list the admin-created assignment
	lasp, err = srvBob.ListScopedRoleAssignments(ctx, &scopedaccessv1.ListScopedRoleAssignmentsRequest{})
	require.NoError(t, err)
	require.Empty(t, lasp.GetNextPageToken())
	require.Len(t, lasp.GetAssignments(), 1)
	require.Equal(t, acrsp.GetAssignment().GetMetadata().GetName(), lasp.GetAssignments()[0].GetMetadata().GetName())

	// verify that admin can update roles

	// start by getting the existing role
	rrsp, err = srvAlice.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: "some-role",
	})
	require.NoError(t, err)
	rrsp.Role.Metadata.Labels = map[string]string{
		"key": "val",
	}

	// update the role
	ursp, err := srvAlice.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: rrsp.GetRole(),
	})
	require.NoError(t, err)
	require.Equal(t, crsp.GetRole().GetMetadata().GetName(), ursp.GetRole().GetMetadata().GetName())
	require.NotEqual(t, rrsp.GetRole().GetMetadata().GetRevision(), ursp.GetRole().GetMetadata().GetRevision())

	// verify that auditor cannot update roles
	rrsp, err = srvBob.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
		Name: crsp.GetRole().GetMetadata().GetName(),
	})
	require.NoError(t, err)
	rrsp.Role.Metadata.Labels = map[string]string{
		"key": "val2",
	}
	ursp, err = srvBob.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{
		Role: rrsp.GetRole(),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
	require.Nil(t, ursp)

	// verify that auditor cannot delete assignments
	_, err = srvBob.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: acrsp.GetAssignment().GetMetadata().GetName(),
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that admin can delete assignments
	_, err = srvAlice.DeleteScopedRoleAssignment(ctx, &scopedaccessv1.DeleteScopedRoleAssignmentRequest{
		Name: acrsp.GetAssignment().GetMetadata().GetName(),
	})
	require.NoError(t, err)

	// wait for deletion to be populated into cache
	waitForAssignmentCondition(t, bk.cache, func(assignments []*scopedaccessv1.ScopedRoleAssignment) bool {
		for _, assignment := range assignments {
			if assignment.GetMetadata().GetName() == acrsp.GetAssignment().GetMetadata().GetName() {
				return false
			}
		}
		return true
	})

	// verify that auditor cannot delete roles
	_, err = srvBob.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: "some-role",
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)

	// verify that admin can delete roles
	_, err = srvAlice.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{
		Name: "some-role",
	})
	require.NoError(t, err)
	// wait for deletion to be populated into cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		for _, role := range roles {
			if role.GetMetadata().GetName() == "some-role" {
				return false
			}
		}
		return true
	})
}

// TestAccessChecksSkipInconsistentAssignments verifies that role assignments which fail cross-resource
// consistency checks during access evaluation are silently skipped rather than treated as hard errors.
// Earlier iterations of scoped APIs used transactional logic to prevent malformed assignments, but that
// presented usability and maintainability issues.
func TestAccessChecksSkipInconsistentAssignments(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newBackendPack(t)
	defer bk.Close()

	// staging-reader grants only read access; staging-admin grants full read/write access.
	// alice is assigned both. When staging-admin is later made inconsistent, alice should
	// retain read access from staging-reader but lose write access.
	initialRoles := []*scopedaccessv1.ScopedRole{
		{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "staging-reader",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList},
					},
				},
			},
		},
		{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "staging-admin",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
				Rules: []*scopedaccessv1.ScopedRule{
					{
						Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
						Verbs:     []string{types.VerbReadNoSecrets, types.VerbList, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
					},
				},
			},
		},
	}

	for _, role := range initialRoles {
		_, err := bk.service.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{Role: role})
		require.NoError(t, err)
	}

	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		return len(roles) == 2
	})

	// alice is assigned both roles at /staging (this is the "certificate" state — it does not change
	// even after we make staging-admin inconsistent below).
	aliceAccessInfo := &services.AccessInfo{
		ScopePin: &scopesv1.Pin{
			Scope: "/staging",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/staging": {"/staging": {"staging-reader", "staging-admin"}},
			}),
		},
		Username: "alice",
	}

	// with both roles consistent, alice has full read/write access.
	srv := newServerForIdentity(t, bk, aliceAccessInfo)

	_, err := srv.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "staging-probe",
			},
			Scope: "/staging",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/staging"},
			},
		},
	})
	require.NoError(t, err, "alice should have write access when both roles are consistent")

	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		return len(roles) == 3
	})

	// now update staging-admin to change its assignable scopes so that it no longer covers /staging.
	// this makes the assignment inconsistent: the role is still referenced in alice's certificate, but
	// it will fail RoleIsEnforceableAt during access checks and be skipped.
	adminRole, err := bk.service.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{Name: "staging-admin"})
	require.NoError(t, err)
	// /staging/sub is a valid sub-scope of the role's resource scope /staging, so it passes
	// StrongValidateRole — but it no longer covers /staging as a scope of effect, making the
	// existing assignment inconsistent.
	adminRole.Role.Spec.AssignableScopes = []string{"/staging/sub"}
	_, err = bk.service.UpdateScopedRole(ctx, &scopedaccessv1.UpdateScopedRoleRequest{Role: adminRole.GetRole()})
	require.NoError(t, err)

	// wait for the updated role to be visible in cache
	waitForRoleCondition(t, bk.cache, func(roles []*scopedaccessv1.ScopedRole) bool {
		for _, role := range roles {
			if role.GetMetadata().GetName() == "staging-admin" {
				return role.GetSpec().GetAssignableScopes()[0] == "/staging/sub"
			}
		}
		return false
	})

	// build a new server for alice with the *same* certificate (same assignment tree).
	// staging-admin is now inconsistent and must be skipped at the PDP.
	srv = newServerForIdentity(t, bk, aliceAccessInfo)

	// read access should still succeed — staging-reader is still consistent and grants it.
	_, err = srv.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{Name: "staging-probe"})
	require.NoError(t, err, "alice should retain read access from the still-consistent staging-reader role")

	// write access should now be denied — staging-admin is skipped, and staging-reader does not grant write.
	_, err = srv.DeleteScopedRole(ctx, &scopedaccessv1.DeleteScopedRoleRequest{Name: "staging-probe"})
	require.Error(t, err, "alice should lose write access when the write-granting role becomes inconsistent")
	require.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
}

func waitForRoleCondition(t *testing.T, reader services.ScopedRoleReader, condition func([]*scopedaccessv1.ScopedRole) bool) {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		var roles []*scopedaccessv1.ScopedRole
		for role, err := range scopedutils.RangeScopedRoles(t.Context(), reader, &scopedaccessv1.ListScopedRolesRequest{}) {
			require.NoError(t, err)
			roles = append(roles, role)
		}

		if condition(roles) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 120):
		case <-timeout:
			require.FailNow(t, "timeout waiting for role condition")
		}
	}
}

func waitForAssignmentCondition(t *testing.T, reader services.ScopedRoleAssignmentReader, condition func([]*scopedaccessv1.ScopedRoleAssignment) bool) {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		var assignments []*scopedaccessv1.ScopedRoleAssignment
		for assignment, err := range scopedutils.RangeScopedRoleAssignments(t.Context(), reader, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
			require.NoError(t, err)
			assignments = append(assignments, assignment)
		}

		if condition(assignments) {
			return
		}

		select {
		case <-time.After(time.Millisecond * 120):
		case <-timeout:
			require.FailNow(t, "timeout waiting for assignment condition")
		}
	}
}
