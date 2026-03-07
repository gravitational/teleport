package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreateAccessRequest_RoleBased(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}
	var createdReq types.AccessRequest
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{createdReq}, nil
	}

	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		createdReq = req
		require.Equal(t, "userFoo", req.GetUser())
		require.Equal(t, []string{"*"}, req.GetRoles())
		require.Equal(t, "some reason", req.GetRequestReason())
		return nil
	}

	request := ui.AccessRequestParameters{
		Reason: "some reason",
	}

	// Test with empty role requests, wild card is used.
	assumeStartTime := time.Now().UTC().Add(1 * time.Hour)
	request.AssumeStartTime = &assumeStartTime
	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.NoError(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, types.RequestState_PENDING.String(), req.State)
	require.Equal(t, assumeStartTime, *req.AssumeStartTime)

	// Test with specific roles requested.
	request.Roles = []string{"role1", "role2"}
	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		require.ElementsMatch(t, req.GetRoles(), []string{"role1", "role2"})
		return nil
	}

	_, err = createAccessRequest(context.Background(), m, request, "userFoo")
	require.NoError(t, err)
}

func TestCreateAccessRequest_SearchBased(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}
	var createdReq types.AccessRequest
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{createdReq}, nil
	}

	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		createdReq = req
		require.Equal(t, "userFoo", req.GetUser())
		require.Empty(t, req.GetRoles())
		require.Equal(t, "some reason", req.GetRequestReason())
		require.Equal(t, []types.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}}, req.GetRequestedResourceIDs())
		return nil
	}

	request := ui.AccessRequestParameters{
		Reason:      "some reason",
		ResourceIDs: []ui.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}},
	}

	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.NoError(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, types.RequestState_PENDING.String(), req.State)
}

func TestCreateAccessRequest_ConstrainedResource(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}
	var createdReq types.AccessRequest
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{createdReq}, nil
	}

	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		createdReq = req
		require.Equal(t, "userFoo", req.GetUser())
		require.Empty(t, req.GetRoles())
		require.Equal(t, "some reason", req.GetRequestReason())
		require.Equal(t, []types.ResourceAccessID{
			{
				Id: types.ResourceID{
					ClusterName: "test-cluster",
					Name:        "test-name",
					Kind:        "app",
				},
				Constraints: &types.ResourceConstraints{
					Version: types.V1,
					Details: &types.ResourceConstraints_AwsConsole{
						AwsConsole: &types.AWSConsoleResourceConstraints{
							RoleArns: []string{"test-role"},
						},
					},
				},
			},
		}, req.GetAllRequestedResourceIDs())
		return nil
	}

	request := ui.AccessRequestParameters{
		Reason: "some reason",
		ResourceAccessIDs: []ui.ResourceAccessID{
			{
				ID: ui.ResourceID{
					ClusterName: "test-cluster",
					Name:        "test-name",
					Kind:        "app",
				},
				Constraints: &types.ResourceConstraints{
					Version: types.V1,
					Details: &types.ResourceConstraints_AwsConsole{
						AwsConsole: &types.AWSConsoleResourceConstraints{
							RoleArns: []string{"test-role"},
						},
					},
				},
			},
		},
	}

	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.NoError(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, types.RequestState_PENDING.String(), req.State)
}

func TestCreateAccessRequest_LongTerm(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn:  accessrequest.GenerateAccessRequestPromotions,
		GenerateLongTermResourceGroupingFn: accessrequest.GenerateLongTermResourceGrouping,
	}
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond), withModules(testModules))
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	t.Cleanup(cancel)

	authClient := s.newAdminAuthClient(ctx, t)
	accessListClient := authClient.AccessListClient()

	_, err := authtest.CreateRole(ctx, authClient, "prod-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"env": []string{"prod"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "dev-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"env": []string{"dev"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"prod-access", "dev-access"},
			},
		},
	})
	require.NoError(t, err)

	node1, err := types.NewServerWithLabels(
		"prod-node",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"env": "prod"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, node1)
	require.NoError(t, err)

	node2, err := types.NewServerWithLabels(
		"dev-node",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"env": "dev"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, node2)
	require.NoError(t, err)

	createUserWithOpts(t, s, "testuser", withRoles("requester"))

	tests := []struct {
		name         string
		request      ui.AccessRequestParameters
		createProdAL bool
		createDevAL  bool
		expectError  bool
		errorMsg     string
		assertions   func(*testing.T, *ui.AccessRequest)
	}{
		{
			name: "successful long-term request for production node",
			request: ui.AccessRequestParameters{
				Reason:      "need long term access to prod",
				RequestKind: types.AccessRequestKind_LONG_TERM,
				ResourceIDs: []ui.ResourceID{
					{Name: "prod-node", Kind: types.KindNode},
				},
			},
			createProdAL: true,
			createDevAL:  true,
			expectError:  false,
			assertions: func(t *testing.T, req *ui.AccessRequest) {
				require.NotNil(t, req.LongTermResourceGrouping)
				require.True(t, req.LongTermResourceGrouping.CanProceed)
				require.Len(t, req.LongTermResourceGrouping.AccessListToResources[req.LongTermResourceGrouping.RecommendedAccessList], 1)
				require.Equal(t, "prod-node", req.LongTermResourceGrouping.AccessListToResources[req.LongTermResourceGrouping.RecommendedAccessList][0].Name)
				require.Contains(t, req.LongTermResourceGrouping.AccessListToResources, "prod-servers")
			},
		},
		{
			name: "long-term dry run request",
			request: ui.AccessRequestParameters{
				Reason:      "testing long term access",
				RequestKind: types.AccessRequestKind_LONG_TERM,
				DryRun:      true,
				ResourceIDs: []ui.ResourceID{
					{Name: "dev-node", Kind: types.KindNode},
				},
			},
			createProdAL: true,
			createDevAL:  true,
			expectError:  false,
			assertions: func(t *testing.T, req *ui.AccessRequest) {
				require.NotNil(t, req.LongTermResourceGrouping)
				require.True(t, req.LongTermResourceGrouping.CanProceed)
				require.Contains(t, req.LongTermResourceGrouping.AccessListToResources, "dev-servers")
			},
		},
		{
			name: "long-term request where only one of the requested resources is grantable",
			request: ui.AccessRequestParameters{
				Reason:      "attempt mixed access with missing access list",
				RequestKind: types.AccessRequestKind_LONG_TERM,
				ResourceIDs: []ui.ResourceID{
					{Name: "dev-node", Kind: types.KindNode},  // valid
					{Name: "prod-node", Kind: types.KindNode}, // invalid — no access list grants it
				},
			},
			createProdAL: false,
			createDevAL:  true,
			expectError:  true,
			errorMsg:     "Long-term access is not available for some selected resources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createProdAL {
				prodAccessList, err := accesslist.NewAccessList(
					header.Metadata{Name: "prod-servers"},
					accesslist.Spec{
						Title:  "Production Servers",
						Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
						Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
						MembershipRequires: accesslist.Requires{
							Roles: []string{"requester"},
						},
						Grants: accesslist.Grants{
							Roles: []string{"prod-access"},
						},
					},
				)
				require.NoError(t, err)
				_, err = accessListClient.UpsertAccessList(ctx, prodAccessList)
				require.NoError(t, err)
			}
			if tt.createDevAL {
				devAccessList, err := accesslist.NewAccessList(
					header.Metadata{Name: "dev-servers"},
					accesslist.Spec{
						Title:  "Development Servers",
						Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
						Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
						MembershipRequires: accesslist.Requires{
							Roles: []string{"requester"},
						},
						Grants: accesslist.Grants{
							Roles: []string{"dev-access"},
						},
					},
				)
				require.NoError(t, err)
				_, err = accessListClient.UpsertAccessList(ctx, devAccessList)
				require.NoError(t, err)
			}

			req, err := createAccessRequest(ctx, authClient, tt.request, "testuser")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)

			if tt.assertions != nil {
				tt.assertions(t, req)
			}

			if tt.createProdAL {
				err := accessListClient.DeleteAccessList(ctx, "prod-servers")
				require.NoError(t, err)
			}
			if tt.createDevAL {
				err := accessListClient.DeleteAccessList(ctx, "dev-servers")
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateAccessRequest_LongTerm_ValidationErrors(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn:  accessrequest.GenerateAccessRequestPromotions,
		GenerateLongTermResourceGroupingFn: accessrequest.GenerateLongTermResourceGrouping,
	}
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond), withModules(testModules))
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	t.Cleanup(cancel)

	authClient := s.newAdminAuthClient(ctx, t)
	accessListClient := authClient.AccessListClient()

	// role that doesn't grant access to anything useful
	_, err := authtest.CreateRole(ctx, authClient, "no-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"nonexistent": []string{"label"},
			},
		},
	})
	require.NoError(t, err)

	// requester role
	_, err = authtest.CreateRole(ctx, authClient, "limited-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"no-access"},
			},
		},
	})
	require.NoError(t, err)

	// node that won't be accessible by the no-access role
	node, err := types.NewServerWithLabels(
		"inaccessible-node",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"access": "denied"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, node)
	require.NoError(t, err)

	// user with limited access
	createUserWithOpts(t, s, "limiteduser", withRoles("limited-requester"))

	// access list that grants the no-access role (which can't access our node)
	noAccessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "no-access-list"},
		accesslist.Spec{
			Title:  "No Access List",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"limited-requester"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"no-access"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, noAccessList)
	require.NoError(t, err)

	tests := []struct {
		name        string
		request     ui.AccessRequestParameters
		user        string
		expectError bool
		errorMsg    string
	}{
		{
			name: "long-term request with no suitable access lists",
			request: ui.AccessRequestParameters{
				Reason:      "need access to inaccessible resource",
				RequestKind: types.AccessRequestKind_LONG_TERM,
				ResourceIDs: []ui.ResourceID{
					{Name: "inaccessible-node", Kind: types.KindNode},
				},
			},
			user:        "limiteduser",
			expectError: true,
			errorMsg:    "Long-term access is not available",
		},
		{
			name: "long-term request with nonexistent resource",
			request: ui.AccessRequestParameters{
				Reason:      "need access to nonexistent resource",
				RequestKind: types.AccessRequestKind_LONG_TERM,
				ResourceIDs: []ui.ResourceID{
					{Name: "nonexistent-node", Kind: types.KindNode},
				},
			},
			user:        "limiteduser",
			expectError: true,
			errorMsg:    "Long-term access is not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := createAccessRequest(ctx, authClient, tt.request, tt.user)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.Contains(t, err.Error(), tt.errorMsg)
				}
				require.Nil(t, req)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)
		})
	}
}

