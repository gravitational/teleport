//go:build ent

package testlib

import (
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestTerraformEnterprise(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			OIDC:                    true,
			SAML:                    true,
			AdvancedAccessWorkflows: true,
		},
	})

	authPlugin, err := eauth.NewPlugin(eauth.Config{
		License: eauth.ValidLicense{},
	})
	require.NoError(t, err)

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Add(authPlugin))

	suite.Run(t, &TerraformSuiteEnterprise{
		TerraformBaseSuite: TerraformBaseSuite{
			AuthHelper: &integration.MinimalAuthHelper{
				PluginRegistry: registry,
			},
		},
	})
}

func TestTerraformEnterpriseWithCache(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			OIDC:                    true,
			SAML:                    true,
			AdvancedAccessWorkflows: true,
		},
	})

	authPlugin, err := eauth.NewPlugin(eauth.Config{
		License: eauth.ValidLicense{},
	})
	require.NoError(t, err)

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Add(authPlugin))

	suite.Run(t, &TerraformSuiteEnterpriseWithCache{
		TerraformBaseSuite: TerraformBaseSuite{
			AuthHelper: &integration.MinimalAuthHelper{
				AuthConfig:     auth.TestAuthServerConfig{CacheEnabled: true},
				PluginRegistry: registry,
			},
		},
	})
}
