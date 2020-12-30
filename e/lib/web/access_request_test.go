package web

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateAccessRequest(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}
	var createdReq services.AccessRequest
	m.mockGetAccessRequests = func(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
		return []services.AccessRequest{createdReq}, nil
	}

	m.mockCreateAccessRequest = func(ctx context.Context, req services.AccessRequest) error {
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
	require.Equal(t, req.State, services.RequestState_PENDING.String())

	// Test with specific roles requested.
	request.Roles = []string{"role1", "role2"}
	m.mockCreateAccessRequest = func(ctx context.Context, req services.AccessRequest) error {
		require.ElementsMatch(t, req.GetRoles(), []string{"role1", "role2"})
		return nil
	}

	_, err = createAccessRequest(context.Background(), m, request, "userFoo")
	require.Nil(t, err)
}

func TestGetAccessRequest(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	var requestID string
	m.mockGetAccessRequests = func(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
		req, err := services.NewAccessRequest(filter.User, []string{"bar"}...)
		require.Nil(t, err)

		req.SetState(services.RequestState_APPROVED)
		req.SetResolveReason("resolved reason")
		requestID = req.GetMetadata().Name

		return []services.AccessRequest{req}, nil
	}

	req, err := getAccessRequest(context.Background(), m, "1234", "foo")
	require.Nil(t, err)
	require.Equal(t, req.ID, requestID)
	require.Equal(t, req.State, services.RequestState_APPROVED.String())
	require.Equal(t, req.ResolveReason, "resolved reason")

	// Test empty request id.
	req, err = getAccessRequest(context.Background(), m, "", "fail")
	require.Nil(t, req)
	require.True(t, trace.IsBadParameter(err))

	// Test no request found.
	m.mockGetAccessRequests = func(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
		return []services.AccessRequest{}, nil
	}
	req, err = getAccessRequest(context.Background(), m, "1234", "foo")
	require.Nil(t, req)
	require.True(t, trace.IsNotFound(err))
}

func TestGetAccessRequests(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	m.mockGetAccessRequests = func(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
		req, err := services.NewAccessRequest("baz", []string{"bar"}...)
		require.Nil(t, err)
		req.SetState(services.RequestState_NONE)

		req2, err := services.NewAccessRequest("foz", []string{"foo"}...)
		require.Nil(t, err)

		return []services.AccessRequest{req, req2}, nil
	}

	// Test request state set to NONE, is not returned.
	reqs, err := getAccessRequests(context.Background(), m, services.AccessRequestFilter{})
	require.Nil(t, err)

	require.Len(t, reqs, 1)
	require.Equal(t, reqs[0].State, services.RequestState_PENDING.String())
}

type mockedAccessRequestAPIGetter struct {
	mockCreateAccessRequest func(ctx context.Context, req services.AccessRequest) error
	mockGetAccessRequests   func(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error)
}

func (m *mockedAccessRequestAPIGetter) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	if m.mockCreateAccessRequest != nil {
		return m.mockCreateAccessRequest(ctx, req)
	}

	return trace.NotImplemented("mockCreateAccessRequest not implemented")
}

func (m *mockedAccessRequestAPIGetter) GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
	if m.mockGetAccessRequests != nil {
		return m.mockGetAccessRequests(ctx, filter)
	}

	return nil, trace.NotImplemented("mockGetAccessRequests not implemented")
}
