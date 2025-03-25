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

	"github.com/gravitational/teleport/api/defaults"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
)

func TestAccessListSync(t *testing.T) {
	ctx := context.Background()

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

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
	oktaApiClient.setRoundTripper(httpMock)

	// Create a user in Okta.
	user := createOktaUser(t, ctx, oktaApiClient, "oktan@test.com")
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
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:       apiCredentials,
		OktaOrganizationUrl:  oktaApiClient.GetOrgUrl(),
		EnableUserSync:       true,
		EnableAppGroupSync:   true,
		EnableAccessListSync: true,
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
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
			require.Len(t, appServers, 3)
			for _, appServer := range appServers {
				labels := appServer.GetAllLabels()
				require.Equal(t, "my-soft-365", labels["teleport.internal/okta-app-name"], "app with labels: %v", labels)
				require.Equal(t, app.Id, labels["teleport.internal/okta-app-id"], "app with labels: %v", labels)
			}
			var appServerDescriptions []string
			for _, appServer := range appServers {
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
		oktaApiClient.AssignUserToApplication(ctx, app.Id, oktasdk.AppUser{Id: user.Id})

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
