// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package statichostuser

import (
	"context"
	"fmt"
	"testing"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services/local"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type authorizerFactory func(t *testing.T, client test.LocalClient) authz.Authorizer

func staticHostUserName(i int) string {
	return fmt.Sprintf("user-%d", i)
}

func makeStaticHostUser(i int) *userprovisioningpb.StaticHostUser {
	name := staticHostUserName(i)
	return userprovisioning.NewStaticHostUser(name, &userprovisioningpb.StaticHostUserSpec{
		Login:  name,
		Groups: []string{"foo", "bar"},
		NodeLabels: &wrappers.LabelValues{
			Values: map[string]wrappers.StringValues{
				"foo": {
					Values: []string{"bar"},
				},
			},
		},
	})
}

func authorizeWithVerbs(verbs []string) authorizerFactory {
	return func(t *testing.T, client test.LocalClient) authz.Authorizer {
		return authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
			return test.AuthorizerForDummyUser(t, ctx, client, types.KindStaticHostUser, verbs), nil
		})
	}
}

func assertTraceErr(f func(error) bool) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...any) {
		require.Error(t, err)
		require.True(t, f(err), "unexpected error: %v", err)
	}
}

func TestStaticHostUserCRUD(t *testing.T) {
	t.Parallel()

	accessTests := []struct {
		name       string
		request    func(ctx context.Context, svc *Service) error
		allowVerbs []string
		denyVerbs  []string
	}{
		{
			name: "list",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.ListStaticHostUsers(ctx, &userprovisioningpb.ListStaticHostUsersRequest{})
				return err
			},
			allowVerbs: []string{types.VerbList, types.VerbRead},
		},
		{
			name: "get",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.GetStaticHostUser(ctx, &userprovisioningpb.GetStaticHostUserRequest{
					Name: staticHostUserName(0),
				})
				return err
			},
			allowVerbs: []string{types.VerbRead},
		},
		{
			name: "create",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.CreateStaticHostUser(ctx, &userprovisioningpb.CreateStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			allowVerbs: []string{types.VerbCreate},
		},
		{
			name: "update",
			request: func(ctx context.Context, svc *Service) error {
				hostUser, err := svc.GetStaticHostUser(ctx, &userprovisioningpb.GetStaticHostUserRequest{
					Name: staticHostUserName(0),
				})
				if err != nil {
					// Don't return the error as-is; an access denied here might
					// cause a false positive.
					return trace.Errorf("GetStaticHostUser failed: %v", err)
				}
				hostUser.Spec.Login = "bob"
				_, err = svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: hostUser,
				})
				return err
			},
			allowVerbs: []string{types.VerbRead, types.VerbUpdate},
			denyVerbs:  []string{types.VerbRead},
		},
		{
			name: "upsert",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.UpsertStaticHostUser(ctx, &userprovisioningpb.UpsertStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			allowVerbs: []string{types.VerbCreate, types.VerbUpdate},
		},
		{
			name: "delete",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.DeleteStaticHostUser(ctx, &userprovisioningpb.DeleteStaticHostUserRequest{
					Name: staticHostUserName(0),
				})
				return err
			},
			allowVerbs: []string{types.VerbDelete},
		},
	}

	for _, tc := range accessTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			t.Run("allow", func(t *testing.T) {
				t.Parallel()
				// Create authorizer with required verbs.
				authorizer := authorizeWithVerbs(tc.allowVerbs)
				// CRUD action should succeed.
				testStaticHostUserAccess(t, authorizer, tc.request, require.NoError)
			})

			t.Run("deny", func(t *testing.T) {
				t.Parallel()
				// Create authorizer without required verbs.
				authorizer := authorizeWithVerbs(tc.denyVerbs)
				// CRUD action should fail.
				testStaticHostUserAccess(t, authorizer, tc.request, assertTraceErr(trace.IsAccessDenied))
			})
		})
	}

	otherTests := []struct {
		name    string
		request func(ctx context.Context, svc *Service) error
		verbs   []string
		assert  require.ErrorAssertionFunc
	}{
		{
			name: "get nonexistent resource",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.GetStaticHostUser(ctx, &userprovisioningpb.GetStaticHostUserRequest{
					Name: "fake",
				})
				return err
			},
			verbs:  []string{types.VerbRead},
			assert: assertTraceErr(trace.IsNotFound),
		},
		{
			name: "create resource twice",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.CreateStaticHostUser(ctx, &userprovisioningpb.CreateStaticHostUserRequest{
					User: makeStaticHostUser(0),
				})
				return err
			},
			verbs:  []string{types.VerbCreate},
			assert: assertTraceErr(trace.IsAlreadyExists),
		},
		{
			name: "delete nonexisting resource",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.DeleteStaticHostUser(ctx, &userprovisioningpb.DeleteStaticHostUserRequest{
					Name: staticHostUserName(10),
				})
				return err
			},
			verbs:  []string{types.VerbDelete},
			assert: assertTraceErr(trace.IsNotFound),
		},
		{
			name: "update with wrong revision",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: makeStaticHostUser(0),
				})
				return err
			},
			verbs:  []string{types.VerbUpdate},
			assert: assertTraceErr(trace.IsCompareFailed),
		},
		{
			name: "update nonexistent resource",
			request: func(ctx context.Context, svc *Service) error {
				_, err := svc.UpdateStaticHostUser(ctx, &userprovisioningpb.UpdateStaticHostUserRequest{
					User: makeStaticHostUser(10),
				})
				return err
			},
			verbs:  []string{types.VerbUpdate},
			assert: assertTraceErr(trace.IsCompareFailed),
		},
	}
	for _, tc := range otherTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authorizer := authorizeWithVerbs(tc.verbs)
			testStaticHostUserAccess(t, authorizer, tc.request, tc.assert)
		})
	}
}

func testStaticHostUserAccess(
	t *testing.T,
	authorizer func(t *testing.T, client test.LocalClient) authz.Authorizer,
	request func(ctx context.Context, svc *Service) error,
	assert require.ErrorAssertionFunc,
) {
	ctx, resourceSvc := initSvc(t, authorizer)
	err := request(ctx, resourceSvc)
	assert(t, err)
}

func initSvc(t *testing.T, authorizerFn authorizerFactory) (context.Context, *Service) {
	ctx, client, backend := test.InitRBACServices(t)
	localResourceService, err := local.NewStaticHostUserService(backend)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		_, err := localResourceService.CreateStaticHostUser(ctx, makeStaticHostUser(i))
		require.NoError(t, err)
	}

	resourceSvc, err := NewService(ServiceConfig{
		Authorizer: authorizerFn(t, client),
		Backend:    localResourceService,
	})
	require.NoError(t, err)
	return ctx, resourceSvc
}
