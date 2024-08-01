package okta

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func requireNotFound(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func TestSelectSelectCredentials(t *testing.T) {
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
						CredPurposeLabel: CredPurposeSCIMToken,
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
					CredPurposeLabel: CredPurposeOktaAuth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some token",
			},
		},
	}

	type testCase struct {
		name         string
		input        []types.PluginStaticCredentials
		expectedErr  require.ErrorAssertionFunc
		expectedCred types.PluginStaticCredentials
	}

	apiTokenTestCases := []testCase{
		{
			name:        "empty",
			expectedErr: requireNotFound,
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
			name:        "multiple, all scim, fails",
			input:       scimCred,
			expectedErr: requireNotFound,
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

	scimTokenTestCases := []testCase{
		{
			name:        "empty",
			expectedErr: requireNotFound,
		}, {
			name:        "single, no purpose",
			input:       []types.PluginStaticCredentials{unlabeled[0]},
			expectedErr: requireNotFound,
		}, {
			name: "mixed API and no purpose",
			input: []types.PluginStaticCredentials{
				unlabeled[1], oktaToken, unlabeled[0],
			},
			expectedErr: requireNotFound,
		}, {
			name:         "single SCIM token",
			input:        []types.PluginStaticCredentials{scimCred[2]},
			expectedErr:  require.NoError,
			expectedCred: scimCred[2],
		}, {
			name: "mixed tokens",
			input: []types.PluginStaticCredentials{
				unlabeled[2], oktaToken, scimCred[1], unlabeled[1],
			},
			expectedErr:  require.NoError,
			expectedCred: scimCred[1],
		}, {
			name: "multiple tokens, picks first",
			input: []types.PluginStaticCredentials{
				scimCred[3], scimCred[0], scimCred[1], scimCred[2],
			},
			expectedErr:  require.NoError,
			expectedCred: scimCred[3],
		},
	}

	t.Run("APIToken", func(t *testing.T) {
		for _, test := range apiTokenTestCases {
			t.Run(test.name, func(t *testing.T) {
				cred, err := SelectAPIToken(test.input)
				test.expectedErr(t, err)
				if test.expectedCred == nil {
					require.Nil(t, cred)
				} else {
					require.Same(t, test.expectedCred, cred)
				}
			})
		}
	})

	t.Run("SCIMToken", func(t *testing.T) {
		for _, test := range scimTokenTestCases {
			t.Run(test.name, func(t *testing.T) {
				cred, err := SelectSCIMToken(test.input)
				test.expectedErr(t, err)
				if test.expectedCred == nil {
					require.Nil(t, cred)
				} else {
					require.Same(t, test.expectedCred, cred)
				}
			})
		}
	})
}
