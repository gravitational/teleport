package okta

import (
	"context"
	"testing"

	oktasdk "github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
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
