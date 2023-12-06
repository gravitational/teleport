package modules

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGetLicenseFeatures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc         string
		license      func() types.License
		wantFeatures modules.Features
	}{
		{
			desc: "legacy non-usage based",
			license: func() types.License {
				license, error := types.NewLicense("license", types.LicenseSpecV3{})
				require.NoError(t, error)
				return license
			},
			// All hard coded values.
			wantFeatures: modules.Features{
				App:                     true,
				OIDC:                    true,
				SAML:                    true,
				AccessControls:          true,
				AdvancedAccessWorkflows: true,
				HSM:                     true,
				Assist:                  true,
				DeviceTrust: modules.DeviceTrustFeature{
					Enabled: true,
				},
			},
		},
		{
			desc: "eub with limits",
			license: func() types.License {
				license, error := types.NewLicense("license", types.LicenseSpecV3{
					UsageBasedBilling: true,
				})
				require.NoError(t, error)
				return license
			},
			wantFeatures: modules.Features{
				IsUsageBasedBilling:     true,
				Assist:                  true,
				App:                     true,
				OIDC:                    true,
				SAML:                    true,
				AccessControls:          true,
				AdvancedAccessWorkflows: true,
				HSM:                     true,
				DeviceTrust:             feature.GetUsageBasedDeviceTrustFeatureLimits(),
				AccessRequests:          feature.GetUsageBasedAccessRequestFeatureLimits(),
				AccessList:              feature.GetUsageBasedAccessListFeatureLimits(),
				AccessMonitoring:        feature.GetUsageBasedAccessMonitoringFeatureLimits(false),
				ProductType:             modules.ProductTypeEUB,
			},
		},
		{
			desc: "eub without limits (igs enabled)",
			license: func() types.License {
				license, error := types.NewLicense("license", types.LicenseSpecV3{
					UsageBasedBilling:                  true,
					SupportsIdentityGovernanceSecurity: true,
				})
				require.NoError(t, error)
				return license
			},
			wantFeatures: modules.Features{
				IsUsageBasedBilling:     true,
				Assist:                  true,
				App:                     true,
				OIDC:                    true,
				SAML:                    true,
				AccessControls:          true,
				AdvancedAccessWorkflows: true,
				HSM:                     true,
				DeviceTrust: modules.DeviceTrustFeature{
					Enabled:           true,
					DevicesUsageLimit: 0,
				},
				AccessRequests: modules.AccessRequestsFeature{
					MonthlyRequestLimit: 0,
				},
				AccessList: modules.AccessListFeature{
					CreateLimit: 0,
				},
				AccessMonitoring: modules.AccessMonitoringFeature{
					Enabled:             false,
					MaxReportRangeLimit: 0,
				},
				ProductType:                modules.ProductTypeEUB,
				IdentityGovernanceSecurity: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotFeatures := getLicenseFeatures(tc.license())
			if diff := cmp.Diff(tc.wantFeatures, gotFeatures); diff != "" {
				t.Errorf("Feature diff (-want +got):\n%s", diff)
			}
		})
	}
}
