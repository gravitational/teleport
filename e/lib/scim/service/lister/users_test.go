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
			return scimpb.Resource_builder{Id: u.GetName()}.Build(), nil
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: "",
		Page: scimpb.Page_builder{
			StartIndex: 1,
			Count:      2,
		}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetResources(), 2)
	require.Equal(t, "alice", resp.GetResources()[0].GetId())
	require.Equal(t, "bob", resp.GetResources()[1].GetId())
	require.Equal(t, int32(3), resp.GetTotalResults())
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
			return scimpb.Resource_builder{Id: u.GetName()}.Build(), nil
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: "",
		Page: scimpb.Page_builder{
			StartIndex: 1,
			Count:      10,
		}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetResources(), 1)
	require.Equal(t, "bob", resp.GetResources()[0].GetId())
	require.Equal(t, int32(1), resp.GetTotalResults())
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
			return scimpb.Resource_builder{Id: u.GetName()}.Build(), nil
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: `userName eq "admin"`,
		Page:   scimpb.Page_builder{StartIndex: 1, Count: 5}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetResources(), 1)
	require.Equal(t, "admin", resp.GetResources()[0].GetId())
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
			return scimpb.Resource_builder{Id: u.GetName()}.Build(), nil
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: `userName eq "qa"`,
		Page:   scimpb.Page_builder{StartIndex: 1, Count: 10}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Empty(t, resp.GetResources())
	require.Equal(t, int32(0), resp.GetTotalResults())
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
			return scimpb.Resource_builder{Id: u.GetName()}.Build(), nil
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: "",
		Page:   scimpb.Page_builder{StartIndex: 1, Count: 10}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Empty(t, resp.GetResources())
	require.Equal(t, int32(0), resp.GetTotalResults())
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
	if req.GetPageToken() != "" {
		var err error
		start, err = strconv.Atoi(req.GetPageToken())
		if err != nil {
			return nil, trace.BadParameter("invalid page token %q", req.GetPageToken())
		}
	}
	end := min(start+int(req.GetPageSize()), len(s.users))
	resp := userspb.ListUsersResponse_builder{
		Users: s.users[start:end],
	}.Build()
	if end < len(s.users) {
		resp.SetNextPageToken(strconv.Itoa(end))
	}

	return resp, nil
}
