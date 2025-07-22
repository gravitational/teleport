package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
)

var userMonitorCmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	cmpopts.IgnoreFields(types.Metadata{}, "Revision"),

	// TODO: remove labels exclusion after teleport#44430 merges
	cmpopts.IgnoreFields(header.Metadata{}, "Labels"),
	cmpopts.IgnoreFields(types.Metadata{}, "Labels"),

	cmpopts.SortSlices(func(u1, u2 *userloginstate.UserLoginState) bool {
		return u1.GetName() < u2.GetName()
	}),
	cmpopts.SortSlices(func(s1, s2 string) bool {
		return s1 < s2
	}),
}

func TestReconcile(t *testing.T) {
	tests := []struct {
		name           string
		roles          []types.Role
		users          []types.User
		states         []*userloginstate.UserLoginState
		locks          []types.Lock
		expectedStates []*userloginstate.UserLoginState
		expectedLocks  map[string]types.LockTarget
	}{
		{
			name:           "no resource",
			expectedStates: []*userloginstate.UserLoginState{},
			expectedLocks:  map[string]types.LockTarget{},
		},
		{
			name: "only users, no user login states",
			roles: []types.Role{
				newRole(t, "role1"),
				newRole(t, "role2"),
			},
			users: []types.User{
				newUser(t, "user1", types.UserTypeSSO, "role1", "role2"),
				newUser(t, "user2", types.UserTypeSSO, "role1"),
			},
			locks: []types.Lock{
				newLock(t, "lock1", types.LockTarget{User: "some-user1"}),
				newLock(t, "lock2", types.LockTarget{User: "some-user2"}),
				newLock(t, "lock3", types.LockTarget{Login: "some-login1"}),
				newLock(t, "lock4", types.LockTarget{Role: "some-role1"}),
				newLock(t, "lock5", types.LockTarget{AccessRequest: "some-access-request-1"}),
			},
			expectedStates: []*userloginstate.UserLoginState{
				newUserLoginState(t, "user1", []string{"role1", "role2"}, []string{"role1", "role2"}, types.UserTypeSSO),
				newUserLoginState(t, "user2", []string{"role1"}, []string{"role1"}, types.UserTypeSSO),
			},
			expectedLocks: map[string]types.LockTarget{
				"lock1": {User: "some-user1"},
				"lock2": {User: "some-user2"},
				"lock4": {Role: "some-role1"},
				"lock5": {AccessRequest: "some-access-request-1"},
			},
		},
		{
			name: "user login states ignored",
			roles: []types.Role{
				newRole(t, "role1"),
				newRole(t, "role2"),
			},
			users: []types.User{
				newUser(t, "user1", types.UserTypeSSO, "role1", "role2"),
				newUser(t, "user2", types.UserTypeSSO, "role1"),
			},
			states: []*userloginstate.UserLoginState{
				newUserLoginState(t, "user1", []string{}, []string{"role1", "role2"}, types.UserTypeSSO),
				newUserLoginState(t, "user2", []string{}, []string{"role1", "role2"}, types.UserTypeSSO),
			},
			expectedStates: []*userloginstate.UserLoginState{
				newUserLoginState(t, "user1", []string{"role1", "role2"}, []string{"role1", "role2"}, types.UserTypeSSO),
				newUserLoginState(t, "user2", []string{"role1"}, []string{"role1"}, types.UserTypeSSO),
			},
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "users rebuilt from user login states",
			roles: []types.Role{
				newRole(t, "role1"),
				newRole(t, "role2"),
			},
			states: []*userloginstate.UserLoginState{
				newUserLoginState(t, "user1", []string{"role1", "role2"}, []string{"role1", "role2", "role3", "role4"}, types.UserTypeSSO),
				newUserLoginState(t, "user2", []string{"role1"}, []string{"role1", "role2", "role3", "role4"}, types.UserTypeSSO),
			},
			expectedStates: []*userloginstate.UserLoginState{
				newUserLoginState(t, "user1", []string{"role1", "role2"}, []string{"role1", "role2"}, types.UserTypeSSO),
				newUserLoginState(t, "user2", []string{"role1"}, []string{"role1"}, types.UserTypeSSO),
			},
			expectedLocks: map[string]types.LockTarget{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			svc := newUserMonitorService(t)

			ctx := context.Background()

			for _, role := range test.roles {
				_, err := svc.authServer.CreateRole(ctx, role)
				require.NoError(t, err)
			}

			for _, user := range test.users {
				_, err := svc.authServer.UpsertUser(ctx, user)
				require.NoError(t, err)
			}

			for _, state := range test.states {
				_, err := svc.authServer.UserLoginStates.UpsertUserLoginState(ctx, state)
				require.NoError(t, err)
			}

			for _, lock := range test.locks {
				require.NoError(t, svc.authServer.UpsertLock(ctx, lock))
			}

			require.NoError(t, svc.reconcile(ctx))

			states, err := svc.authServer.GetUserLoginStates(ctx)
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(test.expectedStates, states, userMonitorCmpOpts...))
			require.Empty(t, cmp.Diff(test.expectedLocks, svc.lockToTarget))
		})
	}
}

