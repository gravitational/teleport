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

package testlib

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
)

var testModules = &modulestest.Modules{
	TestFeatures: modules.Features{
		AdvancedAccessWorkflows: true,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.OIDC:        {Enabled: true},
			entitlements.SAML:        {Enabled: true},
			entitlements.DeviceTrust: {Enabled: true},
			entitlements.Policy:      {Enabled: true},
		},
	},
}

const entTestSkipMessage = "Skipping Enterprise test suite, auth plugin is missing. \n" +
	"If you are running tests locally and have `e/` checked out: use `make test-ent` once, or run `go generate testlib/plugin_test.go`"

func TestTerraformEnterprise(t *testing.T) {
	authPlugin, err := NewPlugin(testModules)
	if trace.IsNotImplemented(err) {
		t.Skip(entTestSkipMessage)
	}
	require.NoError(t, err)

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Add(authPlugin))

	suite.Run(t, &TerraformSuiteEnterprise{
		TerraformBaseSuite: TerraformBaseSuite{
			AuthHelper: &integration.MinimalAuthHelper{
				PluginRegistry: registry,
				AuthConfig: authtest.AuthServerConfig{
					Modules: testModules,
				},
			},
		},
	})
}

func TestTerraformEnterpriseWithCache(t *testing.T) {
	authPlugin, err := NewPlugin(testModules)
	if trace.IsNotImplemented(err) {
		t.Skip(entTestSkipMessage)
	}
	require.NoError(t, err)

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Add(authPlugin))

	suite.Run(t, &TerraformSuiteEnterpriseWithCache{
		TerraformBaseSuite: TerraformBaseSuite{
			AuthHelper: &integration.MinimalAuthHelper{
				AuthConfig: authtest.AuthServerConfig{
					CacheEnabled: true,
					Modules:      testModules,
				},
				PluginRegistry: registry,
			},
		},
	})
}
