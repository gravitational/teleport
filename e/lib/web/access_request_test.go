package web

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
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

	request := accessRequestParameters{
		Reason: "some reason",
	}

	// Test with empty role requests, wild card is used.
	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.NoError(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, types.RequestState_PENDING.String(), req.State)

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

	request := accessRequestParameters{
		Reason:      "some reason",
		ResourceIDs: []ui.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}},
	}

	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.NoError(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, types.RequestState_PENDING.String(), req.State)
}

type mockAuthClient struct {
	auth.ClientI
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

func (m *mockClusterClientProvider) UserClientForCluster(ctx context.Context, clusterName string) (auth.ClientI, error) {
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
				require.Equal(t, "", res.Resources[1].Details.FriendlyName)
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
				require.Equal(t, "", req.Resources[1].Details.FriendlyName)
				require.Equal(t, "test-kube-1", req.Resources[2].ID.Name)
				require.Equal(t, "", req.Resources[2].Details.FriendlyName)
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			req, err := services.NewAccessRequestWithResources("alice", tc.requestedRoles, tc.requestedResources)
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

	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		roleBasedReq1, err := services.NewAccessRequest("baz", []string{"bar"}...)
		require.NoError(t, err)
		roleBasedReq1.SetState(types.RequestState_NONE)

		roleBasedReq2, err := services.NewAccessRequest("foz", []string{"foo"}...)
		require.NoError(t, err)
		roleBasedReq2.SetState(types.RequestState_APPROVED)

		searchBasedReq, err := services.NewAccessRequestWithResources("bar", nil, []types.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}})
		require.NoError(t, err)

		return []types.AccessRequest{roleBasedReq1, roleBasedReq2, searchBasedReq}, nil
	}

	plugin, err := NewPlugin(Config{})
	require.NoError(t, err)

	// Test request state set to NONE, is not returned.
	reqs, err := plugin.getAccessRequests(context.Background(), m, types.AccessRequestFilter{})
	require.NoError(t, err)
	require.Len(t, reqs, 2)
	require.Equal(t, types.RequestState_APPROVED.String(), reqs[0].State)
	require.Equal(t, types.RequestState_PENDING.String(), reqs[1].State)
	require.Equal(t, []ui.Resource{{ID: ui.ResourceID{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}}}, reqs[1].Resources)
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

	reviewSubmission := accessRequestParameters{
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

type mockedAccessRequestAPIGetter struct {
	mockCreateAccessRequest func(ctx context.Context, req types.AccessRequest) error
	mockGetAccessRequests   func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	mockSubmitAccessReview  func(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
}

func (m *mockedAccessRequestAPIGetter) GetAccessRequestAllowedPromotions(_ context.Context, _ types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
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

func (m *mockedAccessRequestAPIGetter) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
	if m.mockSubmitAccessReview != nil {
		return m.mockSubmitAccessReview(ctx, params)
	}

	return nil, trace.NotImplemented("mockSubmitAccessReview not implemented")
}

type fakeBuildModule struct {
	modules.TestModules
}

func (f *fakeBuildModule) GenerateAccessRequestPromotions(ctx context.Context, accessListGetter modules.AccessResourcesGetter, accessReq types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return accessrequest.GenerateAccessRequestPromotions(ctx, accessListGetter, accessReq)
}

func TestSuggestAccessLists(t *testing.T) {
	modules.SetTestModules(t, &fakeBuildModule{
		TestModules: modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				IdentityGovernanceSecurity: true,
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)
	authServer := s.testAuthServer.AuthServer.AuthServer

	// create requester, access and godmode roles
	const requesterRoleName = "requester"
	_, err := auth.CreateRole(ctx, authServer, requesterRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, authServer, "access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"name": []string{"node"},
			},
		},
	})
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, authServer, "godmode", types.RoleSpecV6{})
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
	user, err := types.NewUser("reviewer")
	require.NoError(t, err)
	require.NoError(t, err)
	user.SetRoles([]string{requesterRoleName})
	user.SetTraits(trait.Traits{"preferred_drink": []string{"fanta"}})
	_, err = authServer.UpsertUser(ctx, user)
	require.NoError(t, err)
	err = authServer.UpsertPassword(user.GetName(), []byte(s.testPassword()))
	require.NoError(t, err)

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
		[]types.ResourceID{
			{
				Name: nodeName,
				Kind: types.KindNode,
			},
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
	modules.SetTestModules(t, &fakeBuildModule{
		TestModules: modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				AdvancedAccessWorkflows:    true,
				IdentityGovernanceSecurity: true,
			},
		},
	})

	ctx := context.Background()
	s := newWebSuite(t)

	// Create users
	s.createUser(t, "reviewer", "reviewer", s.testPassword(), s.testOtpSecret())
	s.createUser(t, "requester", "requester", s.testPassword(), s.testOtpSecret())

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

		_, err = authClient.UpsertNode(ctx, node)
		require.NoError(t, err)

		return node
	}

	// create a node, so we can request access to it
	node := createNode()

	createAccessRequest := func() types.AccessRequest {
		// create an access request for reviewer to request access to the "access" role
		accessRequest, err := services.NewAccessRequestWithResources("requester", []string{"access"}, []types.ResourceID{
			{
				Name: node.GetName(),
				Kind: types.KindNode,
			},
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
		_, err := auth.CreateRole(context.Background(), authClient, roleName, types.RoleSpecV6{
			Allow: allow,
		})
		require.NoError(t, err)
	}
	assignRole := func(userName string, role string) {
		user, err := authClient.GetUser(ctx, userName, false)
		require.NoError(t, err)
		user.SetRoles([]string{role})
		_, err = authClient.UpsertUser(ctx, user)
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

	// assign roles to users
	assignRole("reviewer", "reviewerRole")
	assignRole("requester", "requesterRole")

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
