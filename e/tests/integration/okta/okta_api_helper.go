package okta

import (
	"context"
	"testing"

	oktasdk "github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
)

var apiCredentials = &oktav1.OktaAPICredentials{
	Auth: &oktav1.OktaAPICredentials_OauthId{
		OauthId: "test-oauth-client-id-12345",
	},
}

func mustCreateOktaEveryoneGroupAndAssignOktaUsers(t *testing.T, oktaClient *mockOktaAPIClient, users ...*oktaUserType) {
	g := oktasdk.Group{
		Type: "BUILT_IN",
		Profile: &oktasdk.GroupProfile{
			Name: "Everyone",
		},
		Links: map[string]string{
			"href": "https://12345.okta.com/api/v1/apps/0oailpy80iMX0bjlT697/sso/saml/metadata",
			"type": "application/xml",
		},
	}
	everyoneGroup, _, err := oktaClient.CreateGroup(context.Background(), g)
	require.NoError(t, err)
	for _, user := range users {
		_, err := oktaClient.AddUserToGroup(context.Background(), everyoneGroup.Id, user.Id)
		require.NoError(t, err)
	}
}

func requireGroupAssignments(t require.TestingT, oktaAPIClient oktaapi.APIClient, groupID string, expectedUserIDs ...string) {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}
	ctx := context.Background()
	oktaClient := oktaapi.NewForAPIClient(oktaAPIClient)
	userIDs, err := oktaClient.GetGroupAssignments(ctx, oktaapi.OktaGroupID(groupID))
	require.NoError(t, err, "oktaClient.GetGroupAssignments")
	require.Len(t, userIDs, len(expectedUserIDs))
	for _, userID := range expectedUserIDs {
		require.Contains(t, userIDs, oktaapi.OktaUserID(userID))
	}
}

func requireNoGroupAssignments(t require.TestingT, oktaAPIClient oktaapi.APIClient, groupID string) {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}
	requireGroupAssignments(t, oktaAPIClient, groupID)
}