func TestProcessEvent(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	const userName = "test"

	type updateFn func(*testing.T, *auth.Server) types.Event
	tests := []struct {
		name                          string
		setup                         func(*testing.T, *auth.Server)
		updates                       []updateFn
		errAssert                     require.ErrorAssertionFunc
		expected                      *userloginstate.UserLoginState
		expectedLocks                 map[string]types.LockTarget
		expectedOktaAssignmentTargets []types.OktaAssignmentTarget
	}{
		{
			name: "no resource",
			errAssert: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("resource is empty"))
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					return types.Event{}
				},
			},
		},
		{
			name: "bad op",
			errAssert: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("only modification operations are supported"))
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					return types.Event{
						Resource: newAccessList(t, "access-list1", []string{"role1"}),
						Type:     types.OpGet,
					}
				},
			},
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "user changes",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3", "role4")

				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role2", "role3"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					// User should gain access to role4
					user := newUser(t, userName, types.UserTypeSSO, "role1", "role4")
					return types.Event{
						Resource: user,
						Type:     types.OpPut,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1", "role4"}, []string{"role1", "role2", "role3", "role4"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "user locked",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3", "role4")

				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role2", "role3"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
			},
			updates: []updateFn{
				func(t *testing.T, as *auth.Server) types.Event {
					// User should be excluded from access lists.
					lock := addLock(t, as, userName, types.LockTarget{
						User: userName,
					})
					return types.Event{
						Resource: lock,
						Type:     types.OpPut,
					}
				},
			},
			errAssert: require.NoError,
			expected:  newUserLoginState(t, userName, []string{"role1"}, []string{"role1"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{
				userName: {User: userName},
			},
		},
		{
			name: "user unlocked",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3")

				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role2", "role3"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
			},
			updates: []updateFn{
				func(t *testing.T, as *auth.Server) types.Event {
					// User should be excluded from access lists.
					lock := addLock(t, as, userName, types.LockTarget{
						User: userName,
					})
					return types.Event{
						Resource: lock,
						Type:     types.OpPut,
					}
				},
				func(t *testing.T, as *auth.Server) types.Event {
					// User should now be accepted back into access lists.
					require.NoError(t, as.DeleteLock(context.Background(), userName))
					return types.Event{
						Resource: &types.ResourceHeader{
							Metadata: types.Metadata{
								Name: userName,
							},
							Kind: types.KindLock,
						},
						Type: types.OpDelete,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "user deleted (local)",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeLocal, "role1")
				addRoles(t, as, "role2", "role3", "role4")

				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role2", "role3"}, types.UserTypeLocal)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					// User should rebuilt without any roles or traits set.
					require.NoError(t, s.DeleteUser(context.Background(), userName))
					user := newUser(t, userName, types.UserTypeSSO, "role1", "role4")
					return types.Event{
						Resource: user,
						Type:     types.OpDelete,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, nil, []string{"role2", "role3"}, types.UserTypeLocal),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "user deleted (sso)",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3", "role4")

				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role2", "role3"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					// User should be rebuilt based on the user login state here.
					require.NoError(t, s.DeleteUser(context.Background(), userName))
					user := newUser(t, userName, types.UserTypeSSO, "role1", "role4")
					return types.Event{
						Resource: user,
						Type:     types.OpDelete,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "role changes",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3")

				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)

				// This access list should be ignored.
				addAccessList(t, as, "access-list2", []string{"role3"}, userName)

				// User should get access to this user group via the access list.
				userGroup, err := types.NewUserGroup(types.Metadata{
					Name: "ug1",
					Labels: map[string]string{
						types.OriginLabel: types.OriginOkta,
					},
				}, types.UserGroupSpecV1{})
				require.NoError(t, err)
				require.NoError(t, as.CreateUserGroup(context.Background(), userGroup))
			},
			updates: []updateFn{
				func(t *testing.T, as *auth.Server) types.Event {
					role, err := types.NewRole("role2", types.RoleSpecV6{
						Allow: types.RoleConditions{
							GroupLabels: types.Labels{
								types.Wildcard: []string{types.Wildcard},
							},
							Rules: []types.Rule{
								{
									Resources: []string{types.KindUserGroup},
									Verbs:     []string{types.VerbRead, types.VerbList},
								},
							},
						},
					})
					require.NoError(t, err)
					_, err = as.UpsertRole(context.Background(), role)
					require.NoError(t, err)
					return types.Event{
						Resource: role,
						Type:     types.OpPut,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
			expectedOktaAssignmentTargets: []types.OktaAssignmentTarget{
				&types.OktaAssignmentTargetV1{
					Id:   "ug1",
					Type: types.OktaAssignmentTargetV1_GROUP,
				},
			},
		},
		{
			name: "access list membership changes (user not found)",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3", "role4")

				// User should lose access to role4
				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role1", "role2", "role3", "role4"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
				addAccessList(t, as, "access-list2", []string{"role4"})
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					return types.Event{
						Resource: newAccessListMember(t, "access-list2", userName),
						Type:     types.OpDelete,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "access list membership changes (only user login state)",
			setup: func(t *testing.T, as *auth.Server) {
				addRoles(t, as, "role1", "role2", "role3", "role4")
				// User should lose access to role4
				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role1", "role2", "role3", "role4"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
				addAccessList(t, as, "access-list2", []string{"role4"})
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					return types.Event{
						Resource: newAccessListMember(t, "access-list2", userName),
						Type:     types.OpDelete,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "access list itself changes",
			setup: func(t *testing.T, as *auth.Server) {
				addRoles(t, as, "role1", "role2", "role3", "role4")
				// User should gain access to role4
				addUserLoginState(t, as, userName, []string{"role1"}, []string{"role1", "role2", "role3"}, types.UserTypeSSO)

				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, userName)
			},
			updates: []updateFn{
				func(t *testing.T, as *auth.Server) types.Event {
					accessList := newAccessList(t, "access-list1", []string{"role2", "role3", "role4"})
					_, err := as.AccessLists.UpsertAccessList(context.Background(), accessList)
					require.NoError(t, err)
					return types.Event{
						Resource: accessList,
						Type:     types.OpPut,
					}
				},
			},
			errAssert:     require.NoError,
			expected:      newUserLoginState(t, userName, []string{"role1"}, []string{"role1", "role2", "role3", "role4"}, types.UserTypeSSO),
			expectedLocks: map[string]types.LockTarget{},
		},
		{
			name: "access list membership changes with a member whose account does not exist in Teleport",
			setup: func(t *testing.T, as *auth.Server) {
				addRoles(t, as, "role2", "role3")
				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, "non-existent-user")
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					return types.Event{
						Resource: newAccessListMember(t, "access-list1", "non-existent-user"),
						Type:     types.OpDelete,
					}
				},
			},
			errAssert: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `"non-existent-user" doesn't exist`)
			},
		},
		{
			name: "access list membership changes with a member whose account does not exist in Teleport but has OriginAWSIdentityCenter label",
			setup: func(t *testing.T, as *auth.Server) {
				addUser(t, as, userName, types.UserTypeSSO, "role1")
				addRoles(t, as, "role2", "role3")
				addAccessList(t, as, "access-list1", []string{"role2", "role3"}, externalMemberWithOriginAWSIdentityCenter)
			},
			updates: []updateFn{
				func(t *testing.T, s *auth.Server) types.Event {
					return types.Event{
						Resource: newAccessListMember(t, "access-list2", externalMemberWithOriginAWSIdentityCenter),
						Type:     types.OpDelete,
					}
				},
			},
			errAssert: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			svc := newUserMonitorService(t)

			// Only add the Okta user assignment creator if we're testing against Okta assignment targets.
			if test.expectedOktaAssignmentTargets != nil {
				setupOktaUAC(t, svc)
			}

			if test.setup != nil {
				test.setup(t, svc.authServer)
			}

			ctx := context.Background()

			assignments, _, err := svc.authServer.ListOktaAssignments(ctx, 0 /* default page size */, "")
			require.NoError(t, err)
			require.Empty(t, assignments)

			// Process each update.
			var processErrs []error
			for _, update := range test.updates {
				event := update(t, svc.authServer)
				err = svc.processResource(ctx, event.Resource, event.Type)
				if err != nil {
					processErrs = append(processErrs, err)
				}
			}
			test.errAssert(t, trace.NewAggregate(processErrs...))

			if err != nil {
				return
			}
			if test.expected == nil {
				return
			}

			uls, err := svc.authServer.GetUserLoginState(ctx, userName)
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(test.expected, uls, userMonitorCmpOpts...))

			assignments, _, err = svc.authServer.ListOktaAssignments(ctx, 0 /* default page size */, "")
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(test.expectedLocks, svc.lockToTarget, userMonitorCmpOpts...))

			if test.expectedOktaAssignmentTargets != nil {
				require.Len(t, assignments, 1)
				require.Empty(t, cmp.Diff(test.expectedOktaAssignmentTargets, assignments[0].GetTargets()))
			} else {
				require.Empty(t, assignments)
			}
		})
	}
}

