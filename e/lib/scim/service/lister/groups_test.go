package lister

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// TestGroupLister_ListResources tests basic listing, filtering, and pagination of GroupLister.
func TestGroupLister_ListResources(t *testing.T) {
	ctx := context.Background()

	// Sample access lists
	acl1 := mockAccessList("engineering", "Engineering Team")
	acl2 := mockAccessList("hr", "Human Resources")
	acl3 := mockAccessList("sales", "Sales Department")

	mockAccessLists := []*accesslist.AccessList{acl1, acl2, acl3}

	svc := &fakeAccessListService{lists: mockAccessLists}
	// Create GroupLister
	lister := &GroupLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(acl *accesslist.AccessList) bool {
			return true // Include all
		},
		AccessListToResource: func(acl *accesslist.AccessList) (*scimpb.Resource, error) {
			return conv.AccessListToResource(acl, nil)
		},
	}

	// Test with no filter, get first 2 items
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
	require.Equal(t, "engineering", resp.GetResources()[0].GetId())
	require.Equal(t, "hr", resp.GetResources()[1].GetId())
	require.Equal(t, int32(3), resp.GetTotalResults())
}

// TestGroupLister_PredicateExcludes ensures Predicate properly excludes items.
func TestGroupLister_PredicateExcludes(t *testing.T) {
	ctx := context.Background()

	acl := mockAccessList("excluded", "Do Not Show")

	svc := &fakeAccessListService{lists: []*accesslist.AccessList{acl}}
	lister := &GroupLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(acl *accesslist.AccessList) bool {
			return false // exclude all
		},
		AccessListToResource: func(acl *accesslist.AccessList) (*scimpb.Resource, error) {
			return conv.AccessListToResource(acl, nil)
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Page: scimpb.Page_builder{StartIndex: 1, Count: 10}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Empty(t, resp.GetResources())
	require.Equal(t, int32(0), resp.GetTotalResults())
}

// TestGroupLister_FilterMatches verifies SCIM filter is respected.
func TestGroupLister_FilterMatches(t *testing.T) {
	ctx := context.Background()

	acl := mockAccessList("team1", "Team One")

	svc := &fakeAccessListService{lists: []*accesslist.AccessList{acl}}
	lister := &GroupLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(*accesslist.AccessList) bool { return true },
		AccessListToResource: func(acl *accesslist.AccessList) (*scimpb.Resource, error) {
			return conv.AccessListToResource(acl, nil)
		},
	}

	// Filter only groups with title "Team One"
	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: `displayName eq "Team One"`,
		Page:   scimpb.Page_builder{StartIndex: 1, Count: 1}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.GetResources(), 1)
	require.Equal(t, "team1", resp.GetResources()[0].GetId())
}

// TestGroupLister_FilterNoMatch ensures no match from filter returns empty.
func TestGroupLister_FilterNoMatch(t *testing.T) {
	ctx := context.Background()

	acl := mockAccessList("group", "Group")

	svc := &fakeAccessListService{lists: []*accesslist.AccessList{acl}}
	lister := &GroupLister{
		Config: common.Config{
			AccessPoint: svc,
			Backend:     svc,
		},
		Predicate: func(*accesslist.AccessList) bool { return true },
		AccessListToResource: func(acl *accesslist.AccessList) (*scimpb.Resource, error) {
			return conv.AccessListToResource(acl, nil)
		},
	}

	req := scimpb.ListSCIMResourcesRequest_builder{
		Filter: `displayName eq "Nonexistent"`,
		Page:   scimpb.Page_builder{StartIndex: 1, Count: 5}.Build(),
	}.Build()

	resp, err := lister.ListResources(ctx, req)
	require.NoError(t, err)
	require.Empty(t, resp.GetResources())
}

// fakeAccessListService mocks AccessLists for group lister tests.
type fakeAccessListService struct {
	common.AccessPoint
	lists []*accesslist.AccessList
}

func (f *fakeAccessListService) ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error) {
	return f.lists, "", nil
}

func (f *fakeAccessListService) GetAccessListMember(context.Context, string, string) (*accesslist.AccessListMember, error) {
	return nil, nil
}

func (f *fakeAccessListService) ListAccessListMembers(context.Context, string, int, string) ([]*accesslist.AccessListMember, string, error) {
	return nil, "", nil
}

func (f *fakeAccessListService) GetAccessList(context.Context, string) (*accesslist.AccessList, error) {
	return nil, nil
}

func (f *fakeAccessListService) ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error) {
	return nil, "", nil
}

// mockAccessList creates a simple access list with the given name and title.
func mockAccessList(name, title string) *accesslist.AccessList {
	return &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{
				Name: name,
			},
		},
		Spec: accesslist.Spec{
			Title: title,
		},
	}
}