func TestCreateAccessRequest_LongTerm_ConflictingResources(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn:  accessrequest.GenerateAccessRequestPromotions,
		GenerateLongTermResourceGroupingFn: accessrequest.GenerateLongTermResourceGrouping,
	}
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond), withModules(testModules))
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	t.Cleanup(cancel)

	authClient := s.newAdminAuthClient(ctx, t)
	accessListClient := authClient.AccessListClient()

	_, err := authtest.CreateRole(ctx, authClient, "prod-only", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"env": []string{"prod"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "dev-only", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"env": []string{"dev"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "multi-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"prod-only", "dev-only"},
			},
		},
	})
	require.NoError(t, err)

	prodNode, err := types.NewServerWithLabels(
		"prod-server",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"env": "prod"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, prodNode)
	require.NoError(t, err)

	devNode, err := types.NewServerWithLabels(
		"dev-server",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"env": "dev"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, devNode)
	require.NoError(t, err)

	createUserWithOpts(t, s, "multiuser", withRoles("multi-requester"))

	prodOnlyList, err := accesslist.NewAccessList(
		header.Metadata{Name: "prod-only-list"},
		accesslist.Spec{
			Title:  "Production Only Access",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"multi-requester"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"prod-only"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, prodOnlyList)
	require.NoError(t, err)

	devOnlyList, err := accesslist.NewAccessList(
		header.Metadata{Name: "dev-only-list"},
		accesslist.Spec{
			Title:  "Development Only Access",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"multi-requester"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"dev-only"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, devOnlyList)
	require.NoError(t, err)

	request := ui.AccessRequestParameters{
		Reason:      "need access to both prod and dev",
		RequestKind: types.AccessRequestKind_LONG_TERM,
		ResourceIDs: []ui.ResourceID{
			{Name: "prod-server", Kind: types.KindNode},
			{Name: "dev-server", Kind: types.KindNode},
		},
	}

	req, err := createAccessRequest(ctx, authClient, request, "multiuser")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Selected resources cannot be grouped for long-term access")
	require.Nil(t, req)
}

func TestCreateAccessRequest_LongTerm_OptimalSelection(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn:  accessrequest.GenerateAccessRequestPromotions,
		GenerateLongTermResourceGroupingFn: accessrequest.GenerateLongTermResourceGrouping,
	}
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond), withModules(testModules))
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	t.Cleanup(cancel)

	authClient := s.newAdminAuthClient(ctx, t)
	accessListClient := authClient.AccessListClient()

	_, err := authtest.CreateRole(ctx, authClient, "web-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"service": []string{"web"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "db-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"service": []string{"db"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "full-stack-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"service": []string{"web", "db"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "stack-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"web-access", "db-access", "full-stack-access"},
			},
		},
	})
	require.NoError(t, err)

	webNode, err := types.NewServerWithLabels(
		"web-server",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"service": "web"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, webNode)
	require.NoError(t, err)

	dbNode, err := types.NewServerWithLabels(
		"db-server",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"service": "db"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, dbNode)
	require.NoError(t, err)

	createUserWithOpts(t, s, "stackuser", withRoles("stack-requester"))

	// list that only grants web access
	webOnlyList, err := accesslist.NewAccessList(
		header.Metadata{Name: "web-only-list"},
		accesslist.Spec{
			Title:  "Web Only Access",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"stack-requester"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"web-access"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, webOnlyList)
	require.NoError(t, err)

	// list that only grants db access
	dbOnlyList, err := accesslist.NewAccessList(
		header.Metadata{Name: "db-only-list"},
		accesslist.Spec{
			Title:  "Database Only Access",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"stack-requester"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"db-access"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, dbOnlyList)
	require.NoError(t, err)

	// list that grants full stack access (optimal choice)
	fullStackList, err := accesslist.NewAccessList(
		header.Metadata{Name: "full-stack-list"},
		accesslist.Spec{
			Title:  "Full Stack Access",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"stack-requester"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"full-stack-access"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, fullStackList)
	require.NoError(t, err)

	// request for access to both web and db servers
	// should succeed and choose the full-stack access list as optimal
	request := ui.AccessRequestParameters{
		Reason:      "need access to full stack",
		RequestKind: types.AccessRequestKind_LONG_TERM,
		ResourceIDs: []ui.ResourceID{
			{Name: "web-server", Kind: types.KindNode},
			{Name: "db-server", Kind: types.KindNode},
		},
	}

	req, err := createAccessRequest(ctx, authClient, request, "stackuser")
	require.NoError(t, err)
	require.NotNil(t, req)

	require.NotNil(t, req.LongTermResourceGrouping)
	require.True(t, req.LongTermResourceGrouping.CanProceed)

	// should have the full-stack access list as optimal
	require.Equal(t, "full-stack-list", req.LongTermResourceGrouping.RecommendedAccessList)
	require.Len(t, req.LongTermResourceGrouping.AccessListToResources[req.LongTermResourceGrouping.RecommendedAccessList], 2)

	// optimal grouping should contain both servers
	nodeNames := make([]string, len(req.LongTermResourceGrouping.AccessListToResources[req.LongTermResourceGrouping.RecommendedAccessList]))
	for i, resource := range req.LongTermResourceGrouping.AccessListToResources[req.LongTermResourceGrouping.RecommendedAccessList] {
		nodeNames[i] = resource.Name
	}
	require.ElementsMatch(t, []string{"web-server", "db-server"}, nodeNames)

	// should also show other access lists are available but cover fewer resources
	require.Contains(t, req.LongTermResourceGrouping.AccessListToResources, "web-only-list")
	require.Contains(t, req.LongTermResourceGrouping.AccessListToResources, "db-only-list")

	// the web-only and db-only lists should only cover one resource each
	require.Len(t, req.LongTermResourceGrouping.AccessListToResources["web-only-list"], 1)
	require.Len(t, req.LongTermResourceGrouping.AccessListToResources["db-only-list"], 1)
}

func TestCreateAccessRequest_LongTerm_InheritedAccessListMembership(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn:  accessrequest.GenerateAccessRequestPromotions,
		GenerateLongTermResourceGroupingFn: accessrequest.GenerateLongTermResourceGrouping,
	}
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond), withModules(testModules))
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	t.Cleanup(cancel)

	authClient := s.newAdminAuthClient(ctx, t)
	accessListClient := authClient.AccessListClient()

	_, err := authtest.CreateRole(ctx, authClient, "test-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabelsExpression: `labels["env"] == "prod" && contains(user.spec.traits["teams"], labels["team"])`,
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "full-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"env": []string{"prod"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authClient, "requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"full-access"},
			},
		},
	})
	require.NoError(t, err)

	node, err := types.NewServerWithLabels(
		"shared-node",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"env": "prod", "team": "ipsum"},
	)
	require.NoError(t, err)
	_, err = authClient.UpsertNode(ctx, node)
	require.NoError(t, err)

	createUserWithOpts(t, s, "test-user", withRoles("requester"))

	// parent list that grants trait required for role
	parentList, err := accesslist.NewAccessList(
		header.Metadata{Name: "parent-list"},
		accesslist.Spec{
			Title:  "Parent List",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			Grants: accesslist.Grants{Traits: map[string][]string{"teams": {"ipsum"}}},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"requester"},
			},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, parentList)
	require.NoError(t, err)

	// child list that grants role
	childList, err := accesslist.NewAccessList(
		header.Metadata{Name: "child-list"},
		accesslist.Spec{
			Title:  "Child List",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: "admin", Description: "admin"}},
			Grants: accesslist.Grants{Roles: []string{"test-role"}},
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, childList)
	require.NoError(t, err)

	// add child-list as a member of parent-list, so child now grants required role as well
	childMember, err := accesslist.NewAccessListMember(
		header.Metadata{Name: "child-list"},
		accesslist.AccessListMemberSpec{
			AccessList:     "parent-list",
			Name:           "child-list",
			MembershipKind: accesslist.MembershipKindList,
			Joined:         time.Now().Add(-1 * time.Hour),
			AddedBy:        "admin",
		},
	)
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessListMember(ctx, childMember)
	require.NoError(t, err)

	req, err := createAccessRequest(ctx, authClient, ui.AccessRequestParameters{
		Reason:      "request with inherited grants",
		RequestKind: types.AccessRequestKind_LONG_TERM,
		ResourceIDs: []ui.ResourceID{
			{Name: "shared-node", Kind: types.KindNode},
		},
	}, "test-user")
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NotNil(t, req.LongTermResourceGrouping)
	require.True(t, req.LongTermResourceGrouping.CanProceed)
	require.Contains(t, req.LongTermResourceGrouping.AccessListToResources, "child-list")
	require.NotContains(t, req.LongTermResourceGrouping.AccessListToResources, "parent-list")
}

