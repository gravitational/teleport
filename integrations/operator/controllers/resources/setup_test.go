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

package resources

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestEnabledReconcilers(t *testing.T) {
	// Test setup: create the fixtures
	ossFeatures := modulestest.OSSModules().Features()
	entFeatures := modulestest.EnterpriseModules().Features()
	entFeatures.Entitlements[entitlements.OIDC] = modules.EntitlementInfo{Enabled: true}
	entFeatures.Entitlements[entitlements.SAML] = modules.EntitlementInfo{Enabled: true}
	entFeatures.Entitlements[entitlements.Policy] = modules.EntitlementInfo{Enabled: true}
	entFeatures.AdvancedAccessWorkflows = true

	tests := []struct {
		name                string
		scoped              bool
		features            modules.Features
		expectedReconcilers map[string]struct{}
	}{
		{
			name:     "Unscoped, OSS",
			scoped:   false,
			features: ossFeatures,
			expectedReconcilers: map[string]struct{}{
				// scoped
				"TeleportScopedTokenV1":          {},
				"TeleportScopedRoleV1":           {},
				"TeleportScopedRoleAssignmentV1": {},
				// unscoped oss
				"TeleportRole":                     {},
				"TeleportRoleV6":                   {},
				"TeleportRoleV7":                   {},
				"TeleportRoleV8":                   {},
				"TeleportUser":                     {},
				"TeleportGithubConnector":          {},
				"TeleportLockV2":                   {},
				"TeleportProvisionToken":           {},
				"TeleportOpenSSHServerV2":          {},
				"TeleportOpenSSHEICEServerV2":      {},
				"TeleportTrustedClusterV2":         {},
				"TeleportBotV1":                    {},
				"TeleportWorkloadIdentityV1":       {},
				"TeleportAutoupdateConfigV1":       {},
				"TeleportAutoupdateVersionV1":      {},
				"TeleportAppV3":                    {},
				"TeleportDatabaseV3":               {},
				"TeleportAccessMonitoringRuleV1":   {},
				"TeleportSAMLIdPServiceProviderV1": {},
			},
		},
		{
			name:     "Scoped, OSS",
			scoped:   true,
			features: ossFeatures,
			expectedReconcilers: map[string]struct{}{
				"TeleportScopedTokenV1":          {},
				"TeleportScopedRoleV1":           {},
				"TeleportScopedRoleAssignmentV1": {},
			},
		},
		{
			name:     "Unscoped, enterprise",
			scoped:   false,
			features: entFeatures,
			expectedReconcilers: map[string]struct{}{
				// scoped
				"TeleportScopedTokenV1":          {},
				"TeleportScopedRoleV1":           {},
				"TeleportScopedRoleAssignmentV1": {},
				// unscoped oss
				"TeleportRole":                     {},
				"TeleportRoleV6":                   {},
				"TeleportRoleV7":                   {},
				"TeleportRoleV8":                   {},
				"TeleportUser":                     {},
				"TeleportGithubConnector":          {},
				"TeleportLockV2":                   {},
				"TeleportProvisionToken":           {},
				"TeleportOpenSSHServerV2":          {},
				"TeleportOpenSSHEICEServerV2":      {},
				"TeleportTrustedClusterV2":         {},
				"TeleportBotV1":                    {},
				"TeleportWorkloadIdentityV1":       {},
				"TeleportAutoupdateConfigV1":       {},
				"TeleportAutoupdateVersionV1":      {},
				"TeleportAppV3":                    {},
				"TeleportDatabaseV3":               {},
				"TeleportAccessMonitoringRuleV1":   {},
				"TeleportSAMLIdPServiceProviderV1": {},
				// unscoped enterprise
				"TeleportOIDCConnector":    {},
				"TeleportSAMLConnector":    {},
				"TeleportInferenceModel":   {},
				"TeleportInferencePolicy":  {},
				"TeleportInferenceSecret":  {},
				"TeleportRetrievalModelV1": {},
				"TeleportLoginRule":        {},
				"TeleportAccessList":       {},
				"TeleportOktaImportRule":   {},
			},
		},

		{
			name:     "Scoped, enterprise",
			scoped:   true,
			features: entFeatures,
			expectedReconcilers: map[string]struct{}{
				"TeleportScopedTokenV1":          {},
				"TeleportScopedRoleV1":           {},
				"TeleportScopedRoleAssignmentV1": {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			log := logr.FromSlogHandler(logtest.NewLogger().Handler())
			features := tt.features.ToProto()
			reconcilers := enabledReconcilers(log, features, tt.scoped)

			// Test validation: check that the expected reconcilers were enabled
			reconcilersSet := make(map[string]struct{}, len(reconcilers))
			for _, reconciler := range reconcilers {
				reconcilersSet[reconciler.cr] = struct{}{}
			}
			require.Len(t, reconcilersSet, len(tt.expectedReconcilers))
			require.Equal(t, tt.expectedReconcilers, reconcilersSet)
		})
	}
}
