package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/e/lib/web/ui"
)

func TestGetAccessLists(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()
	accessList1, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title:             "access list 1",
		Audit:             accesslist.Audit{Frequency: time.Hour},
		Owners:            []accesslist.Owner{{Name: "llama", Description: "llama desc"}},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	createdAccessList1, err := authClient.AccessListClient().UpsertAccessList(ctx, accessList1)
	require.NoError(t, err)

	m, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: accessList1.GetName(),
		}, accesslist.AccessListMemberSpec{
			AccessList: accessList1.GetName(),
			Name:       "llama",
			Joined:     time.Now(),
			Expires:    time.Now().Add(time.Hour),
			Reason:     "reason",
			AddedBy:    "admin",
		},
	)
	require.NoError(t, err)

	// Add one member, so we can verify the member counter.
	_, err = authClient.AccessListClient().UpsertAccessListMember(ctx, m)
	require.NoError(t, err)

	accesList2, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-2"}, accesslist.Spec{
		Title:             "access list 2",
		Audit:             accesslist.Audit{Frequency: time.Hour},
		Owners:            []accesslist.Owner{{Name: "alpaca", Description: "alpaca desc"}},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"editor"}},
	})
	require.NoError(t, err)
	createdAccessList2, err := authClient.AccessListClient().UpsertAccessList(ctx, accesList2)
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var accessListResp ui.AccessListsResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Len(t, accessListResp.AccessLists, 2)
	require.Equal(t, 1, *accessListResp.AccessLists[0].MembersCount)

	require.Empty(t, cmp.Diff(
		[]*accesslist.AccessList{accessListResp.AccessLists[0].AccessList, accessListResp.AccessLists[1].AccessList},
		[]*accesslist.AccessList{createdAccessList1, createdAccessList2},
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	))
}

func TestCreateAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	createTestAccessList(t, webPack, s)
}

func TestUpdateAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	accessListId := createTestAccessList(t, webPack, s)

	spec := accesslist.Spec{
		Title:              "access list 1",
		Description:        "access list 1 desc - updated",
		Owners:             []accesslist.Owner{{Name: "llama 2", Description: "llama 2 desc"}},
		OwnershipRequires:  accesslist.Requires{Roles: []string{"admin"}, Traits: trait.Traits{}},
		Grants:             accesslist.Grants{Roles: []string{"access"}, Traits: trait.Traits{}},
		MembershipRequires: accesslist.Requires{Traits: trait.Traits{}},
		Audit:              accesslist.Audit{Frequency: time.Hour},
	}

	accessListMember := accesslist.AccessListMemberSpec{
		Name:    "llama",
		Joined:  time.Now(),
		Expires: time.Now().Add(time.Hour),
		Reason:  "reason",
		AddedBy: "admin",
	}

	accessListMember2 := accesslist.AccessListMemberSpec{
		Name:    "llama-2",
		Joined:  time.Now(),
		Expires: time.Now().Add(time.Hour),
		Reason:  "reason",
		AddedBy: "admin",
	}

	accessListMember3 := accesslist.AccessListMemberSpec{
		Name:    "llama-3",
		Joined:  time.Now(),
		Expires: time.Now().Add(time.Hour),
		Reason:  "reason",
		AddedBy: "admin",
	}

	// Add one member. The list should have one member.
	updateAccessList(t, webPack, s, accessListId, spec, accessListMember)
	// Add another member. The list should have two members.
	updateAccessList(t, webPack, s, accessListId, spec, accessListMember, accessListMember2)
	// Add different member. The previous members should be removed, and this member should be the only one.
	updateAccessList(t, webPack, s, accessListId, spec, accessListMember3)
}

