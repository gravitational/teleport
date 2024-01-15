package plugins

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
)

func requireNotFound(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func TestSelectOktaAPICredentials(t *testing.T) {
	const ncreds = 4
	unlabeled := make([]types.PluginStaticCredentials, ncreds)
	for i := 0; i < ncreds; i++ {
		unlabeled[i] = &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeOkta,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "some token",
				},
			},
		}
	}

	scimCred := make([]types.PluginStaticCredentials, ncreds)
	for i := 0; i < ncreds; i++ {
		scimCred[i] = &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeOkta,
					Labels: map[string]string{
						okta.CredPurposeLabel: okta.CredPurposeSCIMToken,
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "some token",
				},
			},
		}
	}

	oktaToken := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeOkta,
				Labels: map[string]string{
					okta.CredPurposeLabel: okta.CredPurposeOktaAuth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some token",
			},
		},
	}

	testCases := []struct {
		name         string
		input        []types.PluginStaticCredentials
		expectedErr  require.ErrorAssertionFunc
		expectedCred types.PluginStaticCredentials
	}{
		{
			name:         "empty",
			input:        []types.PluginStaticCredentials{},
			expectedErr:  requireNotFound,
			expectedCred: nil,
		},
		{
			name:         "single, no purpose",
			input:        []types.PluginStaticCredentials{unlabeled[0]},
			expectedErr:  require.NoError,
			expectedCred: unlabeled[0],
		},
		{
			name:         "multiple, no labeled, picks first",
			input:        []types.PluginStaticCredentials{unlabeled[3], unlabeled[1]},
			expectedErr:  require.NoError,
			expectedCred: unlabeled[3],
		},
		{
			name:         "multiple, all scim, fails",
			input:        scimCred,
			expectedErr:  requireNotFound,
			expectedCred: nil,
		},
		{
			name: "multiple, one nopurpose, picks unlabeled",
			input: []types.PluginStaticCredentials{
				scimCred[3], scimCred[0], unlabeled[1], scimCred[2], unlabeled[0],
			},
			expectedErr:  require.NoError,
			expectedCred: unlabeled[1],
		}, {
			name: "multiple, one labeled oktatoken, picks labeled",
			input: []types.PluginStaticCredentials{
				scimCred[3], scimCred[0], oktaToken, scimCred[2], unlabeled[0],
			},
			expectedErr:  require.NoError,
			expectedCred: oktaToken,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cred, err := selectOktaAPIToken(test.input)
			test.expectedErr(t, err)
			if test.expectedCred == nil {
				require.Nil(t, cred)
			} else {
				require.Same(t, test.expectedCred, cred)
			}
		})
	}
}
