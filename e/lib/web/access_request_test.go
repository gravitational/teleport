package web

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateAccessRequest_RoleBased(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}
	var createdReq types.AccessRequest
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{createdReq}, nil
	}

	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		createdReq = req
		require.Equal(t, req.GetUser(), "userFoo")
		require.Equal(t, req.GetRoles(), []string{"*"})
		require.Equal(t, req.GetRequestReason(), "some reason")
		return nil
	}

	request := accessRequestParameters{
		Reason: "some reason",
	}

	// Test with empty role requests, wild card is used.
	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.Nil(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, req.State, types.RequestState_PENDING.String())

	// Test with specific roles requested.
	request.Roles = []string{"role1", "role2"}
	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		require.ElementsMatch(t, req.GetRoles(), []string{"role1", "role2"})
		return nil
	}

	_, err = createAccessRequest(context.Background(), m, request, "userFoo")
	require.Nil(t, err)
}

func TestCreateAccessRequest_SearchBased(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}
	var createdReq types.AccessRequest
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{createdReq}, nil
	}

	m.mockCreateAccessRequest = func(ctx context.Context, req types.AccessRequest) error {
		createdReq = req
		require.Equal(t, req.GetUser(), "userFoo")
		require.Empty(t, req.GetRoles())
		require.Equal(t, req.GetRequestReason(), "some reason")
		require.Equal(t, req.GetRequestedResourceIDs(), []types.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}})
		return nil
	}

	request := accessRequestParameters{
		Reason:      "some reason",
		ResourceIDs: []ui.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}},
	}

	req, err := createAccessRequest(context.Background(), m, request, "userFoo")
	require.Nil(t, err)
	require.NotEmpty(t, req.ID)
	require.Equal(t, req.State, types.RequestState_PENDING.String())
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

func (m *mockClusterClientProvider) UserClientForCluster(clusterName string) (auth.ClientI, error) {
	return &mockAuthClient{
		resources: m.resourcesByCluster[clusterName],
	}, nil
}

type mockResource struct {
	types.ResourceWithLabels
	kind, name string
}

func (m *mockResource) GetKind() string {
	return m.kind
}

func (m *mockResource) GetName() string {
	return m.name
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
				require.Equal(t, "test-hostname", res.Resources[0].Details.Hostname)
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
				require.Equal(t, "test-hostname-1", res.Resources[0].Details.Hostname)
				require.Equal(t, "test-node-2", res.Resources[1].ID.Name)
				require.Equal(t, "test-hostname-2", res.Resources[1].Details.Hostname)
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
				require.Equal(t, "test-hostname-1", res.Resources[0].Details.Hostname)

				// test-node-2 should be included but the hostname should be missing
				require.Equal(t, "test-node-2", res.Resources[1].ID.Name)
				require.Equal(t, "", res.Resources[1].Details.Hostname)
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
				require.Equal(t, "test-hostname-1", req.Resources[0].Details.Hostname)
				require.Equal(t, "test-app-1", req.Resources[1].ID.Name)
				require.Equal(t, "", req.Resources[1].Details.Hostname)
				require.Equal(t, "test-kube-1", req.Resources[2].ID.Name)
				require.Equal(t, "", req.Resources[2].Details.Hostname)
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
		require.Nil(t, err)
		roleBasedReq1.SetState(types.RequestState_NONE)

		roleBasedReq2, err := services.NewAccessRequest("foz", []string{"foo"}...)
		require.Nil(t, err)
		roleBasedReq2.SetState(types.RequestState_APPROVED)

		searchBasedReq, err := services.NewAccessRequestWithResources("bar", nil, []types.ResourceID{{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}})
		require.Nil(t, err)

		return []types.AccessRequest{roleBasedReq1, roleBasedReq2, searchBasedReq}, nil
	}

	plugin, err := NewPlugin(Config{})
	require.Nil(t, err)

	// Test request state set to NONE, is not returned.
	reqs, err := plugin.getAccessRequests(context.Background(), m, types.AccessRequestFilter{})
	require.Nil(t, err)
	require.Len(t, reqs, 2)
	require.Equal(t, reqs[0].State, types.RequestState_APPROVED.String())
	require.Equal(t, reqs[1].State, types.RequestState_PENDING.String())
	require.Equal(t, reqs[1].Resources, []ui.Resource{{ID: ui.ResourceID{ClusterName: "test-cluster", Name: "test-name", Kind: "test-kind"}}})
}

func TestReviewAccessRequest(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	fakeReq, err := services.NewAccessRequest("foo", []string{"bar"}...)
	require.Nil(t, err)
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{fakeReq}, nil
	}

	m.mockSubmitAccessReview = func(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
		require.Equal(t, params.RequestID, fakeReq.GetMetadata().Name)
		require.Equal(t, params.Review.ProposedState, types.RequestState_DENIED)
		require.Equal(t, params.Review.Reason, "Not today")
		require.Len(t, params.Review.Roles, 0)
		return fakeReq, nil
	}

	reviewSubmission := accessRequestParameters{
		State:  "DENIED",
		Reason: "Not today",
		ID:     fakeReq.GetMetadata().Name,
	}

	_, err = reviewAccessRequest(context.Background(), m, reviewSubmission)
	require.Nil(t, err)

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

func (m *mockedAccessRequestAPIGetter) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	if m.mockCreateAccessRequest != nil {
		return m.mockCreateAccessRequest(ctx, req)
	}

	return trace.NotImplemented("mockCreateAccessRequest not implemented")
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
