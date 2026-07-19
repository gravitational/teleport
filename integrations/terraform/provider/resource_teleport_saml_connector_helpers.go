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

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

const samlConnectorSchemaVersion int64 = 1

func samlConnectorSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	schema, diags := tfschema.GenSchemaSAMLConnectorV2(ctx)
	schema.Version = samlConnectorSchemaVersion
	return schema, diags
}

func upgradeSAMLConnectorState(ctx context.Context) map[int64]tfsdk.ResourceStateUpgrader {
	schema, _ := tfschema.GenSchemaSAMLConnectorV2(ctx)
	return map[int64]tfsdk.ResourceStateUpgrader{
		0: {
			PriorSchema:   &schema,
			StateUpgrader: upgradeSAMLConnectorStateV0,
		},
	}
}

func upgradeSAMLConnectorStateV0(ctx context.Context, req tfsdk.UpgradeResourceStateRequest, resp *tfsdk.UpgradeResourceStateResponse) {
	if req.State == nil {
		resp.Diagnostics.AddError("Unable to Upgrade SAMLConnector State", "Prior SAMLConnector state is unavailable.")
		return
	}

	var state types.Object
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clearSAMLConnectorStatePrivateKeys(&state)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func clearSAMLConnectorStatePrivateKeys(state *types.Object) {
	clearSAMLConnectorStateString(state, "spec", "signing_key_pair", "private_key")
	clearSAMLConnectorStateString(state, "spec", "assertion_key_pair", "private_key")
}

func clearSAMLConnectorStateString(root *types.Object, path ...string) {
	if len(path) == 0 || root.Attrs == nil {
		return
	}
	if len(path) == 1 {
		if _, ok := root.Attrs[path[0]].(types.String); ok {
			root.Attrs[path[0]] = types.String{Null: true}
		}
		return
	}

	current, ok := root.Attrs[path[0]].(types.Object)
	if !ok || current.Null {
		return
	}

	clearSAMLConnectorStateString(&current, path[1:]...)
	root.Attrs[path[0]] = current
}

func preserveSAMLConnectorConfiguredSecrets(config, plan types.Object, target *apitypes.SAMLConnectorV2) {
	preserveSAMLConnectorConfiguredString(config, plan, target, setSAMLSigningPrivateKey, "spec", "signing_key_pair", "private_key")
	preserveSAMLConnectorConfiguredString(config, plan, target, setSAMLEncryptionPrivateKey, "spec", "assertion_key_pair", "private_key")
	preserveSAMLConnectorConfiguredString(config, plan, target, setSAMLOAuthClientSecret, "spec", "credentials", "oauth", "client_secret")
}

func preserveSAMLConnectorStateSecrets(state types.Object, target *apitypes.SAMLConnectorV2) {
	if value, ok := samlConnectorTerraformStringValue(state, "spec", "signing_key_pair", "private_key"); ok {
		setSAMLSigningPrivateKey(target, value)
	}
	if value, ok := samlConnectorTerraformStringValue(state, "spec", "assertion_key_pair", "private_key"); ok {
		setSAMLEncryptionPrivateKey(target, value)
	}
	if value, ok := samlConnectorTerraformStringValue(state, "spec", "credentials", "oauth", "client_secret"); ok {
		setSAMLOAuthClientSecret(target, value)
	}
}

func preserveSAMLConnectorConfiguredString(config, plan types.Object, target *apitypes.SAMLConnectorV2, set func(*apitypes.SAMLConnectorV2, string), path ...string) {
	value, configured := samlConnectorTerraformStringValue(config, path...)
	if !configured {
		return
	}
	// Unknown config values are still configured; use the resolved planned
	// value before writing post-apply state.
	if value == "" {
		var ok bool
		value, ok = samlConnectorTerraformStringValue(plan, path...)
		if !ok {
			return
		}
	}
	set(target, value)
}

func samlConnectorTerraformStringValue(root types.Object, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}

	current := root
	for _, name := range path[:len(path)-1] {
		value, ok := current.Attrs[name].(types.Object)
		if !ok || value.Null {
			return "", false
		}
		current = value
	}

	value, ok := current.Attrs[path[len(path)-1]].(types.String)
	if !ok || value.Null {
		return "", false
	}
	if value.Unknown {
		return "", true
	}
	return value.Value, value.Value != ""
}

func setSAMLSigningPrivateKey(connector *apitypes.SAMLConnectorV2, privateKey string) {
	if connector.Spec.SigningKeyPair == nil {
		connector.Spec.SigningKeyPair = &apitypes.AsymmetricKeyPair{}
	}
	connector.Spec.SigningKeyPair.PrivateKey = privateKey
}

func setSAMLEncryptionPrivateKey(connector *apitypes.SAMLConnectorV2, privateKey string) {
	if connector.Spec.EncryptionKeyPair == nil {
		connector.Spec.EncryptionKeyPair = &apitypes.AsymmetricKeyPair{}
	}
	connector.Spec.EncryptionKeyPair.PrivateKey = privateKey
}

func setSAMLOAuthClientSecret(connector *apitypes.SAMLConnectorV2, clientSecret string) {
	credentials := connector.GetOAuthClientCredentials()
	if credentials == nil {
		credentials = &apitypes.OAuthClientCredentials{}
	}
	credentials.ClientSecret = clientSecret
	connector.SetOAuthClientCredentials(credentials)
}
