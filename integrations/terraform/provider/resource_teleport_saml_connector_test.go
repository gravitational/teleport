/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"testing"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

func TestPreserveSAMLConnectorConfiguredSecrets(t *testing.T) {
	target := &apitypes.SAMLConnectorV2{
		Spec: apitypes.SAMLConnectorSpecV2{
			SigningKeyPair:    &apitypes.AsymmetricKeyPair{Cert: "signing-cert"},
			EncryptionKeyPair: &apitypes.AsymmetricKeyPair{Cert: "assertion-cert"},
			Credentials: &apitypes.SAMLConnectorCredentials{
				Oauth: &apitypes.OAuthClientCredentials{ClientId: "client-id"},
			},
		},
	}

	config := samlConnectorSecretsObject("signing-key", "assertion-key", "client-secret")
	preserveSAMLConnectorConfiguredSecrets(config, types.Object{}, target)

	require.Equal(t, "signing-key", target.Spec.SigningKeyPair.PrivateKey)
	require.Equal(t, "assertion-key", target.Spec.EncryptionKeyPair.PrivateKey)
	require.Equal(t, "client-secret", target.Spec.Credentials.Oauth.ClientSecret)
}

func TestPreserveSAMLConnectorConfiguredSecretsIgnoresUnconfiguredPlanSecrets(t *testing.T) {
	target := &apitypes.SAMLConnectorV2{
		Spec: apitypes.SAMLConnectorSpecV2{
			SigningKeyPair:    &apitypes.AsymmetricKeyPair{Cert: "signing-cert"},
			EncryptionKeyPair: &apitypes.AsymmetricKeyPair{Cert: "assertion-cert"},
			Credentials: &apitypes.SAMLConnectorCredentials{
				Oauth: &apitypes.OAuthClientCredentials{ClientId: "client-id"},
			},
		},
	}

	preserveSAMLConnectorConfiguredSecrets(types.Object{}, samlConnectorSecretsObject("signing-key", "assertion-key", "client-secret"), target)

	require.Empty(t, target.Spec.SigningKeyPair.PrivateKey)
	require.Empty(t, target.Spec.EncryptionKeyPair.PrivateKey)
	require.Empty(t, target.Spec.Credentials.Oauth.ClientSecret)
}

func TestPreserveSAMLConnectorConfiguredSecretsFallsBackToResolvedPlan(t *testing.T) {
	target := &apitypes.SAMLConnectorV2{
		Spec: apitypes.SAMLConnectorSpecV2{
			SigningKeyPair: &apitypes.AsymmetricKeyPair{Cert: "signing-cert"},
		},
	}

	config := samlConnectorSecretsObject("", "", "")
	configSpec := config.Attrs["spec"].(types.Object)
	signingKeyPair := configSpec.Attrs["signing_key_pair"].(types.Object)
	signingKeyPair.Attrs["private_key"] = types.String{Unknown: true}
	configSpec.Attrs["signing_key_pair"] = signingKeyPair
	config.Attrs["spec"] = configSpec

	preserveSAMLConnectorConfiguredSecrets(config, samlConnectorSecretsObject("resolved-signing-key", "", ""), target)

	require.Equal(t, "resolved-signing-key", target.Spec.SigningKeyPair.PrivateKey)
}

func TestSAMLConnectorSchemaUpgradeState(t *testing.T) {
	ctx := context.Background()
	schema, diags := samlConnectorSchema(ctx)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, samlConnectorSchemaVersion, schema.Version)

	upgraders := upgradeSAMLConnectorState(ctx)
	require.Contains(t, upgraders, int64(0))
	require.NotNil(t, upgraders[0].PriorSchema)
	require.NotNil(t, upgraders[0].StateUpgrader)
}

func TestUpgradeSAMLConnectorStateV0ClearsPriorPrivateKeys(t *testing.T) {
	state := samlConnectorSecretsObject("old-signing-key", "old-assertion-key", "old-client-secret")

	clearSAMLConnectorStatePrivateKeys(&state)

	requireSAMLConnectorSecretValue(t, state, "", "spec", "signing_key_pair", "private_key")
	requireSAMLConnectorSecretValue(t, state, "", "spec", "assertion_key_pair", "private_key")
	requireSAMLConnectorSecretValue(t, state, "old-client-secret", "spec", "credentials", "oauth", "client_secret")
}

