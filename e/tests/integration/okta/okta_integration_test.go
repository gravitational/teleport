package okta

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

// TestBasicAssignmentFlow tests the basic assignment flow.
// That tests the basic assignment flow for the apps groups imported by Okta integration
// making sure that removing/adding members to the access list will trigger the assignment/unassignment
// and the proper Okta API calls are made.
func TestBasicAssignmentFlow(t *testing.T) {
	ctx := context.Background()

	oktaInfra := createOktaSetup(t, ctx, newMockOktaAPIClient(), withAppsGroupsUsersCount(1, 2, 7))
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[0].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[4].Id)
	defaultOwner := oktaInfra.Users[5]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
	)

	tclCmd := &tctlCommand{
		DataDir:  sut.DataDir,
		Listener: sut.AuthListenerAddr,
	}

	tclCmd.run(t, []string{
		`plugins`, `install`, `okta`,
		`--org`, "https://trial-1234567.okta.com",
		`--saml-connector`, `okta`,
		`--group-filter=*`,
		`--app-filter=*`,
		fmt.Sprintf(`--api-token=%s`, "secret-okta-api-token"),
		fmt.Sprintf("--owner=%s", defaultOwner.login()),
	})

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100))
	waitForOktaFirstOktaAssignment(t, sut)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), oktaInfra.Users[0])

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		wantMembers := []*accesslist.AccessListMember{
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaInfra.Users[0].login()}}},
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaInfra.Users[1].login()}}},
			{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: oktaInfra.Users[4].login()}}},
		}
		var accessListMembers []*accesslist.AccessListMember
		mustRunTCTLAndGetResultAs(t, tclCmd, []string{
			`acl`, `users`, `ls`, `--format=json`, oktaInfra.Groups[0].Id,
		}, &accessListMembers)
		assertResourcesByName(t, wantMembers, accessListMembers)
	}, time.Second*10, time.Millisecond*100)
	tclCmd.run(t, []string{
		`acl`, `users`, `rm`, oktaInfra.Groups[0].Id, oktaInfra.Users[0].login(),
	})
	oktaInfra.assertUserWasUnassignedFromOktaGroup(t, oktaInfra.Users[0].Id, oktaInfra.Groups[0].Id)

	tclCmd.run(t, []string{
		`acl`, `users`, `add`, oktaInfra.Groups[0].Id, oktaInfra.Users[2].login(),
	})
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[2].Id, oktaInfra.Groups[0].Id)
}

// TestNestedAclAssignment tests the assignment and sync of nested access lists.
// It ensures that members from nested access lists are flattened and added to the root access list in Okta,
// and that on reconciliation during sync, the nested lists are not flattened on the Teleport side.
func TestNestedAclAssignment(t *testing.T) {
	ctx := context.Background()

	oktaInfra := createOktaSetup(t, ctx, newMockOktaAPIClient(), withAppsGroupsUsersCount(1, 2, 7))
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[2].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[3].Id)
	defaultOwner := oktaInfra.Users[0]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
	)

	authServer := sut.Teleport.Process.GetAuthServer()

	tclCmd := &tctlCommand{
		DataDir:  sut.DataDir,
		Listener: sut.AuthListenerAddr,
	}

	tclCmd.run(t, []string{
		`plugins`, `install`, `okta`,
		`--org`, "https://trial-1234567.okta.com",
		`--saml-connector`, `okta`,
		`--group-filter=*`,
		`--app-filter=*`,
		fmt.Sprintf(`--api-token=%s`, "secret-okta-api-token"),
		fmt.Sprintf("--owner=%s", defaultOwner.login()),
	})

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100))
	waitForOktaFirstOktaAssignment(t, sut)
	userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), defaultOwner)

	oktaSyncedList, err := authServer.GetAccessList(ctx, oktaInfra.Groups[0].Id)
	require.NoError(t, err)
	teleportList0 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-0", []string{defaultOwner.login()}, accesslist.Grants{Roles: []string{"access"}}, []string{oktaInfra.Users[4].login()})
	teleportList1 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-1", []string{defaultOwner.login()}, accesslist.Grants{Roles: []string{"editor"}}, []string{oktaInfra.Users[5].login()})
	teleportList2 := createAccessListWithMembers(t, ctx, sut, "teleport-access-list-2", []string{defaultOwner.login()}, accesslist.Grants{Roles: []string{"reviewer"}}, []string{oktaInfra.Users[6].login()})

	// teleport list 1 and 2 are nested within teleport list 0
	nestedTeleportList1, err := authServer.AccessLists.UpsertAccessListMember(ctx, mustCreateMember(t, teleportList0.GetName(), teleportList1.GetName(), accesslist.MembershipKindList))
	require.NoError(t, err)
	nestedTeleportList2, err := authServer.AccessLists.UpsertAccessListMember(ctx, mustCreateMember(t, teleportList0.GetName(), teleportList2.GetName(), accesslist.MembershipKindList))
	require.NoError(t, err)

	// teleport list 0 is nested within okta-created list
	nestedTeleportList0, err := authServer.AccessLists.UpsertAccessListMember(ctx, mustCreateMember(t, oktaSyncedList.GetName(), teleportList0.GetName(), accesslist.MembershipKindList))
	require.NoError(t, err)

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100))

	// members from nested lists should be flattened + added to root in okta
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[4].Id, oktaInfra.Groups[0].Id)
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[5].Id, oktaInfra.Groups[0].Id)
	oktaInfra.assertUserWasAssignedToOktaGroup(t, oktaInfra.Users[6].Id, oktaInfra.Groups[0].Id)

	// ensure membership on Teleport side isn't flattened after sync w/ okta
	assertAccessListMembers(t, ctx, sut, oktaInfra.Groups[0].Id, []string{
		// from root list
		oktaInfra.Users[1].login(),
		oktaInfra.Users[2].login(),
		oktaInfra.Users[3].login(),
		// nested list members
		nestedTeleportList0.GetName(),
	})

	// nested lists should contain their members
	assertAccessListMembers(t, ctx, sut, nestedTeleportList0.GetName(), []string{
		oktaInfra.Users[4].login(),
		nestedTeleportList1.GetName(),
		nestedTeleportList2.GetName(),
	})
	assertAccessListMembers(t, ctx, sut, nestedTeleportList1.GetName(), []string{
		oktaInfra.Users[5].login(),
	})
	assertAccessListMembers(t, ctx, sut, nestedTeleportList2.GetName(), []string{
		oktaInfra.Users[6].login(),
	})
}