func setupOktaUAC(t *testing.T, svc *UserMonitor) {
	t.Helper()

	clusterName, err := svc.authServer.GetClusterName(context.TODO())
	require.NoError(t, err)

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Create an Okta plugin so the Okta UAC will run..
	plugins := local.NewPluginsService(mem)
	oktaPlugin := types.NewPluginV1(types.Metadata{
		Name: "okta",
	}, types.PluginSpecV1{
		Settings: &types.PluginSpecV1_Okta{
			Okta: &types.PluginOktaSettings{
				OrgUrl: "https://www.okta.com",
			},
		},
	}, &types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: &types.PluginStaticCredentialsRef{
				Labels: map[string]string{"dummy": "dummy"},
			},
		},
	})
	require.NoError(t, plugins.CreatePlugin(context.Background(), oktaPlugin))

	connected, err := connected.New(connected.Config{
		DisableCache:    true,
		ConnectedGetter: svc.authServer,
		Plugins:         plugins,
	})
	require.NoError(t, err)

	uac, err := okta.NewUserAssignmentCreator(okta.UserAssignmentCreatorConfig{
		ClusterName:          clusterName.GetClusterName(),
		AccessPoint:          svc.authServer,
		OktaConnected:        connected,
		UnifiedResourceCache: svc.authServer.UnifiedResourceCache,
	})
	require.NoError(t, err)
	svc.authServer.RegisterLoginHook(uac.OnLogin)
}

