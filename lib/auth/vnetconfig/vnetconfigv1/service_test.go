/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package vnetconfigv1

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func TestServiceAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	vnetConfig := &vnet.VnetConfig{
		Kind:    types.KindVnetConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "vnet-config",
		},
	}

	type testCase struct {
		name          string
		allowedVerbs  []string
		allowedStates []authz.AdminActionAuthState
		action        func(*Service) error
		requireEvent  apievents.AuditEvent
	}
	testCases := []testCase{
		{
			name: "CreateVnetConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbCreate},
			action: func(service *Service) error {
				_, err := service.CreateVnetConfig(ctx, &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
				return trace.Wrap(err)
			},
			requireEvent: &apievents.VnetConfigCreate{},
		},
		{
			name: "UpdateVnetConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate},
			action: func(service *Service) error {
				if _, err := service.storage.CreateVnetConfig(ctx, vnetConfig); err != nil {
					return trace.Wrap(err, "creating vnet_config as pre-req for Update test")
				}
				_, err := service.UpdateVnetConfig(ctx, &vnet.UpdateVnetConfigRequest{VnetConfig: vnetConfig})
				return trace.Wrap(err)
			},
			requireEvent: &apievents.VnetConfigUpdate{},
		},
		{
			name: "DeleteVnetConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbDelete},
			action: func(service *Service) error {
				if _, err := service.storage.CreateVnetConfig(ctx, vnetConfig); err != nil {
					return trace.Wrap(err, "creating vnet_config as pre-req for Delete test")
				}
				_, err := service.DeleteVnetConfig(ctx, &vnet.DeleteVnetConfigRequest{})
				return trace.Wrap(err)
			},
			requireEvent: &apievents.VnetConfigDelete{},
		},
		{
			name: "UpsertVnetConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbCreate, types.VerbUpdate},
			action: func(service *Service) error {
				_, err := service.UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
				return trace.Wrap(err)
			},
			requireEvent: &apievents.VnetConfigCreate{},
		},
		{
			name: "UpsertVnetConfig with existing",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbCreate, types.VerbUpdate},
			action: func(service *Service) error {
				if _, err := service.storage.CreateVnetConfig(ctx, vnetConfig); err != nil {
					return trace.Wrap(err, "creating vnet_config as pre-req for Upsert test")
				}
				_, err := service.UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
				return trace.Wrap(err)
			},
			requireEvent: &apievents.VnetConfigCreate{},
		},
		{
			name: "GetVnetConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized, authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified, authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead},
			action: func(service *Service) error {
				if _, err := service.storage.CreateVnetConfig(ctx, vnetConfig); err != nil {
					return trace.Wrap(err, "creating vnet_config as pre-req for Get test")
				}
				_, err := service.GetVnetConfig(ctx, &vnet.GetVnetConfigRequest{})
				return trace.Wrap(err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// test the method with allowed admin states, each one separately.
			t.Run("allowed admin states", func(t *testing.T) {
				for _, state := range tc.allowedStates {
					t.Run(stateToString(state), func(t *testing.T) {
						for _, verbs := range utils.Combinations(tc.allowedVerbs) {
							t.Run(fmt.Sprintf("verbs=%v", verbs), func(t *testing.T) {
								service, emitter := newService(t, state, fakeChecker{allowedVerbs: verbs})
								err := tc.action(service)
								// expect access denied except with full set of verbs.
								if len(verbs) == len(tc.allowedVerbs) {
									require.NoError(t, err, trace.DebugReport(err))
									if tc.requireEvent != nil {
										got := emitter.LastEvent()
										require.NotNil(t, got, "expected an audit event to be emitted")
										require.IsType(t, tc.requireEvent, got)
										require.True(t, eventStatusSuccess(t, got), "expected audit event status success to be true")
									}
								} else {
									require.True(t, trace.IsAccessDenied(err), "expected access denied for verbs %v, got err=%v", verbs, err)
									require.Empty(t, emitter.Events(), "expected no audit events on access denied")
								}
							})
						}
					})
				}
			})

			// test the method with disallowed admin states; expect failures.
			t.Run("disallowed admin states", func(t *testing.T) {
				disallowedStates := otherAdminStates(tc.allowedStates)
				for _, state := range disallowedStates {
					t.Run(stateToString(state), func(t *testing.T) {
						// it is enough to test against tc.allowedVerbs,
						// this is the only different data point compared to the test cases above.
						service, emitter := newService(t, state, fakeChecker{allowedVerbs: tc.allowedVerbs})
						err := tc.action(service)
						require.True(t, trace.IsAccessDenied(err))
						require.Empty(t, emitter.Events(), "expected no audit events on access denied")
					})
				}
			})

			// test the method with storage-layer errors
			t.Run("storage error", func(t *testing.T) {
				service, _ := newServiceWithStorage(t, tc.allowedStates[0],
					fakeChecker{allowedVerbs: tc.allowedVerbs}, badStorage{})
				err := tc.action(service)
				// the returned error should wrap the unexpected storage-layer error.
				require.ErrorIs(t, err, errBadStorage)
			})
		})
	}

	// verify that all declared methods have matching test cases
	t.Run("verify coverage", func(t *testing.T) {
		for _, method := range vnet.VnetConfigService_ServiceDesc.Methods {
			t.Run(method.MethodName, func(t *testing.T) {
				match := slices.ContainsFunc(testCases, func(tc testCase) bool {
					return strings.Contains(method.MethodName, tc.name)
				})
				require.True(t, match, "method %v without coverage, no matching tests", method.MethodName)
			})
		}
	})
}