type mockAuthClient struct {
	authclient.ClientI
	resources []types.ResourceWithLabels
}

func (m *mockAuthClient) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	return &types.ListResourcesResponse{
		Resources: m.resources,
	}, nil
}

type mockClusterClientProvider struct {
	resourcesByCluster map[string][]types.ResourceWithLabels
}

func (m *mockClusterClientProvider) UserClientForCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	return &mockAuthClient{
		resources: m.resourcesByCluster[clusterName],
	}, nil
}

type mockResource struct {
	types.ResourceWithLabels
	kind, name, description, origin string
}

func (m *mockResource) GetKind() string {
	return m.kind
}

func (m *mockResource) GetName() string {
	return m.name
}

func (m *mockResource) Origin() string {
	return m.origin
}

func (m *mockResource) GetMetadata() types.Metadata {
	return types.Metadata{
		Description: m.description,
	}
}

type mockResourceWithHostname struct {
	mockResource
	hostname string
}

func (m *mockResourceWithHostname) GetHostname() string {
	return m.hostname
}

func TestGetAccessRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	m := &mockedAccessRequestAPIGetter{}

	noRequestID := ""
	wrongRequestID := "asdf"

	app, err := types.NewAppV3(types.Metadata{
		Name:        "app",
		Description: "friendly name",
		Labels: map[string]string{
			types.OriginLabel: types.OriginOkta,
		},
	}, types.AppSpecV3{
		URI:        "https://some-uri.com",
		PublicAddr: "https://some-uri.com",
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		desc               string
		requestedRoles     []string
		requestedResources []types.ResourceID
		requestIDOverride  *string
		resourcesByCluster map[string][]types.ResourceWithLabels
		expectError        bool
		resultAssertion    func(*testing.T, *ui.AccessRequest)
	}{
		{
			desc:           "basic",
			requestedRoles: []string{"*"},
		},
		{
			desc:              "empty request ID",
			requestedRoles:    []string{"*"},
			requestIDOverride: &noRequestID,
			expectError:       true,
		},
		{
			desc:              "no such request",
			requestedRoles:    []string{"*"},
			requestIDOverride: &wrongRequestID,
			expectError:       true,
		},
		{
			desc: "with requested node",
			requestedResources: []types.ResourceID{{
				ClusterName: "test-cluster",
				Kind:        types.KindNode,
				Name:        "test-node",
			}},
			resourcesByCluster: map[string][]types.ResourceWithLabels{
				"test-cluster": {
					&mockResourceWithHostname{
						mockResource: mockResource{
							kind: types.KindNode,
							name: "test-node",
						},
						hostname: "test-hostname",
					},
				},
			},
			resultAssertion: func(t *testing.T, res *ui.AccessRequest) {
				require.Len(t, res.Resources, 1)
				require.Equal(t, "test-hostname", res.Resources[0].Details.FriendlyName)
			},
		},
		{
			desc: "with Okta app",
			requestedResources: []types.ResourceID{{
				ClusterName: "test-cluster",
				Kind:        types.KindApp,
				Name:        "app",
			}},
			resourcesByCluster: map[string][]types.ResourceWithLabels{
				"test-cluster": {
					app,
				},
			},
			resultAssertion: func(t *testing.T, res *ui.AccessRequest) {
				require.Len(t, res.Resources, 1)
				require.Equal(t, "friendly name", res.Resources[0].Details.FriendlyName)
			},
		},
		{
			// Tests the case where requested resources are in multiple
			// different clusters.
			desc: "multiple clusters",
			requestedResources: []types.ResourceID{
				{
					ClusterName: "test-cluster-1",
					Kind:        types.KindNode,
					Name:        "test-node-1",
				},
				{
					ClusterName: "test-cluster-2",
					Kind:        types.KindNode,
					Name:        "test-node-2",
				},
			},
			resourcesByCluster: map[string][]types.ResourceWithLabels{
				"test-cluster-1": {
					&mockResourceWithHostname{
						mockResource: mockResource{
							kind: types.KindNode,
							name: "test-node-1",
						},
						hostname: "test-hostname-1",
					},
				},
				"test-cluster-2": {
					&mockResourceWithHostname{
						mockResource: mockResource{
							kind: types.KindNode,
							name: "test-node-2",
						},
						hostname: "test-hostname-2",
					},
				},
			},
			resultAssertion: func(t *testing.T, res *ui.AccessRequest) {
				require.Len(t, res.Resources, 2)
				require.Equal(t, "test-node-1", res.Resources[0].ID.Name)
				require.Equal(t, "test-hostname-1", res.Resources[0].Details.FriendlyName)
				require.Equal(t, "test-node-2", res.Resources[1].ID.Name)
				require.Equal(t, "test-hostname-2", res.Resources[1].Details.FriendlyName)
			},
		},
		{
			// Tests the case where the reviewer does not have permission to
			// list one of the resources.
			desc: "missing resource",
			requestedResources: []types.ResourceID{
				{
					ClusterName: "test-cluster-1",
					Kind:        types.KindNode,
					Name:        "test-node-1",
				},
				{
					ClusterName: "test-cluster-2",
					Kind:        types.KindNode,
					Name:        "test-node-2",
				},
			},
			resourcesByCluster: map[string][]types.ResourceWithLabels{
				"test-cluster-1": {
					&mockResourceWithHostname{
						mockResource: mockResource{
							kind: types.KindNode,
							name: "test-node-1",
						},
						hostname: "test-hostname-1",
					},
				},
			},
			resultAssertion: func(t *testing.T, res *ui.AccessRequest) {
				require.Len(t, res.Resources, 2)
				require.Equal(t, "test-node-1", res.Resources[0].ID.Name)
				require.Equal(t, "test-hostname-1", res.Resources[0].Details.FriendlyName)

				// test-node-2 should be included but the hostname should be missing
				require.Equal(t, "test-node-2", res.Resources[1].ID.Name)
				require.Empty(t, res.Resources[1].Details.FriendlyName)
			},
		},
		{
			// Tests the case where requested resources are of multiple
			// different kinds
			desc: "multiple resource kinds",
			requestedResources: []types.ResourceID{
				{
					ClusterName: "test-cluster-1",
					Kind:        types.KindNode,
					Name:        "test-node-1",
				},
				{
					ClusterName: "test-cluster-1",
					Kind:        types.KindApp,
					Name:        "test-app-1",
				},
				{
					ClusterName: "test-cluster-1",
					Kind:        types.KindKubernetesCluster,
					Name:        "test-kube-1",
				},
			},
			resourcesByCluster: map[string][]types.ResourceWithLabels{
				"test-cluster-1": {
					&mockResourceWithHostname{
						mockResource: mockResource{
							kind: types.KindNode,
							name: "test-node-1",
						},
						hostname: "test-hostname-1",
					},
					&mockResource{
						kind: types.KindApp,
						name: "test-app-1",
					},
					&mockResource{
						kind: types.KindKubernetesCluster,
						name: "test-kube-1",
					},
				},
			},
			resultAssertion: func(t *testing.T, req *ui.AccessRequest) {
				// Node should have a hostname, others shouldn't
				require.Len(t, req.Resources, 3)
				require.Equal(t, "test-node-1", req.Resources[0].ID.Name)
				require.Equal(t, "test-hostname-1", req.Resources[0].Details.FriendlyName)
				require.Equal(t, "test-app-1", req.Resources[1].ID.Name)
				require.Empty(t, req.Resources[1].Details.FriendlyName)
				require.Equal(t, "test-kube-1", req.Resources[2].ID.Name)
				require.Empty(t, req.Resources[2].Details.FriendlyName)
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			req, err := services.NewAccessRequestWithResources("alice", tc.requestedRoles, types.ResourceIDsToResourceAccessIDs(tc.requestedResources))
			require.NoError(t, err)

			m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
				if filter.ID == req.GetName() {
					return []types.AccessRequest{req}, nil
				}
				return nil, trace.NotFound("no such access request")
			}

			clusterClientProvider := &mockClusterClientProvider{
				resourcesByCluster: tc.resourcesByCluster,
			}

			requestID := req.GetName()
			if tc.requestIDOverride != nil {
				requestID = *tc.requestIDOverride
			}

			result, err := getAccessRequest(ctx, m, requestID, withClusterClientProvider(clusterClientProvider))
			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)

			require.Equal(t, req.GetName(), result.ID)
			require.Equal(t, "alice", result.User)

			require.Len(t, result.Resources, len(tc.requestedResources))
			for i := range tc.requestedResources {
				require.Equal(t, tc.requestedResources[i].ClusterName, result.Resources[i].ID.ClusterName)
				require.Equal(t, tc.requestedResources[i].Kind, result.Resources[i].ID.Kind)
				require.Equal(t, tc.requestedResources[i].Name, result.Resources[i].ID.Name)
			}

			if tc.resultAssertion != nil {
				tc.resultAssertion(t, result)
			}
		})
	}
}