func newUserMonitorService(t *testing.T) *UserMonitor {
	t.Helper()

	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	svc, err := NewUserMonitor(UserMonitorConfig{
		AuthServer: as.AuthServer,
		Events:     as.AuthServer,
	})
	require.NoError(t, err)

	return svc
}

func addUser(t *testing.T, as *auth.Server, name string, userType types.UserType, roles ...string) {
	t.Helper()

	addRoles(t, as, roles...)

	user := newUser(t, name, userType, roles...)
	_, err := as.UpsertUser(context.Background(), user)
	require.NoError(t, err)
}

func newUser(t *testing.T, name string, userType types.UserType, roles ...string) types.User {
	t.Helper()

	user, err := types.NewUser(name)
	require.NoError(t, err)

	if userType == types.UserTypeSSO {
		user.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type: "dummy",
			},
		})
	}

	for _, role := range roles {
		user.AddRole(role)
	}

	return user
}

func addAccessList(t *testing.T, as *auth.Server, name string, roleGrants []string, members ...string) {
	t.Helper()

	_, err := as.AccessLists.UpsertAccessList(context.Background(), newAccessList(t, name, roleGrants))
	require.NoError(t, err)

	for _, member := range members {
		addAccessListMember(t, as, name, member)
	}
}

func newAccessList(t *testing.T, name string, roleGrants []string) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title: "title",
			Owners: []accesslist.Owner{
				{
					Name: "test-user1",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Now().Add(365 * 24 * time.Hour),
			},
			MembershipRequires: accesslist.Requires{},
			OwnershipRequires:  accesslist.Requires{},
			Grants: accesslist.Grants{
				Roles: roleGrants,
			},
		},
	)
	require.NoError(t, err)

	return accessList
}

