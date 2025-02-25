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

package machineidv1

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TestBotInstanceServiceAccess ensures RBAC an admin state rules are applied properly
func TestBotInstanceServiceAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		allowedVerbs  []string
		allowedStates []authz.AdminActionAuthState
		skip          bool
	}{
		{
			name: "GetBotInstance",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized, authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified, authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead},
		},
		{
			name: "ListBotInstances",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthUnauthorized, authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified, authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbRead, types.VerbList},
		},
		{
			name: "DeleteBotInstance",
			allowedStates: []authz.AdminActionAuthState{
				authz.AdminActionAuthNotRequired,
				authz.AdminActionAuthMFAVerified,
				authz.AdminActionAuthMFAVerifiedWithReuse,
			},
			allowedVerbs: []string{types.VerbDelete},
		},
		{
			name: "SubmitHeartbeat",

			// SubmitHeartbeat has its own authz and does not follow normal RBAC rules
			skip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test the method with allowed admin states, each one separately.
			t.Run("allowed admin states", func(t *testing.T) {
				for _, state := range tt.allowedStates {
					t.Run(stateToString(state), func(t *testing.T) {
						for _, verbs := range utils.Combinations(tt.allowedVerbs) {
							t.Run(fmt.Sprintf("verbs=%v", verbs), func(t *testing.T) {
								backend := newBotInstanceBackend(t)
								service := newBotInstanceService(t, backend, state, fakeChecker{allowedVerbs: verbs})
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
				if tt.skip {
					t.Skipf("method %+v is excluded from tests", tt.name)
				}

				disallowedStates := otherAdminStates(tt.allowedStates)
				for _, state := range disallowedStates {
					t.Run(stateToString(state), func(t *testing.T) {
						// it is enough to test against tt.allowedVerbs,
						// this is the only different data point compared to the test cases above.
						backend := newBotInstanceBackend(t)
						service := newBotInstanceService(t, backend, state, fakeChecker{allowedVerbs: tt.allowedVerbs})
						err := callMethod(t, service, tt.name)
						require.True(t, trace.IsAccessDenied(err))
					})
				}
			})
		})
	}

	// verify that all declared methods have matching test cases
	t.Run("verify coverage", func(t *testing.T) {
		for _, method := range machineidv1.BotInstanceService_ServiceDesc.Methods {
			t.Run(method.MethodName, func(t *testing.T) {
				match := false
				for _, testCase := range tests {
					match = match || testCase.name == method.MethodName
				}
				require.True(t, match, "method %v without coverage, no matching tests", method.MethodName)
			})
		}
	})
}

// TestBotInstanceServiceReadDelete tests read and delete functionality exposed
// by the BotInstanceService.
func TestBotInstanceServiceReadDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	backend := newBotInstanceBackend(t)
	idsFoo := createInstances(t, ctx, backend, "foo", 3)
	idsBar := createInstances(t, ctx, backend, "bar", 3)

	idsAll := map[string]struct{}{}
	for i := range idsFoo {
		idsAll[i] = struct{}{}
	}
	for i := range idsBar {
		idsAll[i] = struct{}{}
	}

	// Make a service with all useful permissions that doesn't require admin auth
	checker := fakeChecker{allowedVerbs: []string{types.VerbRead, types.VerbList, types.VerbDelete}}
	service := newBotInstanceService(t, backend, authz.AdminActionAuthNotRequired, checker)

	// Make sure we can get all foo instances
	for id := range idsFoo {
		ins, err := service.GetBotInstance(ctx, &machineidv1.GetBotInstanceRequest{
			BotName:    "foo",
			InstanceId: id,
		})
		require.NoError(t, err)

		require.Equal(t, "foo", ins.Spec.BotName)
		require.Equal(t, id, ins.Spec.InstanceId)
	}

	// Make sure we can get all bar instances
	for id := range idsBar {
		ins, err := service.GetBotInstance(ctx, &machineidv1.GetBotInstanceRequest{
			BotName:    "bar",
			InstanceId: id,
		})
		require.NoError(t, err)

		require.Equal(t, "bar", ins.Spec.BotName)
		require.Equal(t, id, ins.Spec.InstanceId)
	}

	// List should work
	fooInstances := listInstances(t, ctx, service, "foo")
	require.Len(t, fooInstances, 3)
	for _, bi := range fooInstances {
		require.Contains(t, idsFoo, bi.Spec.InstanceId)
	}

	barInstances := listInstances(t, ctx, service, "bar")
	require.Len(t, barInstances, 3)
	for _, bi := range barInstances {
		require.Contains(t, idsBar, bi.Spec.InstanceId)
	}

	allInstances := listInstances(t, ctx, service, "")
	require.Len(t, allInstances, 6)
	for _, bi := range allInstances {
		require.Contains(t, idsAll, bi.Spec.InstanceId)
	}

	// Attempt to delete everything
	for id := range idsFoo {
		_, err := service.DeleteBotInstance(ctx, &machineidv1.DeleteBotInstanceRequest{
			BotName:    "foo",
			InstanceId: id,
		})
		require.NoError(t, err)
	}

	for id := range idsBar {
		_, err := service.DeleteBotInstance(ctx, &machineidv1.DeleteBotInstanceRequest{
			BotName:    "bar",
			InstanceId: id,
		})
		require.NoError(t, err)
	}

	allInstances = listInstances(t, ctx, service, "")
	require.Empty(t, allInstances)
}

type identityGetterFn func() tlsca.Identity

func (f identityGetterFn) GetIdentity() tlsca.Identity {
	return f()
}

func TestBotInstanceServiceSubmitHeartbeat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const botName = "test-bot"
	const botInstanceID = "123-456"

	goodIdentity := tlsca.Identity{
		BotName:       botName,
		BotInstanceID: botInstanceID,
	}

	tests := []struct {
		name              string
		identity          tlsca.Identity
		req               *machineidv1.SubmitHeartbeatRequest
		createBotInstance bool
		assertErr         assert.ErrorAssertionFunc
		wantHeartbeat     bool
	}{
		{
			name:              "success",
			createBotInstance: true,
			req: &machineidv1.SubmitHeartbeatRequest{
				Heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
					Hostname: "llama",
				},
			},
			identity:      goodIdentity,
			assertErr:     assert.NoError,
			wantHeartbeat: true,
		},
		{
			name:              "missing bot name",
			createBotInstance: true,
			req: &machineidv1.SubmitHeartbeatRequest{
				Heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
					Hostname: "llama",
				},
			},
			identity: tlsca.Identity{
				BotInstanceID: botInstanceID,
			},
			assertErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsAccessDenied(err)) && assert.Contains(t, err.Error(), "identity did not contain bot name")
			},
			wantHeartbeat: false,
		},
		{
			name:              "missing instance id",
			createBotInstance: true,
			req: &machineidv1.SubmitHeartbeatRequest{
				Heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
					Hostname: "llama",
				},
			},
			identity: tlsca.Identity{
				BotName: botName,
			},
			assertErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsAccessDenied(err)) && assert.Contains(t, err.Error(), "identity did not contain bot instance")
			},
			wantHeartbeat: false,
		},
		{
			name:              "bot instance does not exist",
			createBotInstance: false,
			req: &machineidv1.SubmitHeartbeatRequest{
				Heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
					Hostname: "llama",
				},
			},
			identity: goodIdentity,
			assertErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:              "missing heartbeat",
			createBotInstance: true,
			req: &machineidv1.SubmitHeartbeatRequest{
				Heartbeat: nil,
			},
			identity: goodIdentity,
			assertErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.True(t, trace.IsBadParameter(err)) && assert.Contains(t, err.Error(), "heartbeat: must be non-nil")
			},
			wantHeartbeat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newBotInstanceBackend(t)
			service, err := NewBotInstanceService(BotInstanceServiceConfig{
				Backend: backend,
				Authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
					return &authz.Context{
						Identity: identityGetterFn(func() tlsca.Identity {
							return tt.identity
						}),
					}, nil
				}),
			})
			require.NoError(t, err)

			if tt.createBotInstance {
				bi := newBotInstance(botName)
				bi.Spec.InstanceId = botInstanceID
				_, err := backend.CreateBotInstance(ctx, bi)
				require.NoError(t, err)
			}

			_, err = service.SubmitHeartbeat(ctx, tt.req)
			tt.assertErr(t, err)
			if tt.createBotInstance {
				bi, err := backend.GetBotInstance(ctx, botName, botInstanceID)
				require.NoError(t, err)
				if tt.wantHeartbeat {
					assert.Empty(
						t,
						cmp.Diff(
							bi.Status.InitialHeartbeat,
							tt.req.Heartbeat,
							protocmp.Transform()),
					)
					assert.Len(t, bi.Status.LatestHeartbeats, 1)
					assert.Empty(
						t,
						cmp.Diff(
							bi.Status.LatestHeartbeats[0],
							tt.req.Heartbeat,
							protocmp.Transform()),
					)
				} else {
					assert.Nil(t, bi.Status.InitialHeartbeat)
					assert.Empty(t, bi.Status.LatestHeartbeats)
				}
			}
		})
	}
}

