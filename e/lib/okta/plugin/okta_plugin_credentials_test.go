package oktaplugin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common"
)

type pluginStaticCredentialsDesc struct {
	apiToken string
	clientId string
	labels   map[string]string
}

func newPluginStaticCredentials(desc pluginStaticCredentialsDesc) types.PluginStaticCredentials {
	staticCreds := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeOkta,
				Labels: desc.labels,
			},
		},
	}
	if desc.apiToken != "" {
		staticCreds.Spec = &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: desc.apiToken,
			},
		}
	}
	if desc.clientId != "" {
		staticCreds.Spec = &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId: desc.clientId,
				},
			},
		}
	}
	return staticCreds
}

func getClientID(staticCred types.PluginStaticCredentials) string {
	clientID, _ := staticCred.GetOAuthClientSecret()
	return clientID
}

func Test_SelectOktaCredentials(t *testing.T) {
	const ncreds = 4
	unlabeled := make([]types.PluginStaticCredentials, ncreds)
	scimTokenHash := make([]types.PluginStaticCredentials, ncreds)
	oktaApiToken := make([]types.PluginStaticCredentials, ncreds)
	scimOnlyOktaApiToken := make([]types.PluginStaticCredentials, ncreds)
	oktaClientId := make([]types.PluginStaticCredentials, ncreds)
	for i := range ncreds {
		unlabeled[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test API token (%d)", i),
			labels:   nil,
		})
		scimTokenHash[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test SCIM token hash (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: types.OktaCredPurposeSCIMToken,
			},
		})
		oktaApiToken[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test Okta API token (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: types.OktaCredPurposeAuth,
			},
		})
		scimOnlyOktaApiToken[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test SCIM-only Okta API token (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: types.CredPurposeOKTAAPITokenWithSCIMOnlyIntegration,
			},
		})
		oktaClientId[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			clientId: fmt.Sprintf("test Okta OAuth Client ID (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: common.CredPurposeOktaOauth,
			},
		})
	}

	testCases := []struct {
		name     string
		input    []types.PluginStaticCredentials
		expected SelectedOktaCredentials
	}{
		{
			name:     "empty",
			expected: SelectedOktaCredentials{},
		},
		{
			name:     "SCIM tokens only",
			input:    scimTokenHash,
			expected: SelectedOktaCredentials{},
		},
		{
			name:  "single unlabelled token",
			input: []types.PluginStaticCredentials{unlabeled[0]},
			expected: SelectedOktaCredentials{
				ApiToken: unlabeled[0].GetAPIToken(),
			},
		},
		{
			name:  "multiple unlabeled tokens, picks first",
			input: []types.PluginStaticCredentials{unlabeled[3], unlabeled[1]},
			expected: SelectedOktaCredentials{
				ApiToken: unlabeled[3].GetAPIToken(),
			},
		},
		{
			name: "multiple unlabelled tokens, one labeled token, picks labeled",
			input: []types.PluginStaticCredentials{
				scimTokenHash[3], unlabeled[0], scimTokenHash[0], oktaApiToken[0], scimTokenHash[2], unlabeled[1],
			},
			expected: SelectedOktaCredentials{
				ApiToken: oktaApiToken[0].GetAPIToken(),
			},
		},
		{
			name: "multiple unlabelled tokens, one labeled token, one token for SCIM purpose only, picks the one for SCIM purpose",
			input: []types.PluginStaticCredentials{
				scimTokenHash[3], unlabeled[0], scimTokenHash[0], oktaApiToken[0], scimTokenHash[2], scimOnlyOktaApiToken[0], unlabeled[1],
			},
			expected: SelectedOktaCredentials{
				ApiToken:            scimOnlyOktaApiToken[0].GetAPIToken(),
				ApiTokenForSCIMOnly: true,
			},
		},
		{
			name:  "single OAuth credential",
			input: []types.PluginStaticCredentials{oktaClientId[0]},
			expected: SelectedOktaCredentials{
				OauthClientId: getClientID(oktaClientId[0]),
			},
		},
		{
			name: "multiple OAuth credentials, picks first",
			input: []types.PluginStaticCredentials{
				scimTokenHash[0], oktaApiToken[0], oktaClientId[0], oktaClientId[1], scimTokenHash[1], oktaClientId[2], scimTokenHash[2],
			},
			expected: SelectedOktaCredentials{
				OauthClientId: getClientID(oktaClientId[0]),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cred, err := SelectOktaCredentials(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, cred)
		})
	}

}

func Test_SelectScimTokenHash(t *testing.T) {
	const ncreds = 4
	unlabeled := make([]types.PluginStaticCredentials, ncreds)
	scimTokenHash := make([]types.PluginStaticCredentials, ncreds)
	oktaApiToken := make([]types.PluginStaticCredentials, ncreds)
	scimOnlyOktaApiToken := make([]types.PluginStaticCredentials, ncreds)
	oktaClientId := make([]types.PluginStaticCredentials, ncreds)
	for i := range ncreds {
		unlabeled[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test API token (%d)", i),
			labels:   nil,
		})
		scimTokenHash[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test SCIM token hash (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: types.OktaCredPurposeSCIMToken,
			},
		})
		oktaApiToken[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test Okta API token (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: types.OktaCredPurposeAuth,
			},
		})
		scimOnlyOktaApiToken[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			apiToken: fmt.Sprintf("test SCIM-only Okta API token (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: types.CredPurposeOKTAAPITokenWithSCIMOnlyIntegration,
			},
		})
		oktaClientId[i] = newPluginStaticCredentials(pluginStaticCredentialsDesc{
			clientId: fmt.Sprintf("test Okta OAuth Client ID (%d)", i),
			labels: map[string]string{
				types.OktaCredPurposeLabel: common.CredPurposeOktaOauth,
			},
		})
	}

	testCases := []struct {
		name                  string
		input                 []types.PluginStaticCredentials
		expectedFound         bool
		expectedScimTokenHash string
	}{
		{
			name:                  "empty",
			expectedFound:         false,
			expectedScimTokenHash: "",
		},
		{
			name:                  "single, no purpose",
			input:                 []types.PluginStaticCredentials{unlabeled[0]},
			expectedFound:         false,
			expectedScimTokenHash: "",
		},
		{
			name: "mixed API and no purpose",
			input: []types.PluginStaticCredentials{
				unlabeled[1], oktaApiToken[0], oktaClientId[0], unlabeled[0],
			},
			expectedFound:         false,
			expectedScimTokenHash: "",
		},
		{
			name:                  "single SCIM token",
			input:                 []types.PluginStaticCredentials{scimTokenHash[2]},
			expectedFound:         true,
			expectedScimTokenHash: scimTokenHash[2].GetAPIToken(),
		},
		{
			name: "mixed tokens",
			input: []types.PluginStaticCredentials{
				unlabeled[2], oktaClientId[0], oktaApiToken[0], scimTokenHash[1], unlabeled[1],
			},
			expectedFound:         true,
			expectedScimTokenHash: scimTokenHash[1].GetAPIToken(),
		},
		{
			name: "multiple tokens, picks first",
			input: []types.PluginStaticCredentials{
				scimTokenHash[3], scimTokenHash[0], scimTokenHash[1], scimTokenHash[2],
			},
			expectedFound:         true,
			expectedScimTokenHash: scimTokenHash[3].GetAPIToken(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scimTokenHash, ok, err := SelectSCIMTokenHash(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedFound, ok)
			require.Equal(t, tc.expectedScimTokenHash, scimTokenHash)
		})
	}
}