func TestServiceScopedAccess(t *testing.T) {
	t.Parallel()

	vnetConfig := &vnet.VnetConfig{
		Kind:    types.KindVnetConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameVnetConfig,
		},
	}

	t.Run("read allowed despite scope pin", func(t *testing.T) {
		service, _ := newServiceWithScopedAuthorizer(t, newFakeScopedAuthorizer(t))

		// First create a VnetConfig at the storage later so the next step can read it.
		_, err := service.storage.CreateVnetConfig(t.Context(), vnetConfig)
		require.NoError(t, err)

		// The read should be allowed.
		_, err = service.GetVnetConfig(t.Context(), &vnet.GetVnetConfigRequest{})
		require.NoError(t, err)
	})

	t.Run("write denied for scoped identity", func(t *testing.T) {
		service, emitter := newServiceWithScopedAuthorizer(t, newFakeScopedAuthorizer(t))

		_, err := service.CreateVnetConfig(t.Context(), &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Empty(t, emitter.Events(), "expected no audit events on access denied")

		_, err = service.UpdateVnetConfig(t.Context(), &vnet.UpdateVnetConfigRequest{VnetConfig: vnetConfig})
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Empty(t, emitter.Events(), "expected no audit events on access denied")

		_, err = service.UpsertVnetConfig(t.Context(), &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Empty(t, emitter.Events(), "expected no audit events on access denied")

		_, err = service.DeleteVnetConfig(t.Context(), &vnet.DeleteVnetConfigRequest{})
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		require.Empty(t, emitter.Events(), "expected no audit events on access denied")
	})
}

func eventStatusSuccess(t *testing.T, evt apievents.AuditEvent) bool {
	t.Helper()

	switch e := evt.(type) {
	case *apievents.VnetConfigCreate:
		return e.Status.Success
	case *apievents.VnetConfigUpdate:
		return e.Status.Success
	case *apievents.VnetConfigDelete:
		return e.Status.Success
	default:
		t.Fatalf("unexpected audit event type %T", evt)
	}
	return false
}

var allAdminStates = map[authz.AdminActionAuthState]string{
	authz.AdminActionAuthUnauthorized:         "Unauthorized",
	authz.AdminActionAuthNotRequired:          "NotRequired",
	authz.AdminActionAuthMFAVerified:          "MFAVerified",
	authz.AdminActionAuthMFAVerifiedWithReuse: "MFAVerifiedWithReuse",
}

func stateToString(state authz.AdminActionAuthState) string {
	str, ok := allAdminStates[state]
	if !ok {
		return fmt.Sprintf("unknown(%v)", state)
	}
	return str
}

// otherAdminStates returns all admin states except for those passed in
func otherAdminStates(states []authz.AdminActionAuthState) []authz.AdminActionAuthState {
	var out []authz.AdminActionAuthState
	for state := range allAdminStates {
		found := slices.Index(states, state) != -1
		if !found {
			out = append(out, state)
		}
	}
	return out
}

type fakeChecker struct {
	allowedVerbs []string
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindVnetConfig {
		if slices.Contains(f.allowedVerbs, verb) {
			return nil
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

type fakeAuthorizer struct {
	authState authz.AdminActionAuthState
	checker   services.AccessChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	user, err := types.NewUser("alice")
	if err != nil {
		return nil, err
	}

	return &authz.Context{
		User:                 user,
		Checker:              f.checker,
		AdminActionAuthState: f.authState,
	}, nil
}

func (f *fakeAuthorizer) AuthorizeScoped(ctx context.Context) (*authz.ScopedContext, error) {
	authzCtx, err := f.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return authz.ScopedContextFromUnscopedContext(authzCtx), nil
}

type fakeScopedAuthorizer struct {
	ctx *authz.ScopedContext
}

func (f fakeScopedAuthorizer) AuthorizeScoped(context.Context) (*authz.ScopedContext, error) {
	return f.ctx, nil
}

type fakeScopedRoleReader struct {
	roles map[string]*scopedaccessv1.ScopedRole
}

func (f fakeScopedRoleReader) GetScopedRole(_ context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	role := f.roles[req.GetName()]
	if role == nil {
		return nil, trace.NotFound("scoped role %q not found", req.GetName())
	}
	return &scopedaccessv1.GetScopedRoleResponse{Role: role}, nil
}

func (f fakeScopedRoleReader) ListScopedRoles(context.Context, *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	roles := make([]*scopedaccessv1.ScopedRole, 0, len(f.roles))
	for _, role := range f.roles {
		roles = append(roles, role)
	}
	return &scopedaccessv1.ListScopedRolesResponse{Roles: roles}, nil
}

func newFakeScopedAuthorizer(t *testing.T) fakeScopedAuthorizer {
	t.Helper()

	checkerContext, err := services.NewScopedAccessCheckerContext(t.Context(), &services.AccessInfo{
		Username: "alice",
		ScopePin: &scopesv1.Pin{
			Scope: "/test",
		},
	}, "test-cluster", fakeScopedRoleReader{})
	require.NoError(t, err)

	return fakeScopedAuthorizer{
		ctx: &authz.ScopedContext{
			User: &types.UserV2{
				Metadata: types.Metadata{Name: "alice"},
			},
			CheckerContext: checkerContext,
		},
	}
}

func newService(t *testing.T, authState authz.AdminActionAuthState, checker services.AccessChecker) (*Service, *eventstest.MockRecorderEmitter) {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	storage, err := local.NewVnetConfigService(bk)
	require.NoError(t, err)

	return newServiceWithStorage(t, authState, checker, storage)
}

func newServiceWithStorage(t *testing.T, authState authz.AdminActionAuthState, checker services.AccessChecker, storage services.VnetConfigService) (*Service, *eventstest.MockRecorderEmitter) {
	t.Helper()

	emitter := &eventstest.MockRecorderEmitter{}

	authorizer := &fakeAuthorizer{
		authState: authState,
		checker:   checker,
	}

	service, err := NewService(ServiceConfig{
		ScopedAuthorizer: authorizer,
		Storage:          storage,
		Emitter:          emitter,
	})
	require.NoError(t, err)
	return service, emitter
}

func newServiceWithScopedAuthorizer(t *testing.T, authorizer authz.ScopedAuthorizer) (*Service, *eventstest.MockRecorderEmitter) {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	storage, err := local.NewVnetConfigService(bk)
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}
	service, err := NewService(ServiceConfig{
		ScopedAuthorizer: authorizer,
		Storage:          storage,
		Emitter:          emitter,
	})
	require.NoError(t, err)
	return service, emitter
}

var errBadStorage = errors.New("bad storage")

type badStorage struct{}

func (badStorage) GetVnetConfig(context.Context) (*vnet.VnetConfig, error) {
	return nil, trace.Wrap(errBadStorage)
}

func (badStorage) CreateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	return nil, trace.Wrap(errBadStorage)
}

func (badStorage) UpdateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	return nil, trace.Wrap(errBadStorage)
}

func (badStorage) UpsertVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	return nil, trace.Wrap(errBadStorage)
}

func (badStorage) DeleteVnetConfig(ctx context.Context) error {
	return trace.Wrap(errBadStorage)
}