func addAccessListMember(t *testing.T, as *auth.Server, accessList, name string) {
	t.Helper()

	_, err := as.AccessLists.UpsertAccessListMember(context.Background(), newAccessListMember(t, accessList, name))
	require.NoError(t, err)
}

func newAccessListMember(t *testing.T, accessList, name string) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: name,
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       name,
			Joined:     time.Now(),
			Expires:    time.Now().Add(time.Hour * 24),
			Reason:     "a reason",
			AddedBy:    "dummy",
		},
	)
	require.NoError(t, err)

	if name == externalMemberWithOriginAWSIdentityCenter {
		member.SetOrigin(common.OriginAWSIdentityCenter)
	}

	return member
}

const externalMemberWithOriginAWSIdentityCenter = "external-user1"

func addUserLoginState(t *testing.T, as *auth.Server, name string, originalRoles, roles []string, userType types.UserType) {
	t.Helper()

	_, err := as.UserLoginStates.UpsertUserLoginState(context.Background(), newUserLoginState(t, name, originalRoles, roles, userType))
	require.NoError(t, err)
}

func newUserLoginState(t *testing.T, name string, originalRoles, roles []string, userType types.UserType) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(header.Metadata{
		Name: name,
	}, userloginstate.Spec{
		OriginalRoles: originalRoles,
		Roles:         roles,
		UserType:      userType,
	})
	require.NoError(t, err)

	return uls
}

func addRoles(t *testing.T, as *auth.Server, roleNames ...string) {
	t.Helper()

	ctx := context.Background()
	for _, roleName := range roleNames {
		_, err := as.UpsertRole(ctx, newRole(t, roleName))
		require.NoError(t, err)
	}
}

func newRole(t *testing.T, roleName string) types.Role {
	t.Helper()

	role, err := types.NewRole(roleName, types.RoleSpecV6{})
	require.NoError(t, err)

	return role
}

func newLock(t *testing.T, name string, target types.LockTarget) types.Lock {
	t.Helper()

	lock, err := types.NewLock(name, types.LockSpecV2{
		Target: target,
	})
	require.NoError(t, err)

	return lock
}

func addLock(t *testing.T, as *auth.Server, name string, target types.LockTarget) types.Lock {
	t.Helper()

	lock := newLock(t, name, target)

	require.NoError(t, as.UpsertLock(context.Background(), lock))

	return lock
}