func TestGetAccessRequests(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	m.mockListAccessRequests = func(ctx context.Context, request *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error) {
		roleBasedReq1, err := services.NewAccessRequest("baz", []string{"bar"}...)
		require.NoError(t, err)
		roleBasedReq1.SetState(types.RequestState_NONE)
		rb1, ok := roleBasedReq1.(*types.AccessRequestV3)
		require.True(t, ok)

		roleBasedReq2, err := services.NewAccessRequest("foz", []string{"foo"}...)
		require.NoError(t, err)
		roleBasedReq2.SetState(types.RequestState_APPROVED)
		rb2, ok := roleBasedReq2.(*types.AccessRequestV3)
		require.True(t, ok)

		searchBasedReq, err := services.NewAccessRequestWithResources("bar", nil, []types.ResourceAccessID{{Id: types.ResourceID{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}}})
		require.NoError(t, err)
		sb, ok := searchBasedReq.(*types.AccessRequestV3)
		require.True(t, ok)

		return &proto.ListAccessRequestsResponse{
			AccessRequests: []*types.AccessRequestV3{rb1, rb2, sb},
		}, nil
	}

	plugin, err := NewPlugin(Config{})
	require.NoError(t, err)

	request := &proto.ListAccessRequestsRequest{
		Filter: &types.AccessRequestFilter{},
	}
	// Test request state set to NONE, is not returned.
	reqs, err := plugin.getAccessRequests(context.Background(), m, request)
	require.NoError(t, err)
	require.Len(t, reqs.AccessRequests, 2)
	require.Equal(t, types.RequestState_APPROVED.String(), reqs.AccessRequests[0].State)
	require.Equal(t, types.RequestState_PENDING.String(), reqs.AccessRequests[1].State)
	require.Equal(t, []ui.Resource{{ID: ui.ResourceID{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}}}, reqs.AccessRequests[1].Resources)
}

func TestReviewAccessRequest(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	fakeReq, err := services.NewAccessRequest("foo", []string{"bar"}...)
	require.NoError(t, err)
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{fakeReq}, nil
	}

	m.mockSubmitAccessReview = func(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
		require.Equal(t, fakeReq.GetMetadata().Name, params.RequestID)
		require.Equal(t, types.RequestState_DENIED, params.Review.ProposedState)
		require.Equal(t, "Not today", params.Review.Reason)
		require.Empty(t, params.Review.Roles)
		return fakeReq, nil
	}

	reviewSubmission := ui.AccessRequestParameters{
		State:  "DENIED",
		Reason: "Not today",
		ID:     fakeReq.GetMetadata().Name,
	}

	_, err = reviewAccessRequest(context.Background(), m, reviewSubmission)
	require.NoError(t, err)

	// Test error paths.
	reviewSubmission.State = "NONE"
	req, err := reviewAccessRequest(context.Background(), m, reviewSubmission)
	require.True(t, trace.IsBadParameter(err))
	require.Nil(t, req)

	reviewSubmission.State = "PENDING"
	req, err = reviewAccessRequest(context.Background(), m, reviewSubmission)
	require.True(t, trace.IsBadParameter(err))
	require.Nil(t, req)

	reviewSubmission.State = ""
	req, err = reviewAccessRequest(context.Background(), m, reviewSubmission)
	require.True(t, trace.IsBadParameter(err))
	require.Nil(t, req)
}

