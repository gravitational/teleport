package okta

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

// TestBasicAssigmentFlow tests the basic assignment flow.
// That tests the basic assignment flow for the apps groups imported by Okta integration
// making sure that removing/adding members to the access list will trigger the assignment/unassignment
// and the proper Okta API calls are made.
func TestBasicAssigmentFlow(t *testing.T) {
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
