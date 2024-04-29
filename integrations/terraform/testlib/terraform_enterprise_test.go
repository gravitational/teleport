//go:build enterprisetests

/*
Copyright 2024 Gravitational, Inc.

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

package testlib

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	eintegration "github.com/gravitational/teleport/e/integration"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
)

func TestTerraformEnterprise(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			OIDC:                    true,
			SAML:                    true,
			AdvancedAccessWorkflows: true,
			DeviceTrust:             modules.DeviceTrustFeature{Enabled: true},
		},
	})

	authPlugin := eintegration.GetTestAuthPlugin(t)

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