func TestReviewAccessRequest_Approved(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	validStartTime := time.Now().UTC().Add(1 * time.Hour)

	fakeReq, err := services.NewAccessRequest("foo", []string{"bar"}...)
	require.NoError(t, err)
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{fakeReq}, nil
	}

	m.mockSubmitAccessReview = func(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
		require.Equal(t, fakeReq.GetMetadata().Name, params.RequestID)
		require.Equal(t, types.RequestState_APPROVED, params.Review.ProposedState)
		require.Equal(t, &validStartTime, params.Review.AssumeStartTime)
		return fakeReq, nil
	}

	reviewSubmission := ui.AccessRequestParameters{
		State:           "APPROVED",
		ID:              fakeReq.GetMetadata().Name,
		AssumeStartTime: &validStartTime,
	}

	_, err = reviewAccessRequest(context.Background(), m, reviewSubmission)
	require.NoError(t, err)
}

type mockedAccessRequestAPIGetter struct {
	mockCreateAccessRequest func(ctx context.Context, req types.AccessRequest) error
	mockGetAccessRequests   func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	mockListAccessRequests  func(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error)
	mockSubmitAccessReview  func(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)

	mockGetAccessRequestAllowedPromotions func(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
	mockGetUser                           func(ctx context.Context, userName string, withSecrets bool) (types.User, error)
	mockGetRole                           func(ctx context.Context, name string) (types.Role, error)
	mockListResources                     func(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

func (m *mockedAccessRequestAPIGetter) GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error) {
	if m.mockGetUser != nil {
		return m.mockGetUser(ctx, userName, withSecrets)
	}

	return nil, trace.NotImplemented("mockGetUser not implemented")
}

func (m *mockedAccessRequestAPIGetter) GetRole(ctx context.Context, name string) (types.Role, error) {
	if m.mockGetRole != nil {
		return m.mockGetRole(ctx, name)
	}

	return nil, trace.NotImplemented("mockGetRole not implemented")
}

func (m *mockedAccessRequestAPIGetter) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	if m.mockListResources != nil {
		return m.mockListResources(ctx, req)
	}

	return nil, trace.NotImplemented("mockListResources not implemented")
}

func (m *mockedAccessRequestAPIGetter) GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	if m.mockGetAccessRequestAllowedPromotions != nil {
		return m.mockGetAccessRequestAllowedPromotions(ctx, req)
	}

	return &types.AccessRequestAllowedPromotions{Promotions: []*types.AccessRequestAllowedPromotion{}}, nil
}

func (m *mockedAccessRequestAPIGetter) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	if m.mockCreateAccessRequest != nil {
		return m.mockCreateAccessRequest(ctx, req)
	}

	return trace.NotImplemented("mockCreateAccessRequest not implemented")
}

func (m *mockedAccessRequestAPIGetter) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	if m.mockCreateAccessRequest != nil {
		return req, m.mockCreateAccessRequest(ctx, req)
	}

	return nil, trace.NotImplemented("mockCreateAccessRequest not implemented")
}

func (m *mockedAccessRequestAPIGetter) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	if m.mockGetAccessRequests != nil {
		return m.mockGetAccessRequests(ctx, filter)
	}

	return nil, trace.NotImplemented("mockGetAccessRequests not implemented")
}

func (m *mockedAccessRequestAPIGetter) ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error) {
	if m.mockListAccessRequests != nil {
		return m.mockListAccessRequests(ctx, req)
	}

	return nil, trace.NotImplemented("mockListAccessRequests not implemented")
}

func (m *mockedAccessRequestAPIGetter) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
	if m.mockSubmitAccessReview != nil {
		return m.mockSubmitAccessReview(ctx, params)
	}

	return nil, trace.NotImplemented("mockSubmitAccessReview not implemented")
}