// TestAccessRequest tests the access request flow. That tests the access request flow for the apps groups imported
// by Okta integration where the okta-requester role is used and the access list owner is able to review
// the access request and approve it.
func TestAccessRequest(t *testing.T) {
	ctx := context.Background()

	oktaInfra := createOktaSetup(t, ctx, newMockOktaAPIClient(), withAppsGroupsUsersCount(1, 2, 7))

	oktaInfra.createApplicationGroupAssignment(t, oktaInfra.Apps[0].Id, oktaInfra.Groups[0].Id)
	reviewer := oktaInfra.Users[5]
	requester := oktaInfra.Users[4]

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
	)

	tclCmd := &tctlCommand{
		DataDir:  sut.DataDir,
		Listener: sut.AuthListenerAddr,
	}

	tclCmd.run(t, []string{
		`plugins`, `install`, `okta`,
		`--org`, "https://trial-1234567.okta.com",
		`--saml-connector`, `okta`,
		`--group-filter=*`,
		`--app-filter=*`,
		fmt.Sprintf(`--api-token=%s`, "secret-okta-api-token"),
		fmt.Sprintf("--owner=%s", reviewer.login()),
	})

	waitForOktaSync(t, sut, withTimeout(time.Second*30), withStep(time.Millisecond*100))

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		var wantMembers []*accesslist.AccessListMember
		var accessListMembers []*accesslist.AccessListMember
		mustRunTCTLAndGetResultAs(t, tclCmd, []string{
			`acl`, `users`, `ls`, `--format=json`, oktaInfra.Groups[0].Id,
		}, &accessListMembers)
		assertResourcesByName(t, wantMembers, accessListMembers)
	}, time.Second*10, time.Millisecond*100)

	tshRequester := &tshCommand{
		DataDir:   sut.DataDir,
		Listener:  sut.AuthListenerAddr,
		tshHome:   t.TempDir(),
		clock:     clockwork.NewRealClock(),
		proxyAddr: sut.ProxyAddr,
	}
	t.Run("requester requests access to an app", func(t *testing.T) {
		tshRequester.mustLogin(t, requester.login(), append(oktaInfra.getUserGroups(t, requester.Id), "Everyone"))
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			var apps []*types.AppV3
			mustRunTSHAndGetResultAs(t, tshRequester, []string{
				"app", "ls",
				"--insecure",
				"--format=json",
			}, &apps)
			assert.Empty(t, apps)
		}, time.Second*10, time.Millisecond*100)

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			err := tshRequester.run(t, []string{
				`request`, `create`, `--resource`, fmt.Sprintf(`/local-site/app/%s`, mustGetAppIDbyAppLabel(t, sut, oktaInfra)), `--nowait`,
			})
			assert.NoError(t, err)
		}, time.Second*10, time.Millisecond*100)
	})

	var requestID string
	t.Run("reviewer reviews the access request", func(t *testing.T) {
		tshReviewer := &tshCommand{
			DataDir:   sut.DataDir,
			Listener:  sut.AuthListenerAddr,
			tshHome:   t.TempDir(),
			clock:     clockwork.NewRealClock(),
			proxyAddr: sut.ProxyAddr,
		}
		tshReviewer.mustLogin(t, reviewer.login(), append(oktaInfra.getUserGroups(t, requester.Id), "Everyone"))
		var accReqs []*types.AccessRequestV3
		mustRunTSHAndGetResultAs(t, tshReviewer, []string{
			`request`, `ls`, `--format=json`,
		}, &accReqs)
		require.Len(t, accReqs, 1)

		err := tshReviewer.run(t, []string{
			`request`, `review`, fmt.Sprintf(`--approve=%s`, accReqs[0].GetName()),
		})
		require.NoError(t, err)
		requestID = accReqs[0].GetName()
	})

	t.Run("requester is able to access the app", func(t *testing.T) {
		err := tshRequester.run(t, []string{
			`login`, fmt.Sprintf(`--request-id=%s`, requestID),
			`--insecure`,
		})
		require.NoError(t, err)

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			var apps []*types.AppV3
			mustRunTSHAndGetResultAs(t, tshRequester, []string{
				"app", "ls",
				"--insecure",
				"--format=json",
			}, &apps)
			want := []*types.AppV3{
				{Metadata: types.Metadata{Description: oktaInfra.Apps[0].Label}},
			}
			assertResourcesByDesc(t, want, apps)
		}, time.Second*10, time.Millisecond*100)
	})
}

