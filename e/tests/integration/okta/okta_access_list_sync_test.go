package okta

import (
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	oktasdk "github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
)

func TestAccessListSync(t *testing.T) {
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

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaApiClient.setRoundTripper(httpMock)

	// The value from app name is  taken from idp.EntityDescriptor returned by the httpMock
	// above.
	createOktaSAMLAPP(t, ctx, oktaApiClient, "example_test-okta-app-name")

	// Create a user in Okta.
	user, _ := createOktaUser(t, ctx, oktaApiClient, "oktan@test.com")
	// Create Okta app with multiple embed links.
	appLinks := []oktaApplicationEmbedLink{
		{Name: "Power Dot", Href: "https://powerdot.mysoft365.example.com"},
		{Name: "Expel", Href: "https://expel.mysoft365.example.com"},
		{Name: "Crews", Href: "https://crews.mysoft365.example.com"},
	}
	app := createOktaApp(
		t, ctx, oktaApiClient,
		"my-soft-365",
		withAppLinks(oktaApplicationEmbedLinks{appLinks}),
	)
	mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaApiClient, &oktaUserType{user})

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(httpMock),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:        durationpb.New(1 * time.Second),
		ApiCredentials:            apiCredentials,
		SsoMetadataUrl:            oktaApiClient.GetOrgUrl() + "/app/123487988/sso/saml/metadata",
		EnableUserSync:            true,
		DisableAssignDefaultRoles: false,
		EnableAppGroupSync:        true,
		EnableAccessListSync:      true,
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
			AppFilters:   []string{"my-soft-*"},
		},
	})
	require.NoError(t, err)

	// Before assigning user to application, we should have:
	// - Access List for the application
	// - App server for each application link
	t.Run("before assigning user to application", func(t *testing.T) {
		// Wait for Access List sync.
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// Make sure there is no Access Lists, because the application has no assignments.
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
			appServers, err := sut.Teleport.Process.GetAuthServer().GetApplicationServers(ctx, defaults.Namespace)
			require.NoError(t, err)
			require.Len(t, appServers, 4) // 3 for app links + 1 for the connector SAML app
			var mySoft365AppServers []types.AppServer
			for _, appServer := range appServers {
				if appServer.GetAllLabels()["teleport.internal/okta-app-name"] == "my-soft-365" {
					mySoft365AppServers = append(mySoft365AppServers, appServer)
				}
			}
			require.Len(t, mySoft365AppServers, 3) // for each app link
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
		}, time.Second*2, time.Millisecond*50)
	})

	// After assigning user to application, we should have additionally:
	// - access and reviewer system roles created
	// - App server for each application link
	t.Run("after assigning user to application", func(t *testing.T) {
		// Assign user to application so we have Access List and corresponding access and review
		// system roles created.
		_, _, err := oktaApiClient.AssignUserToApplication(ctx, app.Id, oktasdk.AppUser{Id: user.Id})
		require.NoError(t, err)

		// Wait for Access List sync.
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

		resourceSuffix := testResourceSuffix(t, app.Id, appLinks)

		// Ensure there is 1 Access List.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			require.NoError(t, err)
			require.Len(t, accessLists, 1)
			require.Equal(t, resourceSuffix, accessLists[0].GetName())
		}, time.Second*2, time.Millisecond*50)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// Ensure there are access and reviewer system roles and their names are stable,
			// i.e. created from the first app link.
			roles, err := sut.Teleport.Process.GetAuthServer().GetRoles(ctx)
			require.NoError(t, err)
			var roleNames []string
			for _, r := range roles {
				roleNames = append(roleNames, r.GetName())
			}
			require.Contains(t, roleNames, "my-soft-365-access-okta-acl-role-"+resourceSuffix)
			require.Contains(t, roleNames, "my-soft-365-reviewer-okta-acl-role-"+resourceSuffix)
			accessListRolesCnt := 0
			for _, r := range roleNames {
				if strings.HasPrefix(r, "my-soft-365") {
					accessListRolesCnt++
				}
			}
			require.Equal(t, 2, accessListRolesCnt)
		}, time.Second*15, time.Millisecond*50)
	})
}

func TestAccessListSync_bidirectionalSync(t *testing.T) {
	var err error
	ctx := context.Background()

	// Setup Okta mock.
	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaClient := oktaapi.NewForAPIClient(oktaApiClient)

	// Create Okta SAML app.
	connectorSamlApp := createOktaSAMLAPP(t, ctx, oktaApiClient, "trial-1234567_teleportsamlconnectorapp_1")

	// Create and assign Okta SAML app users.
	user1, _ := createOktaUser(t, ctx, oktaApiClient, "bob")
	user2, _ := createOktaUser(t, ctx, oktaApiClient, "alice")
	err = oktaClient.AssignUserToApplication(ctx, oktaapi.OktaUserID(user1.Id), oktaapi.OktaAppID(connectorSamlApp.Id))
	require.NoError(t, err)
	err = oktaClient.AssignUserToApplication(ctx, oktaapi.OktaUserID(user2.Id), oktaapi.OktaAppID(connectorSamlApp.Id))
	require.NoError(t, err)

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()

	// 1. Create integration with bidirectional sync disabled.

	_, err = oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:      durationpb.New(1 * time.Second),
		ApiCredentials:          apiCredentials,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: false, // disabled
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
		ReuseConnector: "okta-pre-created-test",
	})
	require.NoError(t, err)

	// 2. Wait for the connector SAML app users to be syncrhonized.

	var oktaUsers []types.User
	mustWaitForEvent(t, sut, events.OktaUserSyncEvent)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		users, err := sut.Teleport.Process.GetAuthServer().GetUsers(ctx, false /* withSecrets */)
		require.NoError(t, err)
		oktaUsers = oktaUsers[:0] // clear
		for _, u := range users {
			if v, _ := u.GetLabel("teleport.dev/origin"); v == "okta" {
				oktaUsers = append(oktaUsers, u)
			}
		}
		require.Len(t, oktaUsers, 2, "expected 2 Okta users in all_users = %v", users)
	}, time.Second*2, time.Millisecond*50)

	// 3. Ensure there are okta_assignments for each user and they are "pending"

	mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		oktaAssignments, nextToken, err := authServer.ListOktaAssignments(ctx, 1000, "")
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Len(t, oktaAssignments, 2)
		for _, assignment := range oktaAssignments {
			require.Equal(t, constants.OktaAssignmentStatusPending, assignment.GetStatus(), "okta_assignment for user = %q", assignment.GetUser())
		}
	}, time.Second*2, time.Millisecond*50)

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

	mustUpdateOktaIntegration(ctx, t, oktaAuthClient, &oktav1.UpdateIntegrationRequest{
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true, // enabled
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
	})

	// 6. Wait for AssignmentProcessor event and verify okta_assignments are in "successful" state

	mustWaitForEvent(t, sut, events.OktaAssignmentProcessEvent)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		oktaAssignments, nextToken, err := authServer.ListOktaAssignments(ctx, 1000, "")
		require.NoError(t, err)
		require.Empty(t, nextToken)
		require.Len(t, oktaAssignments, 2)
		for _, assignment := range oktaAssignments {
			require.Equal(t, constants.OktaAssignmentStatusSuccessful, assignment.GetStatus(), "okta_assignment for user = %q", assignment.GetUser())
		}
	}, time.Second*4, time.Millisecond*50)
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