func TestSuggestAccessLists(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn: accessrequest.GenerateAccessRequestPromotions,
	}
	modulestest.SetTestModules(t, *testModules)

	ctx := context.Background()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
	)
	authServer := s.testAuthServer.AuthServer.AuthServer

	// create requester, access and godmode roles
	const requesterRoleName = "requester"
	_, err := authtest.CreateRole(ctx, authServer, requesterRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authServer, "access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"name": []string{"node"},
			},
		},
	})
	require.NoError(t, err)

	_, err = authtest.CreateRole(ctx, authServer, "godmode", types.RoleSpecV6{})
	require.NoError(t, err)

	// create a node, so we can request access to it
	const nodeName = "node"
	node, err := types.NewServerWithLabels(
		nodeName,
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"name": nodeName},
	)
	require.NoError(t, err)

	_, err = authServer.UpsertNode(ctx, node)
	require.NoError(t, err)

	// assign the admin role and preferred_drink=fanta to reviewer
	createUserWithOpts(t, s, "reviewer", withRoles(requesterRoleName), withTraits(trait.Traits{"preferred_drink": []string{"fanta"}}), withPassword())

	webPack := s.newAuthWebPack(t, "reviewer", skipUserCreation())

	// create a new client with the user after the user has been modified to ensure that the
	// user login state reflects the current user modifications.
	authClient := s.newAdminAuthClient(s.ctx, t)
	accessListClient := authClient.AccessListClient()

	// create four access lists:
	// - one that is a close match
	// - one that is overprivileged
	// - one that is missing access due to a role mismatch
	// - one that is missing access due to a trait mismatch
	accessListCloseMatch, err := accesslist.NewAccessList(header.Metadata{Name: "close-match"}, accesslist.Spec{
		Title:              "close match",
		Audit:              accesslist.Audit{NextAuditDate: s.clock.Now()},
		Owners:             []accesslist.Owner{{Name: "reviewer", Description: "reviewer desc"}},
		MembershipRequires: accesslist.Requires{Roles: []string{requesterRoleName}, Traits: trait.Traits{"preferred_drink": []string{"fanta"}}},
		Grants:             accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	accessListCloseMatch, err = accessListClient.UpsertAccessList(ctx, accessListCloseMatch)
	require.NoError(t, err)
	accessListOverprivileged, err := accesslist.NewAccessList(header.Metadata{Name: "overprivileged"}, accesslist.Spec{
		Title:              "overprivileged",
		Audit:              accesslist.Audit{NextAuditDate: s.clock.Now()},
		Owners:             []accesslist.Owner{{Name: "reviewer", Description: "reviewer desc"}},
		MembershipRequires: accesslist.Requires{Roles: []string{requesterRoleName}, Traits: trait.Traits{"preferred_drink": []string{"fanta"}}},
		Grants:             accesslist.Grants{Roles: []string{"access", "godmode"}},
	})
	require.NoError(t, err)
	accessListOverprivileged, err = accessListClient.UpsertAccessList(ctx, accessListOverprivileged)
	require.NoError(t, err)
	accessListWrongMembershipReq, err := accesslist.NewAccessList(header.Metadata{Name: "wrong-membership-requirements-role"}, accesslist.Spec{
		Title:              "missing access role",
		Audit:              accesslist.Audit{NextAuditDate: s.clock.Now()},
		Owners:             []accesslist.Owner{{Name: "reviewer", Description: "reviewer desc"}},
		MembershipRequires: accesslist.Requires{Roles: []string{"godmode"}},
		Grants:             accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, accessListWrongMembershipReq)
	require.NoError(t, err)
	accessListMissingAccessTrait, err := accesslist.NewAccessList(header.Metadata{Name: "missing-access-trait"}, accesslist.Spec{
		Title:              "missing access trait",
		Audit:              accesslist.Audit{NextAuditDate: s.clock.Now()},
		Owners:             []accesslist.Owner{{Name: "reviewer", Description: "reviewer desc"}},
		MembershipRequires: accesslist.Requires{Traits: trait.Traits{"preferred_drink": []string{"coke"}}},
		Grants:             accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	_, err = accessListClient.UpsertAccessList(ctx, accessListMissingAccessTrait)
	require.NoError(t, err)

	// verify all access lists exist in the backend
	existingLists, err := accessListClient.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, existingLists, 4)

	// create an access request for reviewer to request access to the "access" role
	accessRequest, err := services.NewAccessRequestWithResources("reviewer", []string{"access"},
		[]types.ResourceAccessID{
			{Id: types.ResourceID{Name: nodeName, Kind: types.KindNode}},
		})
	require.NoError(t, err)

	accessRequest, err = authClient.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	// fetch suggestions from web api
	endpoint := webPack.clt.Endpoint("enterprise", "accessrequest", accessRequest.GetName(), "suggestions", "accesslist")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	var accessListResp ui.SuggestedAccessLists
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))

	// check such as the suggestion ordering is correct and that one was rejected
	require.Len(t, accessListResp.AccessLists, 2)

	ignoreFieldsFn := cmp.FilterPath(func(path cmp.Path) bool {
		p := path.String()
		// ResourceHeader.Metadata.ID is not set on the request
		// Spec.Owners.IneligibleStatus is not set on the response
		return p == "ResourceHeader.Metadata.ID" || p == "Spec.Owners.IneligibleStatus" || p == "ResourceHeader.Metadata.Revision"
	}, cmp.Ignore())

	require.Empty(t, cmp.Diff(accessListCloseMatch, accessListResp.AccessLists[0], ignoreFieldsFn))
	require.Empty(t, cmp.Diff(accessListOverprivileged, accessListResp.AccessLists[1], ignoreFieldsFn))
}

func TestPromoteAccessRequest(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn: accessrequest.GenerateAccessRequestPromotions,
	}
	modulestest.SetTestModules(t, *testModules)

	ctx := context.Background()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
	)

	authServer := s.testAuthServer.AuthServer.AuthServer
	authClient := s.newAdminAuthClient(s.ctx, t)

	createNode := func() types.Server {
		const nodeName = "node"
		node, err := types.NewServerWithLabels(
			nodeName,
			types.KindNode,
			types.ServerSpecV2{},
			map[string]string{"name": nodeName},
		)
		require.NoError(t, err)

		_, err = authServer.UpsertNode(ctx, node)
		require.NoError(t, err)

		return node
	}

	// create a node, so we can request access to it
	node := createNode()

	createAccessRequest := func() types.AccessRequest {
		// create an access request for reviewer to request access to the "access" role
		accessRequest, err := services.NewAccessRequestWithResources("requester", []string{"access"}, []types.ResourceAccessID{
			{Id: types.ResourceID{Name: node.GetName(), Kind: types.KindNode}},
		})
		require.NoError(t, err)
		accessRequest, err = authClient.CreateAccessRequestV2(ctx, accessRequest)
		require.NoError(t, err)

		return accessRequest
	}

	accessListClient := authClient.AccessListClient()

	createAccessList := func() *accesslist.AccessList {
		accessListCloseMatch, err := accesslist.NewAccessList(
			header.Metadata{
				Name: "close-match",
			},
			accesslist.Spec{
				Title:              "close match title",
				Audit:              accesslist.Audit{NextAuditDate: s.clock.Now()},
				Owners:             []accesslist.Owner{{Name: "reviewer", Description: "reviewer desc"}},
				MembershipRequires: accesslist.Requires{},
				Grants:             accesslist.Grants{Roles: []string{"access"}},
			})
		require.NoError(t, err)
		accessListCloseMatch, err = accessListClient.UpsertAccessList(ctx, accessListCloseMatch)
		require.NoError(t, err)

		return accessListCloseMatch
	}

	createAccessListNoAccess := func() *accesslist.AccessList {
		accessListCloseMatch, err := accesslist.NewAccessList(
			header.Metadata{
				Name: "no-access",
			},
			accesslist.Spec{
				Title:              "no access title",
				Audit:              accesslist.Audit{NextAuditDate: s.clock.Now()},
				Owners:             []accesslist.Owner{{Name: "reviewer", Description: "reviewer desc"}},
				MembershipRequires: accesslist.Requires{},
				Grants:             accesslist.Grants{Roles: []string{"nonexistent-role"}},
			})
		require.NoError(t, err)
		accessListCloseMatch, err = accessListClient.UpsertAccessList(ctx, accessListCloseMatch)
		require.NoError(t, err)

		return accessListCloseMatch
	}

	upsertRole := func(roleName string, allow types.RoleConditions) {
		_, err := authtest.CreateRole(context.Background(), authServer, roleName, types.RoleSpecV6{
			Allow: allow,
		})
		require.NoError(t, err)
	}

	// create a role that allows the reviewer to promote access requests
	upsertRole("reviewerRole", types.RoleConditions{
		ReviewRequests: &types.AccessReviewConditions{
			Roles: []string{"access"},
		},
	})

	// create a role that allows the requester to promote access requests
	upsertRole("requesterRole", types.RoleConditions{
		Request: &types.AccessRequestConditions{
			SearchAsRoles: []string{"access"},
		},
	})

	// create a role that can be requested
	upsertRole("access", types.RoleConditions{
		NodeLabels: types.Labels{
			"name": []string{"node"},
		},
	})

	// create users with required roles
	createUserWithOpts(t, s, "reviewer", withRoles("reviewerRole"), withPassword())
	createUserWithOpts(t, s, "requester", withRoles("requesterRole"), withPassword())

	accessList := createAccessList()
	accessListNoAccess := createAccessListNoAccess()
	accessRequest := createAccessRequest()

	// verify all access lists exist in the backend
	existingLists, err := accessListClient.GetAccessLists(ctx)
	require.NoError(t, err)
	require.Len(t, existingLists, 2)

	// login user as reviewer
	webPack := s.newAuthWebPack(t, "reviewer", skipUserCreation())

	endpoint := webPack.clt.Endpoint("enterprise", "accessrequest", accessRequest.GetName(), "promote")

	// Promoting an access request to not allowed access list should fail
	_, err = webPack.clt.PostJSON(s.ctx, endpoint, &accessRequestPromoteParameters{
		Reason:         "promotion reason",
		AccessListName: accessListNoAccess.GetName(),
	})
	require.Error(t, err)

	// Promoting an access request to allowed access list should succeed
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, &accessRequestPromoteParameters{
		Reason:         "promotion reason",
		AccessListName: accessList.GetName(),
	})
	require.NoError(t, err)

	var promoteResp accessRequestPromoteResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &promoteResp))

	promotedAccessReq := promoteResp.AccessRequest

	// Verify the promoted access request has the correct fields
	require.Equal(t, accessRequest.GetUser(), promotedAccessReq.User)
	require.Equal(t, types.RequestState_PROMOTED.String(), promotedAccessReq.State)
	require.Equal(t, "promotion reason", promotedAccessReq.ResolveReason)
	require.Equal(t, accessList.Spec.Title, promotedAccessReq.PromotedAccessListTitle)
}

