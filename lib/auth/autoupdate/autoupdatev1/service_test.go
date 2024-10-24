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

package autoupdatev1

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

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

// callMethod calls a method with given name in the DatabaseObjectService service
func callMethod(t *testing.T, service *Service, method string) error {
	for _, desc := range autoupdatev1pb.AutoUpdateService_ServiceDesc.Methods {
		if desc.MethodName == method {
			_, err := desc.Handler(service, context.Background(), func(_ any) error { return nil }, nil)
			return err
		}
	}
	require.FailNow(t, "method %v not found", method)
	panic("this line should never be reached: FailNow() should interrupt the test")
}

func TestServiceAccess(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		allowedVerbs     []string
		allowedStates    []authz.AdminActionAuthState
		disallowedStates []authz.AdminActionAuthState
		builtinRole      *authz.BuiltinRole
	}{
		{
			name: "CreateAutoUpdateConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbCreate},
		},
		{
			name: "UpdateAutoUpdateConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate},
		},
		{
			name: "UpsertAutoUpdateConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate, types.VerbCreate},
		},
		{
			name: "GetAutoUpdateConfig",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized,
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead},
		},
		{
			name:          "DeleteAutoUpdateConfig",
			allowedStates: []authz.AdminActionAuthState{authz.AdminActionAuthNotRequired, authz.AdminActionAuthMFAVerified},
			allowedVerbs:  []string{types.VerbDelete},
		},
		// AutoUpdate version check.
		{
			name: "CreateAutoUpdateVersion",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbCreate},
		},
		{
			name: "UpdateAutoUpdateVersion",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate},
		},
		{
			name: "UpsertAutoUpdateVersion",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate, types.VerbCreate},
		},
		{
			name: "GetAutoUpdateVersion",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized,
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead},
		},
		{
			name:          "DeleteAutoUpdateVersion",
			allowedStates: []authz.AdminActionAuthState{authz.AdminActionAuthNotRequired, authz.AdminActionAuthMFAVerified},
			allowedVerbs:  []string{types.VerbDelete},
		},
		// AutoUpdate agent rollout check.
		{
			name: "GetAutoUpdateAgentRollout",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized,
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead},
		},
		{
			name: "CreateAutoUpdateAgentRollout",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbCreate},
			builtinRole:  &authz.BuiltinRole{Role: types.RoleAuth},
		},
		{
			name: "UpdateAutoUpdateAgentRollout",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate},
			builtinRole:  &authz.BuiltinRole{Role: types.RoleAuth},
		},
		{
			name: "UpsertAutoUpdateAgentRollout",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbUpdate, types.VerbCreate},
			builtinRole:  &authz.BuiltinRole{Role: types.RoleAuth},
		},
		{
			name:          "DeleteAutoUpdateAgentRollout",
			allowedStates: []authz.AdminActionAuthState{authz.AdminActionAuthNotRequired, authz.AdminActionAuthMFAVerified},
			allowedVerbs:  []string{types.VerbDelete},
			builtinRole:   &authz.BuiltinRole{Role: types.RoleAuth},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// test the method with allowed admin states, each one separately.
			t.Run("allowed admin states", func(t *testing.T) {
				for _, state := range tt.allowedStates {
					t.Run(stateToString(state), func(t *testing.T) {
						for _, verbs := range utils.Combinations(tt.allowedVerbs) {
							t.Run(fmt.Sprintf("verbs=%v", verbs), func(t *testing.T) {
								service := newService(t, state, fakeChecker{allowedVerbs: verbs, builtinRole: tt.builtinRole}, &libevents.DiscardEmitter{})
								err := callMethod(t, service, tt.name)
								// expect access denied except with full set of verbs.
								if len(verbs) == len(tt.allowedVerbs) {
									require.False(t, trace.IsAccessDenied(err))
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
				disallowedStates := otherAdminStates(tt.allowedStates)
				if tt.disallowedStates != nil {
					disallowedStates = tt.disallowedStates
				}
				for _, state := range disallowedStates {
					t.Run(stateToString(state), func(t *testing.T) {
						// it is enough to test against tt.allowedVerbs,
						// this is the only different data point compared to the test cases above.
						service := newService(t, state, fakeChecker{allowedVerbs: tt.allowedVerbs, builtinRole: tt.builtinRole}, &libevents.DiscardEmitter{})
						err := callMethod(t, service, tt.name)
						require.True(t, trace.IsAccessDenied(err))
					})
				}
			})
		})
	}

	// verify that all declared methods have matching test cases
	t.Run("verify coverage", func(t *testing.T) {
		for _, method := range autoupdatev1pb.AutoUpdateService_ServiceDesc.Methods {
			t.Run(method.MethodName, func(t *testing.T) {
				match := false
				for _, testCase := range testCases {
					match = match || testCase.name == method.MethodName
				}
				require.True(t, match, "method %v without coverage, no matching tests", method.MethodName)
			})
		}
	})
}

func TestAutoUpdateConfigEvents(t *testing.T) {
	rwVerbs := []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
	mockEmitter := &eventstest.MockRecorderEmitter{}
	service := newService(t, authz.AdminActionAuthMFAVerified, fakeChecker{allowedVerbs: rwVerbs}, mockEmitter)
	ctx := context.Background()

	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Tools: &autoupdatev1pb.AutoUpdateConfigSpecTools{
			Mode: autoupdate.ToolsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)

	_, err = service.CreateAutoUpdateConfig(ctx, &autoupdatev1pb.CreateAutoUpdateConfigRequest{Config: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigCreateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigCreateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigCreate).Name)
	mockEmitter.Reset()

	_, err = service.UpdateAutoUpdateConfig(ctx, &autoupdatev1pb.UpdateAutoUpdateConfigRequest{Config: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigUpdate).Name)
	mockEmitter.Reset()

	_, err = service.UpsertAutoUpdateConfig(ctx, &autoupdatev1pb.UpsertAutoUpdateConfigRequest{Config: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigUpdate).Name)
	mockEmitter.Reset()

	_, err = service.DeleteAutoUpdateConfig(ctx, &autoupdatev1pb.DeleteAutoUpdateConfigRequest{})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateConfigDeleteEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateConfigDeleteCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateConfig, mockEmitter.LastEvent().(*apievents.AutoUpdateConfigDelete).Name)
	mockEmitter.Reset()
}

func TestAutoUpdateVersionEvents(t *testing.T) {
	rwVerbs := []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
	mockEmitter := &eventstest.MockRecorderEmitter{}
	service := newService(t, authz.AdminActionAuthMFAVerified, fakeChecker{allowedVerbs: rwVerbs}, mockEmitter)
	ctx := context.Background()

	config, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		Tools: &autoupdatev1pb.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
	})
	require.NoError(t, err)

	_, err = service.CreateAutoUpdateVersion(ctx, &autoupdatev1pb.CreateAutoUpdateVersionRequest{Version: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionCreateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionCreateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionCreate).Name)
	mockEmitter.Reset()

	_, err = service.UpdateAutoUpdateVersion(ctx, &autoupdatev1pb.UpdateAutoUpdateVersionRequest{Version: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionUpdate).Name)
	mockEmitter.Reset()

	_, err = service.UpsertAutoUpdateVersion(ctx, &autoupdatev1pb.UpsertAutoUpdateVersionRequest{Version: config})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionUpdateEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionUpdateCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionUpdate).Name)
	mockEmitter.Reset()

	_, err = service.DeleteAutoUpdateVersion(ctx, &autoupdatev1pb.DeleteAutoUpdateVersionRequest{})
	require.NoError(t, err)
	require.Len(t, mockEmitter.Events(), 1)
	require.Equal(t, libevents.AutoUpdateVersionDeleteEvent, mockEmitter.LastEvent().GetType())
	require.Equal(t, libevents.AutoUpdateVersionDeleteCode, mockEmitter.LastEvent().GetCode())
	require.Equal(t, types.MetaNameAutoUpdateVersion, mockEmitter.LastEvent().(*apievents.AutoUpdateVersionDelete).Name)
	mockEmitter.Reset()
}