func TestSAMLConnectorReadbackPreservesPriorStateSecrets(t *testing.T) {
	ctx := context.Background()
	state := samlConnectorSecretsObject("old-signing-key", "old-assertion-key", "old-client-secret")
	setSAMLConnectorStateTypes(t, ctx, &state)

	samlConnector := &apitypes.SAMLConnectorV2{
		Spec: apitypes.SAMLConnectorSpecV2{
			SigningKeyPair:    &apitypes.AsymmetricKeyPair{Cert: "signing-cert"},
			EncryptionKeyPair: &apitypes.AsymmetricKeyPair{Cert: "assertion-cert"},
			Credentials: &apitypes.SAMLConnectorCredentials{
				Oauth: &apitypes.OAuthClientCredentials{ClientId: "client-id"},
			},
		},
	}

	preserveSAMLConnectorStateSecrets(state, samlConnector)
	diags := tfschema.CopySAMLConnectorV2ToTerraform(ctx, samlConnector, &state)
	require.False(t, diags.HasError(), diags)
	requireSAMLConnectorSecretValue(t, state, "old-signing-key", "spec", "signing_key_pair", "private_key")
	requireSAMLConnectorSecretValue(t, state, "old-assertion-key", "spec", "assertion_key_pair", "private_key")
	requireSAMLConnectorSecretValue(t, state, "old-client-secret", "spec", "credentials", "oauth", "client_secret")
}

func samlConnectorSecretsObject(signingPrivateKey, assertionPrivateKey, clientSecret string) types.Object {
	return types.Object{
		Attrs: map[string]attr.Value{
			"spec": types.Object{
				Attrs: map[string]attr.Value{
					"signing_key_pair": types.Object{
						Attrs: map[string]attr.Value{
							"private_key": types.String{Value: signingPrivateKey},
						},
					},
					"assertion_key_pair": types.Object{
						Attrs: map[string]attr.Value{
							"private_key": types.String{Value: assertionPrivateKey},
						},
					},
					"credentials": types.Object{
						Attrs: map[string]attr.Value{
							"oauth": types.Object{
								Attrs: map[string]attr.Value{
									"client_secret": types.String{Value: clientSecret},
								},
							},
						},
					},
				},
			},
		},
	}
}

func setSAMLConnectorStateTypes(t *testing.T, ctx context.Context, state *types.Object) {
	schema, diags := tfschema.GenSchemaSAMLConnectorV2(ctx)
	require.False(t, diags.HasError(), diags)

	rootType, ok := schema.AttributeType().(types.ObjectType)
	require.True(t, ok)
	state.AttrTypes = rootType.AttrTypes

	specType := requireObjectType(t, rootType.AttrTypes["spec"])
	spec := state.Attrs["spec"].(types.Object)
	spec.AttrTypes = specType.AttrTypes

	signingKeyPair := spec.Attrs["signing_key_pair"].(types.Object)
	signingKeyPair.AttrTypes = requireObjectType(t, specType.AttrTypes["signing_key_pair"]).AttrTypes
	spec.Attrs["signing_key_pair"] = signingKeyPair

	assertionKeyPair := spec.Attrs["assertion_key_pair"].(types.Object)
	assertionKeyPair.AttrTypes = requireObjectType(t, specType.AttrTypes["assertion_key_pair"]).AttrTypes
	spec.Attrs["assertion_key_pair"] = assertionKeyPair

	credentials := spec.Attrs["credentials"].(types.Object)
	credentials.AttrTypes = requireObjectType(t, specType.AttrTypes["credentials"]).AttrTypes

	oauth := credentials.Attrs["oauth"].(types.Object)
	oauth.AttrTypes = requireObjectType(t, credentials.AttrTypes["oauth"]).AttrTypes
	credentials.Attrs["oauth"] = oauth

	spec.Attrs["credentials"] = credentials
	state.Attrs["spec"] = spec
}

func requireObjectType(t *testing.T, value attr.Type) types.ObjectType {
	t.Helper()
	objectType, ok := value.(types.ObjectType)
	require.True(t, ok)
	return objectType
}

func requireSAMLConnectorSecretValue(t *testing.T, root types.Object, expected string, path ...string) {
	t.Helper()
	require.NotEmpty(t, path)

	current := root
	for _, name := range path[:len(path)-1] {
		value, ok := current.Attrs[name].(types.Object)
		require.True(t, ok)
		current = value
	}

	value, ok := current.Attrs[path[len(path)-1]].(types.String)
	require.True(t, ok)
	require.Equal(t, expected, value.Value)
	require.False(t, value.Unknown)
}