func updateAccessList(t *testing.T, webPack *authWebPack, s *webSuite, accessListId string, spec accesslist.Spec, accessListMember ...accesslist.AccessListMemberSpec) {
	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", accessListId)
	resp, err := webPack.clt.PutJSON(s.ctx, endpoint, ui.UpsertAccessListRequest{
		Spec:    spec,
		Members: accessListMember,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Empty(t, cmp.Diff(spec, accessListResp.AccessList.Spec, cmpopts.EquateEmpty()))
	require.Equal(t, accessListId, accessListResp.AccessList.Metadata.Name)
	require.Len(t, accessListResp.AccessList.Members, len(accessListMember))
	for i, member := range accessListResp.AccessList.Members {
		require.Equal(t, member, accessListResp.AccessList.Members[i])
	}
}

func TestGetAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	accessList, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title: "access list 1",
		Audit: accesslist.Audit{Frequency: time.Hour},
		Owners: []accesslist.Owner{
			{
				Name:        "llama",
				Description: "llama desc",
				// this owner does not exist in the backend, but that's okay, we are only
				// testing for a "getter" for this test.
				IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST.String(),
			},
		},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	createdAccessList, err := authClient.AccessListClient().UpsertAccessList(context.Background(), accessList)
	require.NoError(t, err)
	require.Empty(t, createdAccessList.Spec.Owners[0].IneligibleStatus)
	// Set the ineligibleStatus back, because `upsert's` does not preserve ineligible reasons.
	createdAccessList.Spec.Owners[0].IneligibleStatus = accessList.Spec.Owners[0].IneligibleStatus

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: createdAccessList.GetName(),
		}, accesslist.AccessListMemberSpec{
			AccessList: createdAccessList.GetName(),
			Name:       "llama",
			Joined:     time.Now(),
			Expires:    time.Now().Add(time.Hour),
			Reason:     "reason",
			AddedBy:    "admin",
			// this member does not exist in the backend, but that's okay, we are only
			// testing for a "getter" for this test.
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST.String(),
		},
	)
	require.NoError(t, err)

	createdMember, err := authClient.AccessListClient().UpsertAccessListMember(context.Background(), member)
	require.NoError(t, err)
	require.Empty(t, createdMember.Spec.IneligibleStatus)
	// Set the ineligibleStatus back, because `upsert's` does not preserve ineligible reasons.
	createdMember.Spec.IneligibleStatus = member.Spec.IneligibleStatus

	accessListResp := getAccessList(t, webPack, s, createdAccessList.GetName())
	require.Empty(t, cmp.Diff(createdAccessList, accessListResp.AccessList.AccessList, cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision")))

	// Members are returned by the API.
	require.Len(t, accessListResp.AccessList.Members, 1)
	require.Equal(t, createdMember.Spec, accessListResp.AccessList.Members[0])
}

func TestDeleteAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	accesList, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title:             "access list 1",
		Audit:             accesslist.Audit{Frequency: time.Hour},
		Owners:            []accesslist.Owner{{Name: "llama", Description: "llama desc"}},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)

	ctx := context.Background()
	createdAccessList, err := authClient.AccessListClient().UpsertAccessList(ctx, accesList)
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", createdAccessList.GetName())
	_, err = webPack.clt.Delete(s.ctx, endpoint)
	require.NoError(t, err)

	_, err = authClient.AccessListClient().GetAccessList(ctx, createdAccessList.GetName())
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
}

func TestAddMemberToAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	accessListName := createTestAccessList(t, webPack, s)

	accessListMember := accesslist.AccessListMemberSpec{
		Name:    "llama-3",
		Joined:  time.Now(),
		Expires: time.Now().Add(time.Hour),
		Reason:  "reason",
		AddedBy: "admin",
	}

	addMemberToAccessList(t, webPack, s, accessListName, accessListMember)

	// Check that the member was added.
	accessListResp := getAccessList(t, webPack, s, accessListName)
	require.Len(t, accessListResp.AccessList.Members, 1)
	require.Equal(t, "llama-3", accessListResp.AccessList.Members[0].Name)
}

func addMemberToAccessList(t *testing.T, webPack *authWebPack, s *webSuite, accessListName string, accessListMember accesslist.AccessListMemberSpec) {
	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", accessListName, "members")
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.AddAccessListMemberRequest{
		Members: []accesslist.AccessListMemberSpec{
			accessListMember,
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
}

func createTestAccessList(t *testing.T, webPack *authWebPack, s *webSuite) string {
	t.Helper()

	spec := accesslist.Spec{
		Title:              "access list 1",
		Owners:             []accesslist.Owner{{Name: "llama", Description: "llama desc"}},
		OwnershipRequires:  accesslist.Requires{Roles: []string{"admin"}, Traits: trait.Traits{}},
		Grants:             accesslist.Grants{Roles: []string{"access"}, Traits: trait.Traits{}},
		MembershipRequires: accesslist.Requires{Traits: trait.Traits{}},
		Audit:              accesslist.Audit{Frequency: time.Hour},
	}

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist")
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.UpsertAccessListRequest{
		Spec: spec,
	})
	require.NoError(t, err)

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Equal(t, spec, accessListResp.AccessList.Spec)

	return accessListResp.AccessList.Metadata.Name
}

func getAccessList(t *testing.T, webPack *authWebPack, s *webSuite, accessListName string) ui.AccessListResponse {
	t.Helper()

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", accessListName)
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))

	return accessListResp
}
