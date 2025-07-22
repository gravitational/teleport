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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

var (
	accessListCmpOpts = cmp.Options{
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	}
	mainOwner = "llama"
)

func TestGetAccessLists(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()
	accessList1, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title:             "access list 1",
		Audit:             accesslist.Audit{NextAuditDate: s.clock.Now()},
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
		Audit:             accesslist.Audit{NextAuditDate: s.clock.Now()},
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
	require.Equal(t, uint32(1), *accessListResp.AccessLists[0].MembersCount)

	require.Empty(t, cmp.Diff(
		[]*accesslist.AccessList{accessListResp.AccessLists[0].AccessList, accessListResp.AccessLists[1].AccessList},
		[]*accesslist.AccessList{createdAccessList1, createdAccessList2},
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	))
}

func TestCreateAccessList(t *testing.T) {
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")

	createTestAccessList(t, webPack, s)
}

func TestUpdateAccessList(t *testing.T) {
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")

	accessListId := createTestAccessList(t, webPack, s)

	// Add labels to the access list.
	adminClient := s.newAdminAuthClient(s.ctx, t)

	accessList, err := adminClient.AccessListClient().GetAccessList(s.ctx, accessListId)
	require.NoError(t, err)

	accessList.SetStaticLabels(map[string]string{
		"label": "value",
	})

	accessList, _, err = adminClient.AccessListClient().UpsertAccessListWithMembers(s.ctx, accessList, nil)
	require.NoError(t, err)

	accessListMember := accesslist.AccessListMemberSpec{
		Name:    "llama-1",
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

	webPack = s.newAuthWebPack(t, mainOwner, skipUserCreation())
	// Add one member. The list should have one member.
	updateAccessList(t, webPack, s, accessListId, accessList.Spec, accessListMember)
	// Add another member. The list should have two members.
	updateAccessList(t, webPack, s, accessListId, accessList.Spec, accessListMember, accessListMember2)
	// Add different member. The previous members should be removed, and this member should be the only one.
	updateAccessList(t, webPack, s, accessListId, accessList.Spec, accessListMember3)
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
	require.Empty(t, cmp.Diff(spec, accessListResp.AccessList.Spec, accessListCmpOpts))
	require.Equal(t, accessListId, accessListResp.AccessList.Metadata.Name)
	require.Len(t, accessListResp.AccessList.Members, len(accessListMember))
	for i, member := range accessListResp.AccessList.Members {
		require.Equal(t, member, accessListResp.AccessList.Members[i])
	}
}

func TestGetAccessList(t *testing.T) {
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	accessList, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title: "access list 1",
		Audit: accesslist.Audit{NextAuditDate: s.clock.Now()},
		Owners: []accesslist.Owner{
			{
				Name:        "llama",
				Description: "llama desc",
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
		},
	)
	require.NoError(t, err)

	createdMember, err := authClient.AccessListClient().UpsertAccessListMember(context.Background(), member)
	require.NoError(t, err)
	require.Empty(t, createdMember.Spec.IneligibleStatus)
	// Set the ineligibleStatus back, because `upsert's` does not preserve ineligible reasons.
	createdMember.Spec.IneligibleStatus = member.Spec.IneligibleStatus

	accessListResp := getAccessList(t, webPack, s, createdAccessList.GetName())
	require.Empty(t, cmp.Diff(createdAccessList, accessListResp.AccessList.AccessList, accessListCmpOpts))
	// Members are returned by the API.
	require.Len(t, accessListResp.AccessList.Members, 1)
	require.Equal(t, createdMember.Spec, accessListResp.AccessList.Members[0])
}

func TestDeleteAccessList(t *testing.T) {
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	accesList, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title:             "access list 1",
		Audit:             accesslist.Audit{NextAuditDate: s.clock.Now()},
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
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
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

func TestReviewAccessList(t *testing.T) {
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")

	accessListName := createTestAccessList(t, webPack, s)

	review := accesslist.ReviewSpec{
		AccessList: accessListName,
		Reviewers:  []string{"does-not-matter"},
		ReviewDate: s.clock.Now(),
		Changes: accesslist.ReviewChanges{
			MembershipRequirementsChanged: &accesslist.Requires{Roles: []string{"access"}},
		},
	}

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", accessListName, "reviews")
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.ReviewAccessListRequest{
		ReviewSpec: review,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var reviewResp ui.ReviewAccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &reviewResp))
	require.NotEmpty(t, reviewResp.NextAuditDate)

	// Fetch the updated access list to test the review date matches.
	updatedList := getAccessList(t, webPack, s, accessListName)
	require.Equal(t, updatedList.AccessList.Spec.Audit.NextAuditDate, reviewResp.NextAuditDate)

	// Check that membership required roles was added.
	accessListResp := getAccessList(t, webPack, s, accessListName)
	require.Equal(t, []string{"access"}, accessListResp.AccessList.AccessList.GetMembershipRequires().Roles)
}

func createTestAccessList(t *testing.T, webPack *authWebPack, s *webSuite) string {
	t.Helper()

	// Create a valid user and role.
	role, err := authtest.CreateRole(s.ctx, s.testAuthServer.Auth(), "llama-role", types.RoleSpecV6{})
	require.NoError(t, err)
	user, err := types.NewUser(mainOwner)
	require.NoError(t, err)
	user.AddRole(role.GetName())
	_, err = s.testAuthServer.AuthServer.AuthServer.CreateUser(s.ctx, user)
	require.NoError(t, err)
	err = s.testAuthServer.Auth().UpsertPassword(user.GetName(), []byte(s.testPassword()))
	require.NoError(t, err)

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "name",
	}, accesslist.Spec{
		Title:              "access list 1",
		Owners:             []accesslist.Owner{{Name: "llama", Description: "llama desc", IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()}},
		OwnershipRequires:  accesslist.Requires{Roles: []string{"llama-role"}},
		Grants:             accesslist.Grants{Roles: []string{"access"}},
		MembershipRequires: accesslist.Requires{},
		Audit:              accesslist.Audit{NextAuditDate: time.Now()},
	})
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist")
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.UpsertAccessListRequest{
		Spec: accessList.Spec,
	})
	require.NoError(t, err)

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Empty(t, cmp.Diff(accessList.Spec, accessListResp.AccessList.Spec,
		accessListCmpOpts))

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