func TestBotInstanceServiceSubmitHeartbeat_HeartbeatLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const botName = "test-bot"
	const botInstanceID = "123-456"

	backend := newBotInstanceBackend(t)
	service, err := NewBotInstanceService(BotInstanceServiceConfig{
		Backend: backend,
		Authorizer: authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
			return &authz.Context{
				Identity: identityGetterFn(func() tlsca.Identity {
					return tlsca.Identity{
						BotName:       botName,
						BotInstanceID: botInstanceID,
					}
				}),
			}, nil
		}),
	})
	require.NoError(t, err)

	bi := newBotInstance(botName)
	bi.Spec.InstanceId = botInstanceID
	_, err = backend.CreateBotInstance(ctx, bi)
	require.NoError(t, err)

	extraHeartbeats := 5
	for i := 0; i < (heartbeatHistoryLimit + extraHeartbeats); i++ {
		_, err = service.SubmitHeartbeat(ctx, &machineidv1.SubmitHeartbeatRequest{
			Heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				Hostname: strconv.Itoa(i),
			},
		})
		require.NoError(t, err)
	}

	bi, err = backend.GetBotInstance(ctx, botName, botInstanceID)
	require.NoError(t, err)
	assert.Len(t, bi.Status.LatestHeartbeats, heartbeatHistoryLimit)
	assert.Equal(t, "0", bi.Status.InitialHeartbeat.Hostname)
	// Ensure we have the last 10 heartbeats
	for i := 0; i < heartbeatHistoryLimit; i++ {
		wantHostname := strconv.Itoa(i + extraHeartbeats)
		assert.Equal(t, wantHostname, bi.Status.LatestHeartbeats[i].Hostname)
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

type fakeChecker struct {
	allowedVerbs []string
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindBotInstance {
		for _, allowedVerb := range f.allowedVerbs {
			if allowedVerb == verb {
				return nil
			}
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

// callMethod calls a method with given name in the BotInstanceService
func callMethod(t *testing.T, service *BotInstanceService, method string) error {
	for _, desc := range machineidv1.BotInstanceService_ServiceDesc.Methods {
		if desc.MethodName == method {
			_, err := desc.Handler(service, context.Background(), func(_ any) error { return nil }, nil)
			return err
		}
	}
	require.FailNow(t, "method %v not found", method)
	panic("this line should never be reached: FailNow() should interrupt the test")
}

// newBotInstance creates a new bot instance for the named bot with a random ID
func newBotInstance(botName string) *machineidv1.BotInstance {
	id := uuid.New()

	bi := &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    botName,
			InstanceId: id.String(),
		},
		Status: &machineidv1.BotInstanceStatus{},
	}

	return bi
}

// createInstances creates and inserts many random bot instances for the named bot
func createInstances(t *testing.T, ctx context.Context, backend *local.BotInstanceService, botName string, count int) map[string]struct{} {
	t.Helper()

	ids := map[string]struct{}{}

	for i := 0; i < count; i++ {
		bi := newBotInstance(botName)
		_, err := backend.CreateBotInstance(ctx, bi)
		require.NoError(t, err)

		ids[bi.Spec.InstanceId] = struct{}{}
	}

	return ids
}

// listInstances lists all instances for the named bot (if any)
func listInstances(t *testing.T, ctx context.Context, service *BotInstanceService, botName string) []*machineidv1.BotInstance {
	t.Helper()

	var resources []*machineidv1.BotInstance
	var nextKey string

	for {
		res, err := service.ListBotInstances(ctx, &machineidv1.ListBotInstancesRequest{
			FilterBotName: botName,
			PageSize:      0,
			PageToken:     nextKey,
		})
		require.NoError(t, err)

		resources = append(resources, res.BotInstances...)

		nextKey = res.NextPageToken
		if nextKey == "" {
			break
		}
	}

	return resources
}

// newBotInstanceBackend creates a new local backend for BotInstance CRUD
// operations.
func newBotInstanceBackend(t *testing.T) *local.BotInstanceService {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	backendService, err := local.NewBotInstanceService(b, clockwork.NewFakeClock())
	require.NoError(t, err)

	return backendService
}

// newBotInstanceService creates a gRPC bot instance service for testing
func newBotInstanceService(
	t *testing.T,
	backendService *local.BotInstanceService,
	authState authz.AdminActionAuthState,
	checker services.AccessChecker,
) *BotInstanceService {
	t.Helper()

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("example")
		if err != nil {
			return nil, err
		}

		return &authz.Context{
			User:                 user,
			Checker:              checker,
			AdminActionAuthState: authState,
		}, nil
	})

	service, err := NewBotInstanceService(BotInstanceServiceConfig{
		Authorizer: authorizer,
		Backend:    backendService,
	})
	require.NoError(t, err)

	return service
}
