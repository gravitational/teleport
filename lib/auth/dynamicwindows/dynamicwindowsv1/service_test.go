/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package dynamicwindowsv1

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	dynamicwindowsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dynamicwindows/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestServiceAccess(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		allowedVerbs  []string
		allowedStates []authz.AdminActionAuthState
	}{
		{
			name:          "CreateDynamicWindowsDesktop",
			allowedStates: []authz.AdminActionAuthState{authz.AdminActionAuthNotRequired, authz.AdminActionAuthMFAVerified},
			allowedVerbs:  []string{types.VerbCreate},
		},
		{
			name:          "UpdateDynamicWindowsDesktop",
			allowedStates: []authz.AdminActionAuthState{authz.AdminActionAuthNotRequired, authz.AdminActionAuthMFAVerified},
			allowedVerbs:  []string{types.VerbUpdate},
		},
		{
			name:          "DeleteDynamicWindowsDesktop",
			allowedStates: []authz.AdminActionAuthState{authz.AdminActionAuthNotRequired, authz.AdminActionAuthMFAVerified},
			allowedVerbs:  []string{types.VerbDelete},
		},
		{
			name: "ListDynamicWindowsDesktops",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized, authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified, authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead, types.VerbList},
		},
		{
			name: "GetDynamicWindowsDesktop",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized, authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified, authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead},
		},
	}

	for _, tt := range testCases {
		for _, state := range tt.allowedStates {
			for _, verbs := range utils.Combinations(tt.allowedVerbs) {
				t.Run(fmt.Sprintf("%v,allowed:%v,verbs:%v", tt.name, stateToString(state), verbs), func(t *testing.T) {
					service := newService(t, state, fakeChecker{allowedVerbs: verbs})
					err := callMethod(service, tt.name)
					// expect access denied except with full set of verbs.
					if len(verbs) == len(tt.allowedVerbs) {
						require.False(t, trace.IsAccessDenied(err))
					} else {
						require.Error(t, err)
						require.True(t, trace.IsAccessDenied(err), "expected access denied for verbs %v, got err=%v", verbs, err)
					}
				})
			}
		}

		disallowedStates := otherAdminStates(tt.allowedStates)
		for _, state := range disallowedStates {
			t.Run(fmt.Sprintf("%v,disallowed:%v", tt.name, stateToString(state)), func(t *testing.T) {
				// it is enough to test against tt.allowedVerbs,
				// this is the only different data point compared to the test cases above.
				service := newService(t, state, fakeChecker{allowedVerbs: tt.allowedVerbs})
				err := callMethod(service, tt.name)
				require.True(t, trace.IsAccessDenied(err))
			})
		}
	}

	// verify that all declared methods have matching test cases
	for _, method := range dynamicwindowsv1.DynamicWindowsService_ServiceDesc.Methods {
		t.Run(fmt.Sprintf("%v covered", method.MethodName), func(t *testing.T) {
			match := false
			for _, testCase := range testCases {
				match = match || testCase.name == method.MethodName
			}
			require.True(t, match, "method %v without coverage, no matching tests", method.MethodName)
		})
	}
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

// callMethod calls a method with given name in the DynamicWindowsDesktop service
func callMethod(service *Service, method string) error {
	for _, desc := range dynamicwindowsv1.DynamicWindowsService_ServiceDesc.Methods {
		if desc.MethodName == method {
			_, err := desc.Handler(service, context.Background(), func(arg any) error {
				switch arg := arg.(type) {
				case *dynamicwindowsv1.CreateDynamicWindowsDesktopRequest:
					arg.Desktop, _ = types.NewDynamicWindowsDesktopV1("test", nil, types.DynamicWindowsDesktopSpecV1{
						Addr: "test",
					})
				case *dynamicwindowsv1.UpdateDynamicWindowsDesktopRequest:
					arg.Desktop, _ = types.NewDynamicWindowsDesktopV1("test", nil, types.DynamicWindowsDesktopSpecV1{
						Addr: "test",
					})
				}
				return nil
			}, nil)
			return err
		}
	}
	return fmt.Errorf("method %v not found", method)
}

type fakeChecker struct {
	allowedVerbs []string
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindDynamicWindowsDesktop {
		if slices.Contains(f.allowedVerbs, verb) {
			return nil
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

func newService(t *testing.T, authState authz.AdminActionAuthState, checker services.AccessChecker) *Service {
	t.Helper()

	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	backendService, err := local.NewDynamicWindowsDesktopService(b)
	require.NoError(t, err)

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("probakowski")
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:                 user,
			Checker:              checker,
			AdminActionAuthState: authState,
			Identity: authz.LocalUser{
				Identity: tlsca.Identity{
					Username: user.GetName(),
				},
			},
		}, nil
	})

	service, err := NewService(ServiceConfig{
		Authorizer: authorizer,
		Backend:    backendService,
		Cache:      backendService,
	})
	require.NoError(t, err)
	return service
}