func TestCreateAccessRequest_SuggestedReviewers(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestSuggestedReviewersFn: accessrequest.GenerateAccessRequestSuggestedReviewers,
		GenerateLongTermResourceGroupingFn:        accessrequest.GenerateLongTermResourceGrouping,
	}
	modulestest.SetTestModules(t, *testModules)

	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond), withModules(testModules))

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	t.Cleanup(cancel)

	authClient := s.newAdminAuthClient(ctx, t)
	accessListClient := authClient.AccessListClient()

	accessRole, err := authtest.CreateRole(ctx, authClient, "access-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{"*": []string{"*"}},
		},
	})
	require.NoError(t, err)

	requesterRole, err := authtest.CreateRole(ctx, authClient, "requester-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles:         []string{accessRole.GetName()},
				SearchAsRoles: []string{accessRole.GetName()},
			},
		},
	})
	require.NoError(t, err)

	reviewerRole, err := authtest.CreateRole(ctx, authClient, "reviewer-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			ReviewRequests: &types.AccessReviewConditions{
				Roles: []string{accessRole.GetName()},
			},
		},
	})
	require.NoError(t, err)

	ownerUser, err := types.NewUser("test-owner")
	require.NoError(t, err)

	_, err = authClient.UpsertUser(ctx, ownerUser)
	require.NoError(t, err)

	testAccessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "test-servers"},
		accesslist.Spec{
			Title:  "Test Servers",
			Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
			Owners: []accesslist.Owner{{Name: ownerUser.GetName(), Description: "Owner of the list"}},
			Grants: accesslist.Grants{
				Roles: []string{requesterRole.GetName()},
			},
			OwnerGrants: accesslist.Grants{
				Roles: []string{reviewerRole.GetName()},
			},
		},
	)
	require.NoError(t, err)

	_, err = accessListClient.UpsertAccessList(ctx, testAccessList)
	require.NoError(t, err)

	node, err := types.NewServer(
		"test-node",
		types.KindNode,
		types.ServerSpecV2{},
	)
	require.NoError(t, err)

	_, err = authClient.UpsertNode(ctx, node)
	require.NoError(t, err)

	t.Run("user who is not a member gets no suggested reviewers", func(t *testing.T) {
		requesterNonMemberUser, err := types.NewUser("test-requester-non-member")
		require.NoError(t, err)

		requesterNonMemberUser.SetRoles([]string{requesterRole.GetName()})

		_, err = authClient.UpsertUser(ctx, requesterNonMemberUser)
		require.NoError(t, err)

		req, err := createAccessRequest(ctx, authClient, ui.AccessRequestParameters{
			Reason:      "Request expecting membership reviewer NOT to be found",
			RequestKind: types.AccessRequestKind_SHORT_TERM,
			ResourceIDs: []ui.ResourceID{{Name: node.GetName(), Kind: types.KindNode}},
			Roles:       []string{accessRole.GetName()},
			DryRun:      true,
		}, requesterNonMemberUser.GetName())

		require.NoError(t, err)
		require.Empty(t, req.SuggestedReviewers)
	})

	t.Run("user who is a member gets owner as suggested reviewer", func(t *testing.T) {
		requesterMemberUser, err := types.NewUser("test-requester-member")
		require.NoError(t, err)

		_, err = authClient.UpsertUser(ctx, requesterMemberUser)
		require.NoError(t, err)

		requesterACLMember, err := accesslist.NewAccessListMember(
			header.Metadata{Name: requesterMemberUser.GetName()},
			accesslist.AccessListMemberSpec{
				Name:           requesterMemberUser.GetName(),
				AccessList:     testAccessList.GetName(),
				MembershipKind: accesslist.MembershipKindUser,
				Joined:         s.clock.Now().Add(-1 * time.Hour),
				AddedBy:        ownerUser.GetName(),
			},
		)
		require.NoError(t, err)

		_, err = accessListClient.UpsertAccessListMember(ctx, requesterACLMember)
		require.NoError(t, err)

		// Applies UserLoginState (i.e. the granted roles from access list) to user.
		err = s.testAuthServer.AuthServer.AuthServer.CallLoginHooks(ctx, requesterMemberUser)
		require.NoError(t, err)

		req, err := createAccessRequest(ctx, authClient, ui.AccessRequestParameters{
			Reason:      "Request expecting membership reviewer to be found",
			RequestKind: types.AccessRequestKind_SHORT_TERM,
			ResourceIDs: []ui.ResourceID{{Name: node.GetName(), Kind: types.KindNode}},
			Roles:       []string{accessRole.GetName()},
			DryRun:      true,
		}, requesterMemberUser.GetName())

		require.NoError(t, err)
		require.ElementsMatch(t, req.SuggestedReviewers, []string{ownerUser.GetName()})
	})

	t.Run("user who is a member of long-term access list gets owner suggested for long-term requests", func(t *testing.T) {
		longTermOwnerUser, err := types.NewUser("test-long-term-owner")
		require.NoError(t, err)

		_, err = authClient.UpsertUser(ctx, longTermOwnerUser)
		require.NoError(t, err)

		longTermReviewerRole, err := authtest.CreateRole(ctx, authClient, "long-term-reviewer-role", types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{accessRole.GetName()},
				},
			},
		})
		require.NoError(t, err)

		_, err = authClient.UpsertRole(ctx, longTermReviewerRole)
		require.NoError(t, err)

		longTermAccessList, err := accesslist.NewAccessList(
			header.Metadata{Name: "long-term"},
			accesslist.Spec{
				Title:  "Long Term Test",
				Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
				Owners: []accesslist.Owner{{Name: longTermOwnerUser.GetName()}},
				Grants: accesslist.Grants{
					Roles: []string{accessRole.GetName()},
				},
				OwnerGrants: accesslist.Grants{
					Roles: []string{longTermReviewerRole.GetName()},
				},
			},
		)
		require.NoError(t, err)

		_, err = accessListClient.UpsertAccessList(ctx, longTermAccessList)
		require.NoError(t, err)

		longTermMemberUser, err := types.NewUser("test-long-term-member")
		require.NoError(t, err)

		longTermMemberUser.SetRoles([]string{requesterRole.GetName()})

		_, err = authClient.UpsertUser(ctx, longTermMemberUser)
		require.NoError(t, err)

		shortTermMember, err := accesslist.NewAccessListMember(
			header.Metadata{Name: longTermMemberUser.GetName()},
			accesslist.AccessListMemberSpec{
				Name:           longTermMemberUser.GetName(),
				AccessList:     testAccessList.GetName(),
				MembershipKind: accesslist.MembershipKindUser,
				Joined:         s.clock.Now().Add(-1 * time.Hour),
				AddedBy:        ownerUser.GetName(),
			},
		)
		require.NoError(t, err)

		_, err = accessListClient.UpsertAccessListMember(ctx, shortTermMember)
		require.NoError(t, err)

		longTermMember, err := accesslist.NewAccessListMember(
			header.Metadata{Name: longTermMemberUser.GetName()},
			accesslist.AccessListMemberSpec{
				Name:           longTermMemberUser.GetName(),
				AccessList:     longTermAccessList.GetName(),
				MembershipKind: accesslist.MembershipKindUser,
				Joined:         s.clock.Now().Add(-1 * time.Hour),
				AddedBy:        longTermOwnerUser.GetName(),
			},
		)
		require.NoError(t, err)

		_, err = accessListClient.UpsertAccessListMember(ctx, longTermMember)
		require.NoError(t, err)

		req, err := createAccessRequest(ctx, authClient, ui.AccessRequestParameters{
			Reason:      "Request expecting long term suggestion",
			RequestKind: types.AccessRequestKind_LONG_TERM,
			ResourceIDs: []ui.ResourceID{{Name: node.GetName(), Kind: types.KindNode}},
			Roles:       []string{accessRole.GetName()},
			DryRun:      true,
		}, longTermMemberUser.GetName())

		require.NoError(t, err)
		require.ElementsMatch(t, req.SuggestedReviewers, []string{longTermOwnerUser.GetName()})
	})

	t.Run("user who is a member of long-term access list gets owner suggested for role-only requests", func(t *testing.T) {
		longTermRoleOwner, err := types.NewUser("long-term-role-owner")
		require.NoError(t, err)

		_, err = authClient.UpsertUser(ctx, longTermRoleOwner)
		require.NoError(t, err)

		longTermRoleReviewerRole, err := authtest.CreateRole(ctx, authClient, "long-term-role-reviewer-role", types.RoleSpecV6{
			Allow: types.RoleConditions{
				ReviewRequests: &types.AccessReviewConditions{
					Roles: []string{accessRole.GetName()},
				},
			},
		})
		require.NoError(t, err)

		longTermAccessList, err := accesslist.NewAccessList(
			header.Metadata{Name: "long-term-role"},
			accesslist.Spec{
				Title:  "Long term role request",
				Audit:  accesslist.Audit{NextAuditDate: s.clock.Now().Add(24 * time.Hour)},
				Owners: []accesslist.Owner{{Name: longTermRoleOwner.GetName()}},
				Grants: accesslist.Grants{
					Roles: []string{accessRole.GetName()},
				},
				OwnerGrants: accesslist.Grants{
					Roles: []string{longTermRoleReviewerRole.GetName()},
				},
			},
		)
		require.NoError(t, err)

		_, err = accessListClient.UpsertAccessList(ctx, longTermAccessList)
		require.NoError(t, err)

		longTermMemberUser, err := types.NewUser("long-term-member-user")
		require.NoError(t, err)

		longTermMemberUser.SetRoles([]string{requesterRole.GetName()})

		_, err = authClient.UpsertUser(ctx, longTermMemberUser)
		require.NoError(t, err)

		member, err := accesslist.NewAccessListMember(
			header.Metadata{Name: longTermMemberUser.GetName()},
			accesslist.AccessListMemberSpec{
				Name:           longTermMemberUser.GetName(),
				AccessList:     longTermAccessList.GetName(),
				MembershipKind: accesslist.MembershipKindUser,
				Joined:         s.clock.Now().Add(-1 * time.Hour),
				AddedBy:        longTermRoleOwner.GetName(),
			},
		)
		require.NoError(t, err)

		_, err = accessListClient.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)

		req, err := createAccessRequest(ctx, authClient, ui.AccessRequestParameters{
			Reason:      "Request role-only expecting long-term suggestion",
			RequestKind: types.AccessRequestKind_LONG_TERM,
			Roles:       []string{accessRole.GetName()},
			DryRun:      true,
		}, longTermMemberUser.GetName())

		require.NoError(t, err)
		require.ElementsMatch(t, req.SuggestedReviewers, []string{longTermRoleOwner.GetName()})
	})
}