type fakeChecker struct {
	allowedVerbs []string
	builtinRole  *authz.BuiltinRole
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindAutoUpdateConfig || resource == types.KindAutoUpdateVersion || resource == types.KindAutoUpdateAgentRollout {
		for _, allowedVerb := range f.allowedVerbs {
			if allowedVerb == verb {
				return nil
			}
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

func (f fakeChecker) HasRole(name string) bool {
	if f.builtinRole == nil {
		return false
	}
	return name == f.builtinRole.Role.String()
}

func (f fakeChecker) identityGetter() authz.IdentityGetter {
	if f.builtinRole != nil {
		return *f.builtinRole
	}
	return nil
}

func newService(t *testing.T, authState authz.AdminActionAuthState, checker fakeChecker, emitter apievents.Emitter) *Service {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	storage, err := local.NewAutoUpdateService(bk)
	require.NoError(t, err)

	return newServiceWithStorage(t, authState, checker, storage, emitter)
}

func newServiceWithStorage(t *testing.T, authState authz.AdminActionAuthState, checker fakeChecker, storage services.AutoUpdateService, emitter apievents.Emitter) *Service {
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
			Identity:             checker.identityGetter(),
		}, nil
	})

	service, err := NewService(ServiceConfig{
		Authorizer: authorizer,
		Backend:    storage,
		Cache:      storage,
		Emitter:    emitter,
	})
	require.NoError(t, err)
	return service
}
