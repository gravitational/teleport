package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

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
		cmpopts.IgnoreFields(accesslist.AccessList{}, "Status"),
		cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
	}
)

func TestGetAccessLists(t *testing.T) {
	testModules := &modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
	testModules := &modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
	ctx := context.Background()
	testModules := &modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
	testModules := &modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	ctx := context.Background()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
	testModules := &modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
	testModules := &modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	}
	modulestest.SetTestModules(t, *testModules)

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
		withModules(testModules),
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
