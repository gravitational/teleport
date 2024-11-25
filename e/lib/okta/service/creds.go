package oktaservice

import (
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func buildAPITokenCredentials(SSWSToken string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeOkta,
				Labels: map[string]string{
					common.CredPurposeLabel: common.CredPurposeOktaAuth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: SSWSToken,
			},
		},
	}
}

func buildOAuthCredentials(clientID string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeOkta,
				Labels: map[string]string{
					common.CredPurposeLabel: common.CredPurposeOktaOauth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     clientID,
					ClientSecret: "none",
				},
			},
		},
	}
}

func buildSCIMCredentials(scimToken string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: common.OktaSCIMTokenName,
				Labels: map[string]string{
					common.CredPurposeLabel: common.CredPurposeSCIMToken,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: scimToken,
			},
		},
	}
}

// getOktaPluginCredentials returns a list of plugin credentials based on the
// provided request in order to save then into plugin static credential backend storage.
// Depending on the plugin some credentials may be required or optional but this is validated
// by the plugin itself.
func getOktaPluginCredentials(req *oktapb.CreateIntegrationRequest) ([]*types.PluginStaticCredentialsV1, error) {
	var out []*types.PluginStaticCredentialsV1
	if req.GetApiCredentials().GetOauthId() != "" {
		out = append(out, buildOAuthCredentials(req.GetApiCredentials().GetOauthId()))
	}
	if token := req.GetApiCredentials().GetSswsBearerToken(); token != "" {
		out = append(out, buildAPITokenCredentials(token))
	}
	if modules.GetModules().Features().GetEntitlement(entitlements.OktaSCIM).Enabled {
		if req.GetScimToken() != "" {
			scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(req.GetScimToken()), bcrypt.DefaultCost)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out = append(out, buildSCIMCredentials(string(scimTokenHash)))
		}
	}
	return out, nil
}
