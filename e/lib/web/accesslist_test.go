package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/web"
)

var (
	accessListCmpOpts = cmp.Options{
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(accesslist.AccessList{}, "Status"),
		cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
	}
)

func TestGetAccessLists(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
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
		// Computed for the caller of each request, which differs between the
		// upsert and the list request.
		cmpopts.IgnoreFields(accesslist.Status{}, "CurrentUserAssignments"),
	))
}

// TestAccessListsAdjustPageSize verifies that all access-list listing endpoints
// retry with a smaller page size when their initial gRPC response is too large.
func TestAccessListsAdjustPageSize(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t,
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
		// Use a low gRPC receive message size limit (4 KiB) so we can more easily simulate
		// messages exceeding the limit without using excessive memory.
		withHandlerOption(
			web.WithWebSessionRootClientDialOption(
				grpc.WithDefaultCallOptions(
					grpc.MaxCallRecvMsgSize(4*1024),
				),
			),
		),
	)

	authClient := s.newAdminAuthClient(t.Context(), t)
	for _, name := range []string{"test-1", "test-2"} {
		// Each access list is about 3 KiB, so both lists exceed the 4 KiB
		// gRPC receive limit when returned in a single response.
		accessList, err := accesslist.NewAccessList(header.Metadata{Name: name}, accesslist.Spec{
			Title:             name,
			Description:       strings.Repeat("x", 3*1024),
			Audit:             accesslist.Audit{NextAuditDate: s.clock.Now()},
			Owners:            []accesslist.Owner{{Name: "alice"}},
			OwnershipRequires: accesslist.Requires{},
			Grants:            accesslist.Grants{Roles: []string{"admin"}},
		})
		require.NoError(t, err)
		_, err = authClient.AccessListClient().UpsertAccessList(t.Context(), accessList)
		require.NoError(t, err)
	}

	webPack := s.newAuthWebPack(t, "foo")

	t.Run("v1/legacy list accesslists endpoint", func(t *testing.T) {
		endpoint := webPack.clt.Endpoint("enterprise", "accesslist")
		resp, err := webPack.clt.Get(t.Context(), endpoint, url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		var accessListResp ui.AccessListsResponse
		require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
		require.Len(t, accessListResp.AccessLists, 2)
		require.Equal(t, "test-1", accessListResp.AccessLists[0].GetName())
		require.Equal(t, "test-2", accessListResp.AccessLists[1].GetName())
	})

	t.Run("v2 list accesslists endpoint", func(t *testing.T) {
		endpoint := webPack.clt.Endpoint("v2", "enterprise", "accesslists")

		var firstPage ui.AccessListsResponse
		resp, err := webPack.clt.Get(t.Context(), endpoint, url.Values{
			"sort":  []string{"name:asc"},
			"limit": []string{"2"},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
		require.NoError(t, json.Unmarshal(resp.Bytes(), &firstPage))
		require.Len(t, firstPage.AccessLists, 1)
		require.Equal(t, "test-1", firstPage.AccessLists[0].GetName())
		require.NotEmpty(t, firstPage.StartKey)

		var secondPage ui.AccessListsResponse
		resp, err = webPack.clt.Get(t.Context(), endpoint, url.Values{
			"sort":     []string{"name:asc"},
			"limit":    []string{"2"},
			"startKey": []string{firstPage.StartKey},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
		require.NoError(t, json.Unmarshal(resp.Bytes(), &secondPage))
		require.Len(t, secondPage.AccessLists, 1)
		require.Equal(t, "test-2", secondPage.AccessLists[0].GetName())
		require.Empty(t, secondPage.StartKey)
	})

	t.Run("list user accesslists endpoint", func(t *testing.T) {
		_ = createUser(t, s, "alice")

		endpoint := webPack.clt.Endpoint("enterprise", "users", "alice", "accesslists")

		var firstPage ui.AccessListsResponse
		resp, err := webPack.clt.Get(t.Context(), endpoint, url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
		require.NoError(t, json.Unmarshal(resp.Bytes(), &firstPage))
		require.Len(t, firstPage.AccessLists, 1)
		require.Equal(t, "test-1", firstPage.AccessLists[0].GetName())
		require.NotEmpty(t, firstPage.StartKey)
		require.Equal(t, int32(2), firstPage.TotalCount)

		var secondPage ui.AccessListsResponse
		resp, err = webPack.clt.Get(t.Context(), endpoint, url.Values{"startKey": []string{firstPage.StartKey}})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
		require.NoError(t, json.Unmarshal(resp.Bytes(), &secondPage))
		require.Len(t, secondPage.AccessLists, 1)
		require.Equal(t, "test-2", secondPage.AccessLists[0].GetName())
		require.Empty(t, secondPage.StartKey)
		require.Equal(t, int32(2), secondPage.TotalCount)
	})
}

func TestCreateAccessList(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo")

	owner := createUser(t, s, "llama")
	ownerWebClt := s.newAuthWebPack(t, owner.GetName(), skipUserCreation()).clt

	t.Run("can create a regular access list", func(t *testing.T) {
		for _, typ := range []accesslist.Type{accesslist.DeprecatedDynamic, accesslist.Default, accesslist.SCIM} {
			t.Run(string(typ), func(t *testing.T) {
				_ = testCreateAccessListRequireOK(t, webPack.clt, owner, withType(typ))
			})
		}
	})

	t.Run("cannot create UI RO access list", func(t *testing.T) {
		for _, typ := range []accesslist.Type{accesslist.Static} {
			t.Run(string(typ), func(t *testing.T) {
				_, resp, err := testCreateAccessList(t, ownerWebClt, owner, withType(typ))
				require.Error(t, err)
				require.ErrorContains(t, err, fmt.Sprintf("is of type %q and cannot be created or modified via web UI", typ))
				require.True(t, trace.IsBadParameter(err))
				require.Equal(t, http.StatusBadRequest, resp.Code())
			})
		}
	})
}

func TestUpdateAccessList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	owner := createUser(t, s, "llama")
	ownerWebClt := s.newAuthWebPack(t, owner.GetName(), skipUserCreation()).clt

	t.Run("can update a regular access list", func(t *testing.T) {
		for _, typ := range []accesslist.Type{accesslist.DeprecatedDynamic, accesslist.Default} {
			t.Run(string(typ), func(t *testing.T) {
				svc := s.testAuthServer.AuthServer.AuthServer.AccessListsInternal

				accessList, err := svc.UpsertAccessList(ctx, newAccessList(t,
					"test_"+string(typ),
					withType(typ),
					withOwners([]accesslist.Owner{{Name: owner.GetName()}}),
				))

				require.NoError(t, err)

				// Add labels to the access list.
				adminClient := s.newAdminAuthClient(ctx, t)

				accessList.SetStaticLabels(map[string]string{
					"label": "value",
				})

				accessList, _, err = adminClient.AccessListClient().UpsertAccessListWithMembers(ctx, accessList, nil)
				require.NoError(t, err)

				accessListMember := newAccessListMemberSpec(t, "llama-1", withExpires(time.Now().Add(time.Hour)), withReason("reason"))
				accessListMember2 := newAccessListMemberSpec(t, "llama-2", withExpires(time.Now().Add(time.Hour)), withReason("reason"))
				accessListMember3 := newAccessListMemberSpec(t, "llama-3", withExpires(time.Now().Add(time.Hour)), withReason("reason"))

				// Add one member. The list should have one member.
				testUpdateAccessListRequireOK(t, ownerWebClt, accessList.GetName(), accessList.Spec, accessListMember)
				// Add another member. The list should have two members.
				testUpdateAccessListRequireOK(t, ownerWebClt, accessList.GetName(), accessList.Spec, accessListMember, accessListMember2)
				// Add different member. The previous members should be removed, and this member should be the only one.
				testUpdateAccessListRequireOK(t, ownerWebClt, accessList.GetName(), accessList.Spec, accessListMember3)
			})
		}
	})

	t.Run("SCIM access list member modification is blocked", func(t *testing.T) {
		svc := s.testAuthServer.AuthServer.AuthServer.AccessListsInternal
		accessList, err := svc.UpsertAccessList(ctx, newAccessList(t,
			"test_scim",
			withType(accesslist.SCIM),
			withOwners([]accesslist.Owner{{Name: owner.GetName()}}),
		))
		require.NoError(t, err)

		accessListMember := newAccessListMemberSpec(t, "llama-1", withExpires(time.Now().Add(time.Hour)), withReason("reason"))
		resp, err := testUpdateAccessList(t, ownerWebClt, accessList.GetName(), accessList.Spec, accessListMember)
		require.Error(t, err)
		require.ErrorContains(t, err, "SCIM-sourced Access List members modification not allowed")
		require.Equal(t, http.StatusBadRequest, resp.Code())
	})

	t.Run("cannot update  UI RO access list", func(t *testing.T) {
		for _, typ := range []accesslist.Type{accesslist.Static} {
			t.Run(string(typ), func(t *testing.T) {
				svc := s.testAuthServer.AuthServer.AuthServer.AccessListsInternal

				accessList, err := svc.UpsertAccessList(ctx, newAccessList(t,
					"test_"+string(typ),
					withType(typ),
					withOwners([]accesslist.Owner{{Name: owner.GetName()}}),
				))
				require.NoError(t, err)

				resp, err := testUpdateAccessList(t, ownerWebClt, accessList.GetName(), accessList.Spec)
				require.Error(t, err)
				require.ErrorContains(t, err, fmt.Sprintf("is of type %q and cannot be created or modified via web UI", typ))
				require.True(t, trace.IsBadParameter(err))
				require.Equal(t, http.StatusBadRequest, resp.Code())
			})
		}
	})
}

func TestGetAccessList(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	displayUser, err := types.NewUser("llama")
	require.NoError(t, err)
	displayUser.SetTraits(map[string][]string{
		"displayName": {"Llama Display"},
		"email":       {"llama@example.com"},
	})
	_, err = authClient.UpsertUser(context.Background(), displayUser)
	require.NoError(t, err)

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

	adderUser, err := types.NewUser(createdMember.Spec.AddedBy)
	require.NoError(t, err)
	adderUser.SetTraits(map[string][]string{
		"displayName": {"Adder Display"},
		"email":       {"adder@example.com"},
	})
	_, err = authClient.UpsertUser(context.Background(), adderUser)
	require.NoError(t, err)

	accessListResp := getAccessList(t, webPack, s, createdAccessList.GetName())
	require.Empty(t, cmp.Diff(createdAccessList, accessListResp.AccessList.AccessList, accessListCmpOpts))
	// Members are returned by the API.
	require.Len(t, accessListResp.AccessList.Members, 1)
	require.Equal(t, createdMember.Spec, accessListResp.AccessList.Members[0])
	require.Equal(t, map[string]types.UserDisplay{
		"llama":                    {Primary: "Llama Display", Secondary: "llama@example.com"},
		createdMember.Spec.AddedBy: {Primary: "Adder Display", Secondary: "adder@example.com"},
	}, accessListResp.AccessList.UserDisplays)

	t.Run("returns members across multiple pages", func(t *testing.T) {
		pagedAccessList, err := authClient.AccessListClient().UpsertAccessList(t.Context(), newAccessList(t, "paged-accesslist"))
		require.NoError(t, err)

		memberCount := apidefaults.DefaultChunkSize + 1
		for i := range memberCount {
			member, err := accesslist.NewAccessListMember(
				header.Metadata{Name: fmt.Sprintf("member-%04d", i)},
				accesslist.AccessListMemberSpec{
					AccessList: pagedAccessList.GetName(),
					Name:       fmt.Sprintf("member-%04d", i),
					Joined:     time.Now(),
					Expires:    time.Now().Add(time.Hour),
					Reason:     "reason",
					AddedBy:    "admin",
				},
			)
			require.NoError(t, err)

			_, err = authClient.AccessListClient().UpsertAccessListMember(context.Background(), member)
			require.NoError(t, err)
		}

		accessListResp := getAccessList(t, webPack, s, pagedAccessList.GetName())
		require.Len(t, accessListResp.AccessList.Members, memberCount)
		require.Equal(t, "member-0000", accessListResp.AccessList.Members[0].Name)
		require.Equal(t, fmt.Sprintf("member-%04d", memberCount-1), accessListResp.AccessList.Members[memberCount-1].Name)
	})
}

func TestCollectAccessListUserDisplays(t *testing.T) {
	accessList := &accesslist.AccessList{
		Status: accesslist.Status{
			OwnerDisplays: map[string]types.UserDisplay{
				"owner": {Primary: "Owner Display", Secondary: "owner@example.com"},
			},
		},
	}
	members := []*accesslist.AccessListMember{
		{
			Spec: accesslist.AccessListMemberSpec{
				Name:    "member",
				AddedBy: "adder",
			},
			Status: &accesslist.AccessListMemberStatus{
				Display:        &types.UserDisplay{Primary: "Member Display", Secondary: "member@example.com"},
				AddedByDisplay: &types.UserDisplay{},
			},
		},
		{
			Spec: accesslist.AccessListMemberSpec{
				Name: "missing-display",
			},
		},
	}

	require.Equal(t, map[string]types.UserDisplay{
		"member": {Primary: "Member Display", Secondary: "member@example.com"},
		"adder":  {},
		"owner":  {Primary: "Owner Display", Secondary: "owner@example.com"},
	}, collectAccessListUserDisplays(accessList, members))
}

func TestUpsertAccessListUserDisplays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	owner := createUser(t, s, "llama")
	ownerWebClt := s.newAuthWebPack(t, owner.GetName(), skipUserCreation()).clt

	adminClient := s.newAdminAuthClient(ctx, t)

	// The owner is also the adder for members added through the web handler.
	owner.SetTraits(map[string][]string{
		"displayName": {"Llama Display"},
		"email":       {"llama@example.com"},
	})
	_, err := adminClient.UpsertUser(ctx, owner)
	require.NoError(t, err)

	memberUser, err := types.NewUser("display-member")
	require.NoError(t, err)
	memberUser.SetTraits(map[string][]string{
		"displayName": {"Member Display"},
		"email":       {"member@example.com"},
	})
	_, err = adminClient.UpsertUser(ctx, memberUser)
	require.NoError(t, err)

	svc := s.testAuthServer.AuthServer.AuthServer.AccessListsInternal
	accessList, err := svc.UpsertAccessList(ctx, newAccessList(t, "display-list",
		withOwners([]accesslist.Owner{{Name: owner.GetName()}}),
	))
	require.NoError(t, err)

	memberSpec := newAccessListMemberSpec(t, "display-member", withExpires(time.Now().Add(time.Hour)), withReason("reason"))
	resp, err := testUpdateAccessList(t, ownerWebClt, accessList.GetName(), accessList.Spec, memberSpec)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))

	// The upsert response carries owner, member, and added_by displays (AC-11b).
	require.Equal(t, map[string]types.UserDisplay{
		"llama":          {Primary: "Llama Display", Secondary: "llama@example.com"},
		"display-member": {Primary: "Member Display", Secondary: "member@example.com"},
	}, accessListResp.AccessList.UserDisplays)
}

func TestDeleteAccessList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	t.Run("can delete a regular access list", func(t *testing.T) {
		for _, typ := range []accesslist.Type{accesslist.DeprecatedDynamic, accesslist.Default} {
			t.Run(string(typ), func(t *testing.T) {
				accessList, err := authClient.AccessListClient().UpsertAccessList(ctx, newAccessList(t,
					"access_list_1_"+string(typ), withType(typ),
				))
				require.NoError(t, err)

				testDeleteAccessListRequireOK(t, webPack.clt, accessList.GetName())

				_, err = authClient.AccessListClient().GetAccessList(ctx, accessList.GetName())
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			})
		}
	})

	t.Run("cannot delete UI RO access list", func(t *testing.T) {
		for _, typ := range []accesslist.Type{accesslist.Static} {
			t.Run(string(typ), func(t *testing.T) {
				accessList, err := authClient.AccessListClient().UpsertAccessList(ctx, newAccessList(t,
					"access_list_1_"+string(typ), withType(typ),
				))
				require.NoError(t, err)

				resp, err := testDeleteAccessList(t, webPack.clt, accessList.GetName())
				require.Error(t, err)
				require.ErrorContains(t, err, fmt.Sprintf("is of type %q and cannot be created or modified via web UI", typ))
				require.True(t, trace.IsBadParameter(err))
				require.Equal(t, http.StatusBadRequest, resp.Code())

				_, err = authClient.AccessListClient().GetAccessList(ctx, accessList.GetName())
				require.NoError(t, err)
			})
		}
	})
}

