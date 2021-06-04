package web

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateAccessRequest(t *testing.T) {
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

func TestGetAccessRequest(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		require.Equal(t, filter.ID, "1234")

		req, err := services.NewAccessRequest("foo", []string{"*"}...)
		require.Nil(t, err)
		return []types.AccessRequest{req}, nil
	}

	_, err := getAccessRequest(context.Background(), m, "1234")
	require.Nil(t, err)

	// Test empty request id.
	req, err := getAccessRequest(context.Background(), m, "")
	require.Nil(t, req)
	require.True(t, trace.IsBadParameter(err))

	// Test no request found.
	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		return []types.AccessRequest{}, nil
	}
	req, err = getAccessRequest(context.Background(), m, "1234")
	require.Nil(t, req)
	require.True(t, trace.IsNotFound(err))
}

func TestGetAccessRequests(t *testing.T) {
	m := &mockedAccessRequestAPIGetter{}

	m.mockGetAccessRequests = func(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
		req, err := services.NewAccessRequest("baz", []string{"bar"}...)
		require.Nil(t, err)
		req.SetState(types.RequestState_NONE)

		req2, err := services.NewAccessRequest("foz", []string{"foo"}...)
		require.Nil(t, err)

		return []types.AccessRequest{req, req2}, nil
	}

	plugin, err := NewPlugin(Config{})
	require.Nil(t, err)

	// Test request state set to NONE, is not returned.
	reqs, err := plugin.getAccessRequests(context.Background(), m, types.AccessRequestFilter{})
	require.Nil(t, err)
	require.Len(t, reqs, 1)
	require.Equal(t, reqs[0].State, types.RequestState_PENDING.String())
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