func TestPromoteAccessRequest_NoMemberOnFailure(t *testing.T) {
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			AdvancedAccessWorkflows: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
		GenerateAccessRequestPromotionsFn: accessrequest.GenerateAccessRequestPromotions,
	}
	modulestest.SetTestModules(t, *testModules)

	ctx := t.Context()
	s := newWebSuite(t,
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
	)

	authServer := s.testAuthServer.AuthServer.AuthServer
	authClient := s.newAdminAuthClient(s.ctx, t)
	accessListClient := authClient.AccessListClient()

	const (
		requesterUserName       = "requester"
		ownerWithReviewUserName = "owner-with-review"
		ownerNoReviewUserName   = "owner-no-review"
		nodeName                = "node"
	)

	// create a node for requesting
	node, err := types.NewServerWithLabels(
		nodeName,
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"name": nodeName},
	)
	require.NoError(t, err)
	_, err = authServer.UpsertNode(ctx, node)
	require.NoError(t, err)

	upsertRole := func(roleName string, allow types.RoleConditions) {
		_, err := authtest.CreateRole(context.Background(), authServer, roleName, types.RoleSpecV6{
			Allow: allow,
		})
		require.NoError(t, err)
	}

	// role granting access to node
	upsertRole("access", types.RoleConditions{
		NodeLabels: types.Labels{
			"name": []string{nodeName},
		},
	})
	// role for requester
	upsertRole("requesterRole", types.RoleConditions{
		Request: &types.AccessRequestConditions{
			SearchAsRoles: []string{"access"},
		},
	})
	// role for owner who can review requests
	upsertRole("ownerWithReviewRole", types.RoleConditions{
		ReviewRequests: &types.AccessReviewConditions{
			Roles: []string{"access"},
		},
	})
	// role for owner who can't review requests
	upsertRole("ownerNoReviewRole", types.RoleConditions{})

	createUserWithOpts(t, s, requesterUserName, withRoles("requesterRole"), withPassword())
	createUserWithOpts(t, s, ownerWithReviewUserName, withRoles("ownerWithReviewRole"), withPassword())
	createUserWithOpts(t, s, ownerNoReviewUserName, withRoles("ownerNoReviewRole"), withPassword())

	// access list owned by both owner users
	accessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "test-access-list"},
		accesslist.Spec{
			Title: "Test Access List",
			Audit: accesslist.Audit{NextAuditDate: s.clock.Now()},
			Owners: []accesslist.Owner{
				{Name: ownerWithReviewUserName, Description: "owner with review perms"},
				{Name: ownerNoReviewUserName, Description: "owner without review perms"},
			},
			MembershipRequires: accesslist.Requires{},
			Grants:             accesslist.Grants{Roles: []string{"access"}},
		})
	require.NoError(t, err)
	accessList, err = accessListClient.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	createPendingAccessRequest := func(t *testing.T, username string) types.AccessRequest {
		t.Helper()
		accessRequest, err := services.NewAccessRequestWithResources(username, []string{"access"}, []types.ResourceAccessID{
			{Id: types.ResourceID{Name: nodeName, Kind: types.KindNode}},
		})
		require.NoError(t, err)
		accessRequest, err = authClient.CreateAccessRequestV2(ctx, accessRequest)
		require.NoError(t, err)
		return accessRequest
	}

	ownerWithReviewWebPack := s.newAuthWebPack(t, ownerWithReviewUserName, skipUserCreation())
	ownerNoReviewWebPack := s.newAuthWebPack(t, ownerNoReviewUserName, skipUserCreation())

	t.Run("owner with reviewer permission should add member on success", func(t *testing.T) {
		accessRequest := createPendingAccessRequest(t, requesterUserName)

		// promotion should succeed
		endpoint := ownerWithReviewWebPack.clt.Endpoint("enterprise", "accessrequest", accessRequest.GetName(), "promote")
		_, err = ownerWithReviewWebPack.clt.PostJSON(s.ctx, endpoint, &accessRequestPromoteParameters{
			Reason:         "promotion reason",
			AccessListName: accessList.GetName(),
		})
		require.NoError(t, err)

		// verify requester was added
		_, err = accessListClient.GetAccessListMember(ctx, accessList.GetName(), requesterUserName)
		require.NoError(t, err)

		t.Cleanup(func() {
			err = accessListClient.DeleteAccessListMember(ctx, accessList.GetName(), requesterUserName)
			require.NoError(t, err)
		})
	})

	t.Run("owner without reviewer permission should not add member on failure", func(t *testing.T) {
		accessRequest := createPendingAccessRequest(t, "requester")

		// attempt to promote should fail because user lacks reviewer permission
		endpoint := ownerNoReviewWebPack.clt.Endpoint("enterprise", "accessrequest", accessRequest.GetName(), "promote")
		_, err = ownerNoReviewWebPack.clt.PostJSON(s.ctx, endpoint, &accessRequestPromoteParameters{
			Reason:         "promotion reason",
			AccessListName: accessList.GetName(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "cannot submit reviews")

		// verify requester was not added
		_, err = accessListClient.GetAccessListMember(ctx, accessList.GetName(), requesterUserName)
		require.True(t, trace.IsNotFound(err), "member should not have been added when promotion failed, got error: %v", err)
	})

	t.Run("denied access request should not add member on failure", func(t *testing.T) {
		accessRequest := createPendingAccessRequest(t, requesterUserName)

		// deny request using a reviewer
		reviewEndpoint := ownerWithReviewWebPack.clt.Endpoint("enterprise", "accessrequest")
		_, err = ownerWithReviewWebPack.clt.PutJSON(s.ctx, reviewEndpoint, &ui.AccessRequestParameters{
			ID:     accessRequest.GetName(),
			State:  "DENIED",
			Reason: "denied for testing",
		})
		require.NoError(t, err)

		// try to promote the denied request, should fail
		promoteEndpoint := ownerWithReviewWebPack.clt.Endpoint("enterprise", "accessrequest", accessRequest.GetName(), "promote")
		_, err = ownerWithReviewWebPack.clt.PostJSON(s.ctx, promoteEndpoint, &accessRequestPromoteParameters{
			Reason:         "promotion reason",
			AccessListName: accessList.GetName(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, fmt.Sprintf("user %q has already reviewed this request", ownerWithReviewUserName))

		// verify requester was not added
		_, err = accessListClient.GetAccessListMember(ctx, accessList.GetName(), requesterUserName)
		require.True(t, trace.IsNotFound(err), "member should not have been added when promotion of denied request failed, got error: %v", err)
	})
}