func TestAddMemberToAccessList(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo")

	owner := createUser(t, s, "llama")
	accessListName := testCreateAccessListRequireOK(t, webPack.clt, owner)

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
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")

	owner := createUser(t, s, "llama")
	accessListName := testCreateAccessListRequireOK(t, webPack.clt, owner)

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

func TestListAccessListReviewsIncludesReviewerInfo(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)

	webPack := s.newAuthWebPack(t, "foo")
	reviewer, err := s.testAuthServer.Auth().Services.GetUser(s.ctx, "foo", false)
	require.NoError(t, err)
	reviewer.SetTraits(map[string][]string{
		"displayName": {"Review Owner"},
		"email":       {"reviewer@example.com"},
	})
	_, err = s.testAuthServer.Auth().Services.UpdateUser(s.ctx, reviewer)
	require.NoError(t, err)

	owner := createUser(t, s, "reviewer-display-owner")
	accessListName := testCreateAccessListRequireOK(t, webPack.clt, owner)

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", accessListName, "reviews")
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.ReviewAccessListRequest{
		ReviewSpec: accesslist.ReviewSpec{
			AccessList: accessListName,
			Reviewers:  []string{"does-not-matter"},
			ReviewDate: s.clock.Now(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var reviewResp struct {
		Reviews []struct {
			Spec struct {
				Reviewers []string `json:"reviewers"`
			} `json:"spec"`
			ReviewersInfo []struct {
				Username string             `json:"username"`
				Display  *types.UserDisplay `json:"display,omitempty"`
			} `json:"reviewersInfo"`
		} `json:"reviews"`
		ReviewerDisplays json.RawMessage `json:"reviewerDisplays"`
	}
	require.NoError(t, json.Unmarshal(resp.Bytes(), &reviewResp))
	require.Len(t, reviewResp.Reviews, 1)
	require.Equal(t, []string{reviewer.GetName()}, reviewResp.Reviews[0].Spec.Reviewers)
	require.Len(t, reviewResp.Reviews[0].ReviewersInfo, 1)
	require.Equal(t, reviewer.GetName(), reviewResp.Reviews[0].ReviewersInfo[0].Username)
	require.Equal(t, &types.UserDisplay{
		Primary:   "Review Owner",
		Secondary: "reviewer@example.com",
	}, reviewResp.Reviews[0].ReviewersInfo[0].Display)
	require.Empty(t, reviewResp.ReviewerDisplays)
}

// Creates a user with an empty role with the same name and sets the users's password to
// `s.testPassword()`.
func createUser(t *testing.T, s *webSuite, name string) types.User {
	t.Helper()

	role, err := authtest.CreateRole(s.ctx, s.testAuthServer.Auth(), name, types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := types.NewUser(name)
	require.NoError(t, err)

	user.AddRole(role.GetName())
	user, err = s.testAuthServer.AuthServer.AuthServer.CreateUser(s.ctx, user)
	require.NoError(t, err)

	err = s.testAuthServer.Auth().UpsertPassword(user.GetName(), []byte(s.testPassword()))
	require.NoError(t, err)

	return user
}

func testCreateAccessList(t *testing.T, ownerWebClt *TestWebClient, owner types.User, opts ...accessListOpt) (accessListID string, resp *roundtrip.Response, err error) {
	t.Helper()
	ctx := context.Background()

	spec := newAccessListSpec(t,
		append([]accessListOpt{
			// prepend the default options so they can be overwritten by the opts function arg
			withOwners([]accesslist.Owner{{Name: owner.GetName(), Description: owner.GetName() + " desc"}}),
			withOwnershipRequires(accesslist.Requires{Roles: []string{owner.GetName()}}),
		}, opts...)...,
	)

	endpoint := ownerWebClt.Endpoint("enterprise", "accesslist")
	resp, err = ownerWebClt.PostJSON(ctx, endpoint, ui.UpsertAccessListRequest{
		Spec: spec,
	})
	if err != nil {
		return "", resp, err
	}

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Empty(t, cmp.Diff(spec, accessListResp.AccessList.Spec,
		accessListCmpOpts))
	// The response must include the caller's assignments so clients can derive
	// their permissions from it without refetching the access list.
	require.NotNil(t, accessListResp.AccessList.CurrentUserAssignments)

	return accessListResp.AccessList.Metadata.Name, resp, nil
}

func testCreateAccessListRequireOK(t *testing.T, ownerWebClt *TestWebClient, owner types.User, opts ...accessListOpt) string {
	accessListID, _, err := testCreateAccessList(t, ownerWebClt, owner, opts...)
	require.NoError(t, err)
	return accessListID
}

func testUpdateAccessList(t *testing.T, clt *TestWebClient, accessListID string, spec accesslist.Spec, memberSpec ...accesslist.AccessListMemberSpec) (resp *roundtrip.Response, err error) {
	t.Helper()
	ctx := context.Background()

	endpoint := clt.Endpoint("enterprise", "accesslist", accessListID)
	resp, err = clt.PutJSON(ctx, endpoint, ui.UpsertAccessListRequest{
		Spec:    spec,
		Members: memberSpec,
	})
	return resp, err
}

func testUpdateAccessListRequireOK(t *testing.T, clt *TestWebClient, accessListID string, spec accesslist.Spec, memberSpec ...accesslist.AccessListMemberSpec) {
	t.Helper()

	resp, err := testUpdateAccessList(t, clt, accessListID, spec, memberSpec...)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Empty(t, cmp.Diff(spec, accessListResp.AccessList.Spec, accessListCmpOpts))
	// The response must include the caller's assignments so clients can derive
	// their permissions from it without refetching the access list. The caller
	// here is always an owner of the list.
	require.NotNil(t, accessListResp.AccessList.CurrentUserAssignments)
	require.True(t, accessListResp.AccessList.CurrentUserAssignments.IsOwner())
	require.Len(t, accessListResp.AccessList.Members, len(memberSpec))
	for i, member := range memberSpec {
		require.Equal(t, member.Name, accessListResp.AccessList.Members[i].Name)
		require.Equal(t, accessListID, accessListResp.AccessList.Members[i].AccessList)
		require.Empty(t, cmp.Diff(
			member, accessListResp.AccessList.Members[i],
			append(
				accessListCmpOpts,
				cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "AddedBy"),    // set by auth
				cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "Joined"),     // set by auth
				cmpopts.IgnoreFields(accesslist.AccessListMemberSpec{}, "AccessList"), // set by auth and checked above
			),
		))
	}
}

func testDeleteAccessList(t *testing.T, clt *TestWebClient, accessListID string) (*roundtrip.Response, error) {
	t.Helper()
	ctx := context.Background()

	endpoint := clt.Endpoint("enterprise", "accesslist", accessListID)
	resp, err := clt.Delete(ctx, endpoint)
	return resp, err
}

func testDeleteAccessListRequireOK(t *testing.T, clt *TestWebClient, accessListID string) {
	t.Helper()
	resp, err := testDeleteAccessList(t, clt, accessListID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
}

func TestListAccessLists(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()

	// Create access lists with different owners and roles for filtering tests
	accessListConfigs := []struct {
		name   string
		owners []accesslist.Owner
		roles  []string
	}{
		{
			name:   "apple",
			owners: []accesslist.Owner{{Name: "alice", Description: "alice desc"}},
			roles:  []string{"viewer"},
		},
		{
			name:   "banana",
			owners: []accesslist.Owner{{Name: "bob", Description: "bob desc"}},
			roles:  []string{"editor"},
		},
		{
			name:   "cherry",
			owners: []accesslist.Owner{{Name: "alice", Description: "alice desc"}},
			roles:  []string{"admin"},
		},
		{
			name:   "appletwo",
			owners: []accesslist.Owner{{Name: "charlie", Description: "charlie desc"}},
			roles:  []string{"viewer"},
		},
		{
			name:   "orange",
			owners: []accesslist.Owner{{Name: "bob", Description: "bob desc"}},
			roles:  []string{"admin"},
		},
	}

	for _, config := range accessListConfigs {
		accessList, err := accesslist.NewAccessList(header.Metadata{Name: config.name}, accesslist.Spec{
			Title:             config.name,
			Audit:             accesslist.Audit{NextAuditDate: s.clock.Now()},
			Owners:            config.owners,
			OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
			Grants:            accesslist.Grants{Roles: config.roles},
		})
		require.NoError(t, err)
		_, err = authClient.AccessListClient().UpsertAccessList(ctx, accessList)
		require.NoError(t, err)
	}

	testCases := []struct {
		name                     string
		queryParams              url.Values
		expectedLen              int
		expectedListNamesInOrder []string
		expectedNext             string
	}{
		{
			name: "filter by search",
			queryParams: url.Values{
				"search": []string{"apple"},
				"sort":   []string{"name:asc"},
				"limit":  []string{"2"},
			},
			expectedLen:              2,
			expectedListNamesInOrder: []string{"apple", "appletwo"},
		},
		{
			name: "filter by search",
			queryParams: url.Values{
				"search": []string{"apple"},
				"sort":   []string{"name:asc"},
				"limit":  []string{"1"},
			},
			expectedLen:              1,
			expectedListNamesInOrder: []string{"apple"},
			expectedNext:             "appletwo",
		},
		{
			name: "next page exists",
			queryParams: url.Values{
				"limit": []string{"2"},
				"sort":  []string{"name:asc"},
			},
			expectedLen:              2,
			expectedListNamesInOrder: []string{"apple", "appletwo"},
			expectedNext:             "banana",
		},
		{
			name: "using nextKey in params",
			queryParams: url.Values{
				"limit":    []string{"2"},
				"sort":     []string{"name:asc"},
				"startKey": []string{"banana"},
			},
			expectedLen:              2,
			expectedListNamesInOrder: []string{"banana", "cherry"},
			expectedNext:             "orange",
		},
		{
			name: "filter by single owner",
			queryParams: url.Values{
				"owners": []string{"alice"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              2,
			expectedListNamesInOrder: []string{"apple", "cherry"},
		},
		{
			name: "filter by multiple owners",
			queryParams: url.Values{
				"owners": []string{"alice", "bob"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              4,
			expectedListNamesInOrder: []string{"apple", "banana", "cherry", "orange"},
		},
		{
			name: "filter by nonexistent owner",
			queryParams: url.Values{
				"owners": []string{"nonexistent"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              0,
			expectedListNamesInOrder: []string{},
		},
		{
			name: "filter by search and owner",
			queryParams: url.Values{
				"search": []string{"apple"},
				"owners": []string{"alice"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              1,
			expectedListNamesInOrder: []string{"apple"},
		},
		{
			name: "filter by search with owner filter - search doesn't match",
			queryParams: url.Values{
				"search": []string{"nonexistent"},
				"owners": []string{"alice"},
				"sort":   []string{"name:asc"},
			},
		},
		{
			name: "filter by single owner",
			queryParams: url.Values{
				"owners": []string{"alice"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              2,
			expectedListNamesInOrder: []string{"apple", "cherry"},
			expectedNext:             "",
		},
		{
			name: "filter by multiple owners",
			queryParams: url.Values{
				"owners": []string{"alice", "bob"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              4,
			expectedListNamesInOrder: []string{"apple", "banana", "cherry", "orange"},
		},
		{
			name: "filter by nonexistent owner",
			queryParams: url.Values{
				"owners": []string{"nonexistent"},
				"sort":   []string{"name:asc"},
			},
			expectedLen:              0,
			expectedListNamesInOrder: []string{},
		},
		{
			name: "filter by search with owner filter - search doesn't match",
			queryParams: url.Values{
				"search": []string{"nonexistent"},
				"owners": []string{"alice"},
				"sort":   []string{"name:asc"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := webPack.clt.Endpoint("v2", "enterprise", "accesslists")
			resp, err := webPack.clt.Get(s.ctx, endpoint, tc.queryParams)
			require.NoError(t, err)

			var accessListResp ui.AccessListsResponse
			require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))

			require.Len(t, accessListResp.AccessLists, tc.expectedLen)
			require.Equal(t, tc.expectedNext, accessListResp.StartKey)

			for i, expectedName := range tc.expectedListNamesInOrder {
				require.Equal(t, expectedName, accessListResp.AccessLists[i].GetName())
			}
		})
	}
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

func TestListUserAccessLists(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t,
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(&modulestest.Modules{
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity: {Enabled: true},
				},
			},
		}),
	)

	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()

	_ = createUser(t, s, "alice")

	accessListConfigs := []struct {
		name   string
		owners []accesslist.Owner
		roles  []string
	}{
		{
			name:   "apple",
			owners: []accesslist.Owner{{Name: "alice", Description: "alice desc"}},
			roles:  []string{"viewer"},
		},
		{
			name:   "banana",
			owners: []accesslist.Owner{{Name: "bob", Description: "bob desc"}},
			roles:  []string{"editor"},
		},
		{
			name:   "cherry",
			owners: []accesslist.Owner{{Name: "alice", Description: "alice desc"}},
			roles:  []string{"admin"},
		},
	}

	for _, config := range accessListConfigs {
		accessList, err := accesslist.NewAccessList(header.Metadata{Name: config.name}, accesslist.Spec{
			Title:             config.name,
			Audit:             accesslist.Audit{NextAuditDate: s.clock.Now()},
			Owners:            config.owners,
			OwnershipRequires: accesslist.Requires{},
			Grants:            accesslist.Grants{Roles: config.roles},
		})
		require.NoError(t, err)
		_, err = authClient.AccessListClient().UpsertAccessList(ctx, accessList)
		require.NoError(t, err)
	}

	endpoint := webPack.clt.Endpoint("enterprise", "users", "alice", "accesslists")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// returns alice's 'owned' ACLs for admin
	var accessListResp ui.AccessListsResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Len(t, accessListResp.AccessLists, 2)
	for i, expectedName := range []string{"apple", "cherry"} {
		require.Equal(t, expectedName, accessListResp.AccessLists[i].GetName())
	}

	// returns error for unauthorized user
	bob := createUser(t, s, "bob")
	bobWebClt := s.newAuthWebPack(t, bob.GetName(), skipUserCreation()).clt

	endpoint = bobWebClt.Endpoint("enterprise", "users", "alice", "accesslists")
	resp, err = bobWebClt.Get(s.ctx, endpoint, url.Values{})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.Equal(t, http.StatusForbidden, resp.Code())
}
