package okta

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
)

func TestAccessListSync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create a user in Okta.
	user := fakeOkta.CreateUser("oktan")

	// Create Okta app with multiple embed links.
	appLinks := []oktaApplicationEmbedLink{
		{Name: "Power Dot", Href: "https://powerdot.mysoft365.example.com"},
		{Name: "Expel", Href: "https://expel.mysoft365.example.com"},
		{Name: "Crews", Href: "https://crews.mysoft365.example.com"},
	}
	app := fakeOkta.CreateBasicApp("my-soft-365", appLinks...)

	group := fakeOkta.CreateBuiltInGroup("Everyone")
	fakeOkta.AddUserToGroup(group.Id, user.Id)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	appServerWatcher := sut.NewResourceWatcher(t, types.KindAppServer)
	accessListWatcher := sut.NewResourceWatcher(t, types.KindAccessList)
	roleWatcher := sut.NewResourceWatcher(t, types.KindRole)

	_, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:            apiCredentials,
		SsoMetadataUrl:            fakeOkta.URL() + "/sso/saml/metadata",
		EnableUserSync:            true,
		DisableAssignDefaultRoles: false,
		EnableAppGroupSync:        true,
		EnableAccessListSync:      true,
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
			AppFilters:   []string{"my-soft-*"},
		}.Build(),
	}.Build())
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})

	// Before assigning user to application, we should have:
	// - Access List for the application
	// - App server for each application link
	t.Run("before assigning user to application", func(t *testing.T) {
		// Wait for Access List sync.
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

		// Make sure there are no Access Lists, because the application has no assignments.
		accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
		require.NoError(t, err)
		require.Empty(t, accessLists)

		// Ensure there are no roles for the app.
		roles, err := sut.Teleport.Process.GetAuthServer().GetRoles(ctx)
		require.NoError(t, err)
		for _, r := range roles {
			require.NotContains(t, r.GetName(), "my-soft-365", "role: %v", r)
		}

		// Ensure there is app server for each Okta app embed link.
		mySoft365AppServers := waitForResourceCount(t, appServerWatcher, 3, func(as types.AppServer) bool {
			return as.GetAllLabels()["teleport.internal/okta-app-name"] == "my-soft-365"
		})
		for _, appServer := range mySoft365AppServers {
			labels := appServer.GetAllLabels()
			require.Equal(t, "my-soft-365", labels["teleport.internal/okta-app-name"], "app with labels: %v", labels)
			require.Equal(t, app.Id, labels["teleport.internal/okta-app-id"], "app with labels: %v", labels)
		}
		var appServerDescriptions []string
		for _, appServer := range mySoft365AppServers {
			description, _ := appServer.GetLabel("teleport.internal/okta-app-description")
			appServerDescriptions = append(appServerDescriptions, description)
		}
		require.ElementsMatch(t,
			appServerDescriptions,
			[]string{
				"Power Dot",
				"Expel",
				"Crews",
			},
		)

		// Ensure there is exactly one app server per app link; the connector SAML app is skipped.
		appServers, err := sut.Teleport.Process.GetAuthServer().GetApplicationServers(ctx, defaults.Namespace)
		require.NoError(t, err)
		require.Len(t, appServers, 3)
	})

	// After assigning user to application, we should have additionally:
	// - access and reviewer system roles created
	// - App server for each application link
	t.Run("after assigning user to application", func(t *testing.T) {
		// Assign user to application so we have Access List and corresponding access and review
		// system roles created.
		require.NoError(t, fakeOkta.AssignUserToApplication(app.Id, user.Id))

		// Wait for Access List sync.
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

		resourceSuffix := testResourceSuffix(t, app.Id, appLinks)

		// Ensure there is 1 Access List.
		accessLists := waitForResourceCount(t, accessListWatcher, 1, func(*accesslist.AccessList) bool {
			return true
		})
		require.Equal(t, resourceSuffix, accessLists[0].GetName())

		// Ensure there are access and reviewer system roles and their names are stable,
		// i.e. created from the first app link.
		roles := waitForResourceCount(t, roleWatcher, 2, func(r types.Role) bool {
			return strings.HasPrefix(r.GetName(), "my-soft-365-")
		})

		roleNames := getResourceNames(roles)
		require.Contains(t, roleNames, "my-soft-365-access-okta-acl-role-"+resourceSuffix)
		require.Contains(t, roleNames, "my-soft-365-reviewer-okta-acl-role-"+resourceSuffix)
	})
}

func TestAccessListSync_bidirectionalSync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create and assign Okta SAML app users.
	user1 := fakeOkta.CreateUser("bob")
	user2 := fakeOkta.CreateUser("alice")
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user1.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user2.Id))

	app := fakeOkta.CreateBasicApp("app-bidirectional-sync")
	require.NoError(t, fakeOkta.AssignUserToApplication(app.Id, user1.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(app.Id, user2.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)
	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)

	// 1. Create integration with bidirectional sync disabled.

	_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: false, // disabled
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		ReuseConnector: "okta-pre-created-test",
	}.Build())
	require.NoError(t, err)
	updateOktaDelays(t, sut, delays{
		timeBetweenImports:                1 * time.Second,
		timeBetweenAssignmentProcessLoops: 1 * time.Second,
	})

	// 2. Wait for the connector SAML app users to be synchronized.

	mustWaitForEvent(t, sut, events.OktaUserSyncEvent)

	waitForResourceCount(t, userWatcher, 2, func(u types.User) bool {
		return u.Origin() == types.OriginOkta
	})

	// 3. Ensure there are okta_assignments for each user and they are "pending"

	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

	waitForResourceCount(t, assignmentWatcher, 2, func(a types.OktaAssignment) bool {
		return a.GetStatus() == constants.OktaAssignmentStatusPending
	})

	// 4. Check if the okta_assignments are still "pending" after another Access List sync

	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

	oktaAssignments, nextToken, err := authServer.ListOktaAssignments(ctx, 1000, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, oktaAssignments, 2)
	for _, assignment := range oktaAssignments {
		require.Equal(t, constants.OktaAssignmentStatusPending, assignment.GetStatus(), "okta_assignment for user = %q", assignment.GetUser())
	}

	// 5. Update integration enabling bidirectional sync

	mustUpdateOktaIntegration(ctx, t, oktaAuthClient, oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true, // enabled
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
	}.Build())

	// 6. Wait for AssignmentProcessor event and verify okta_assignments are in "successful" state

	mustWaitForEvent(t, sut, events.OktaAssignmentProcessEvent)

	waitForResourceCount(t, assignmentWatcher, 2, func(a types.OktaAssignment) bool {
		return a.GetStatus() == constants.OktaAssignmentStatusSuccessful
	})
}

func testResourceSuffix(t *testing.T, oktaAppId string, appLinks []oktaApplicationEmbedLink) string {
	t.Helper()
	var all []string
	for _, link := range appLinks {
		s, err := okta.AppName(oktaAppId, link.Name)
		require.NoError(t, err, "okta.AppName")
		all = append(all, s)
	}
	sort.Strings(all)
	return all[0]
}
