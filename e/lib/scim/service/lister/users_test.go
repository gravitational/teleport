package lister

import (
	"context"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

func TestUserLister_ListResources_Basic(t *testing.T) {
	ctx := context.Background()

	users := []*types.UserV2{
		mockUser(t, "alice"),
		mockUser(t, "bob"),
		mockUser(t, "carol"),
	}

	svc := &fakeUserService{users: users}
	lister := &UserLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(ctx context.Context, u types.User) bool {
			return true // include all
		},
		UserToResource: func(u types.User) (*scimpb.Resource, error) {
			return &scimpb.Resource{Id: u.GetName()}, nil
		},
	}

	req := &scimpb.ListSCIMResourcesRequest{
		Filter: "",
		Page: &scimpb.Page{
			StartIndex: 1,
			Count:      2,
		},
	}

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 2)
	require.Equal(t, "alice", resp.Resources[0].Id)
	require.Equal(t, "bob", resp.Resources[1].Id)
	require.Equal(t, int32(3), resp.TotalResults)
}

func TestUserLister_ListResources_Predicate(t *testing.T) {
	ctx := context.Background()

	users := []*types.UserV2{
		mockUser(t, "alice"),
		mockUser(t, "bob"),
	}

	svc := &fakeUserService{users: users}
	lister := &UserLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(ctx context.Context, u types.User) bool {
			return u.GetName() == "bob"
		},
		UserToResource: func(u types.User) (*scimpb.Resource, error) {
			return &scimpb.Resource{Id: u.GetName()}, nil
		},
	}

	req := &scimpb.ListSCIMResourcesRequest{
		Filter: "",
		Page: &scimpb.Page{
			StartIndex: 1,
			Count:      10,
		},
	}

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Equal(t, "bob", resp.Resources[0].Id)
	require.Equal(t, int32(1), resp.TotalResults)
}

func TestUserLister_ListResources_FilterMatch(t *testing.T) {
	ctx := context.Background()

	users := []*types.UserV2{mockUser(t, "admin")}

	svc := &fakeUserService{users: users}
	lister := &UserLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(ctx context.Context, u types.User) bool { return true },
		UserToResource: func(u types.User) (*scimpb.Resource, error) {
			return &scimpb.Resource{Id: u.GetName()}, nil
		},
	}

	req := &scimpb.ListSCIMResourcesRequest{
		Filter: `userName eq "admin"`,
		Page:   &scimpb.Page{StartIndex: 1, Count: 5},
	}

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)
	require.Equal(t, "admin", resp.Resources[0].Id)
}

func TestUserLister_ListResources_FilterNoMatch(t *testing.T) {
	ctx := context.Background()

	users := []*types.UserV2{mockUser(t, "dev")}

	svc := &fakeUserService{users: users}
	lister := &UserLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(ctx context.Context, u types.User) bool { return true },
		UserToResource: func(u types.User) (*scimpb.Resource, error) {
			return &scimpb.Resource{Id: u.GetName()}, nil
		},
	}

	req := &scimpb.ListSCIMResourcesRequest{
		Filter: `userName eq "qa"`,
		Page:   &scimpb.Page{StartIndex: 1, Count: 10},
	}

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
	require.Equal(t, int32(0), resp.TotalResults)
}

func TestUserLister_ListResources_EmptyUsers(t *testing.T) {
	ctx := context.Background()

	svc := &fakeUserService{users: []*types.UserV2{}}
	lister := &UserLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(ctx context.Context, u types.User) bool { return true },
		UserToResource: func(u types.User) (*scimpb.Resource, error) {
			return &scimpb.Resource{Id: u.GetName()}, nil
		},
	}

	req := &scimpb.ListSCIMResourcesRequest{
		Filter: "",
		Page:   &scimpb.Page{StartIndex: 1, Count: 10},
	}

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Empty(t, resp.Resources)
	require.Equal(t, int32(0), resp.TotalResults)
}

// mockUser creates a new types.User with the given name.
func mockUser(t *testing.T, name string) *types.UserV2 {
	u, err := types.NewUser(name)
	require.NoError(t, err)
	return u.(*types.UserV2)
}

// fakeUserService implements minimal AccessPoint for testing.
type fakeUserService struct {
	common.AccessPoint
	users []*types.UserV2
}

func (s *fakeUserService) ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error) {
	return nil, "", nil
}

func (s *fakeUserService) GetAccessListMember(context.Context, string, string) (*accesslist.AccessListMember, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (s *fakeUserService) ListAccessListMembers(context.Context, string, int, string) ([]*accesslist.AccessListMember, string, error) {
	return nil, "", nil
}

func (s *fakeUserService) GetAccessList(context.Context, string) (*accesslist.AccessList, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (s *fakeUserService) ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error) {
	return nil, "", nil
}

func (s *fakeUserService) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	start := 0
	if req.PageToken != "" {
		var err error
		start, err = strconv.Atoi(req.PageToken)
		if err != nil {
			return nil, trace.BadParameter("invalid page token %q", req.PageToken)
		}
	}
	end := min(start+int(req.PageSize), len(s.users))
	resp := &userspb.ListUsersResponse{
		Users: s.users[start:end],
	}
	if end < len(s.users) {
		resp.NextPageToken = strconv.Itoa(end)
	}

	return resp, nil
}
