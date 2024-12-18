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
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
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
								service := newService(t, state, fakeChecker{allowedVerbs: verbs})
								err := tc.action(service)
								// expect access denied except with full set of verbs.
								if len(verbs) == len(tc.allowedVerbs) {
									require.NoError(t, err, trace.DebugReport(err))
								} else {
									require.True(t, trace.IsAccessDenied(err), "expected access denied for verbs %v, got err=%v", verbs, err)
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
						service := newService(t, state, fakeChecker{allowedVerbs: tc.allowedVerbs})
						err := tc.action(service)
						require.True(t, trace.IsAccessDenied(err))
					})
				}
			})

			// test the method with storage-layer errors
			t.Run("storage error", func(t *testing.T) {
				service := newServiceWithStorage(t, tc.allowedStates[0],
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

func newService(t *testing.T, authState authz.AdminActionAuthState, checker services.AccessChecker) *Service {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	storage, err := local.NewVnetConfigService(bk)
	require.NoError(t, err)

	return newServiceWithStorage(t, authState, checker, storage)
}

func newServiceWithStorage(t *testing.T, authState authz.AdminActionAuthState, checker services.AccessChecker, storage services.VnetConfigService) *Service {
	t.Helper()

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("alice")
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:                 user,
			Checker:              checker,
			AdminActionAuthState: authState,
		}, nil
	})

	return NewService(storage, authorizer)
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