// TestPluginEnrolmentSSOMetadataURL tests the enrolment of the Okta plugin where the SSO metadata URL is provided
// and the SAML connector is created based on the provided metadata URL.
func TestPluginEnrolmentSSOMetadataURLOnly(t *testing.T) {
	ctx := context.Background()
	httpMock := RoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/sso/saml/metadata") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(idp.EntityDescriptor)),
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
		}, nil
	})

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(httpMock),
	)

	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ScimToken:      "12345",
		SsoMetadataUrl: "https://trial-7284229.okta.com/app/exkjel1ccet9biVnA697/sso/saml/metadata",
	})
	require.NoError(t, err)
	resp, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta-integration", false)
	require.NoError(t, err)
	require.Equal(t, "https://trial-7284229.okta.com", resp.GetMetadata().Labels[types.OktaOrgURLLabel])
}

func assertAccessListMembers(t *testing.T, ctx context.Context, sut *common.SUT, accessListName string, want []string) {
	got, _, err := sut.Teleport.Process.GetAuthServer().ListAccessListMembers(ctx, accessListName, 0, "")
	require.NoError(t, err)

	var wantMembers []*accesslist.AccessListMember
	for _, name := range want {
		wantMembers = append(wantMembers, &accesslist.AccessListMember{ResourceHeader: header.ResourceHeader{Metadata: header.Metadata{Name: name}}})
	}

	assertResourcesByName(t, wantMembers, got)
}

func createAccessListWithMembers(t *testing.T, ctx context.Context, sut *common.SUT, name string, owners []string, grants accesslist.Grants, members []string) *accesslist.AccessList {
	var accessListOwners []accesslist.Owner
	for _, owner := range owners {
		accessListOwners = append(accessListOwners, accesslist.Owner{
			Name:             owner,
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
			MembershipKind:   accesslist.MembershipKindUser,
		})
	}
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title:  name,
		Owners: accessListOwners,
		Grants: grants,
		Audit:  accesslist.Audit{NextAuditDate: sut.Clock.Now()},
	})
	require.NoError(t, err)

	var accessListMembers []*accesslist.AccessListMember
	for _, member := range members {
		accessListMembers = append(accessListMembers, mustCreateMember(t, accessList.GetName(), member, accesslist.MembershipKindUser))
	}

	_, _, err = sut.Teleport.Process.GetAuthServer().AccessLists.UpsertAccessListWithMembers(ctx, accessList, accessListMembers)
	require.NoError(t, err)

	return accessList
}

func mustCreateMember(t *testing.T, aclName, memberName string, memberType string) *accesslist.AccessListMember {
	clock := clockwork.NewRealClock()
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     aclName,
			Name:           memberName,
			Joined:         clock.Now(),
			AddedBy:        "added by",
			Expires:        clock.Now().Add(time.Hour * 24).UTC(),
			MembershipKind: memberType,
		},
	)
	require.NoError(t, err)
	return member
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
