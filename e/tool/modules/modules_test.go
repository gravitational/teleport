package modules

import (
	"testing"
	"time"

	"github.com/gravitational/license"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func TestSetModules_RecoveryCodes(t *testing.T) {
	t.Parallel()

	licenseFile := &licensefile.LicenseFile{
		KeyPair: &license.License{},
		License: &types.LicenseV3{
			Spec: types.LicenseSpecV3{
				Cloud: false,
			},
		},
	}

	m := NewEnterpriseModules(EnterpriseModulesConfig{
		Cloud:   true,
		License: licenseFile,
		Features: modules.Features{
			RecoveryCodes: false,
			Cloud:         false,
		},
	})

	// assert recovery codes are always enabled on cloud envs
	require.True(t, m.Features().RecoveryCodes)

	// Cleaning the env var will make it not a cloud env
	m = NewEnterpriseModules(EnterpriseModulesConfig{
		Cloud:   false,
		License: licenseFile,
		Features: modules.Features{
			RecoveryCodes: true,
			Cloud:         true,
		},
	})

	// assert recovery codes are always disabled on non-cloud envs
	require.False(t, m.Features().RecoveryCodes)
}

func TestGetSelfHostedLicenseFeatures_LegacyLicenseFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		license  func() types.License
		expected modules.Features
	}{
		{
			desc: "sets license and static fields; no custom logic, no entitlements",
			license: func() types.License {
				license, err := types.NewLicense("license", types.LicenseSpecV3{
					// set from license
					Cloud:                              false, // also recovery codes (same)
					CustomTheme:                        "some-theme",
					UsageBasedBilling:                  false,
					SupportsKubernetes:                 false,
					SupportsApplicationAccess:          types.NewBoolP(false),
					SupportsDatabaseAccess:             false,
					SupportsDesktopAccess:              false,
					SupportsFeatureHiding:              false,
					SupportsIdentityGovernanceSecurity: true,
					SupportsPolicy:                     false,
				})
				require.NoError(t, err)
				return license
			},
			expected: modules.Features{
				Cloud:                   false, // also recovery codes
				RecoveryCodes:           false,
				CustomTheme:             "some-theme",
				IsUsageBasedBilling:     false,
				ProductType:             modules.ProductTypeUnknown, // set per usage based
				SupportType:             proto.SupportType_SUPPORT_TYPE_PREMIUM,
				AccessControls:          true,
				AdvancedAccessWorkflows: true,
				Questionnaire:           false,
				IsStripeManaged:         false,
				Plugins:                 false,
				AutomaticUpgrades:       false,
				AccessGraph:             false,
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.AccessLists:            {Enabled: true},
					entitlements.AccessMonitoring:       {Enabled: true},
					entitlements.AccessRequests:         {Enabled: true},
					entitlements.App:                    {},
					entitlements.CloudAuditLogRetention: {},
					entitlements.DB:                     {},
					entitlements.Desktop:                {},
					entitlements.DeviceTrust:            {Enabled: true},
					entitlements.ExternalAuditStorage:   {},
					entitlements.FeatureHiding:          {},
					entitlements.HSM:                    {Enabled: true},
					entitlements.Identity:               {Enabled: true},
					entitlements.JoinActiveSessions:     {Enabled: true},
					entitlements.K8s:                    {},
					entitlements.MobileDeviceManagement: {Enabled: true},
					entitlements.OIDC:                   {Enabled: true},
					entitlements.OktaSCIM:               {Enabled: true},
					entitlements.OktaUserSync:           {Enabled: true},
					entitlements.Policy:                 {},
					entitlements.SAML:                   {Enabled: true},
					entitlements.SessionLocks:           {Enabled: true},
					entitlements.UpsellAlert:            {},
					entitlements.UsageReporting:         {},
				},
			},
		},
		{
			desc: "sets product type if usage based",
			license: func() types.License {
				license, err := types.NewLicense("license", types.LicenseSpecV3{
					UsageBasedBilling:                  true,
					SupportsIdentityGovernanceSecurity: true,
					Cloud:                              false,
					Entitlements:                       map[string]types.EntitlementInfo{},
				})
				require.NoError(t, err)
				return license
			},
			expected: modules.Features{
				ProductType:             modules.ProductTypeEUB, // set per usage based
				IsUsageBasedBilling:     true,
				Cloud:                   false,
				AccessControls:          true,
				SupportType:             proto.SupportType_SUPPORT_TYPE_PREMIUM,
				AdvancedAccessWorkflows: true,
				Questionnaire:           false,
				IsStripeManaged:         false,
				CustomTheme:             "",
				RecoveryCodes:           false,
				Plugins:                 false,
				AutomaticUpgrades:       false,
				AccessGraph:             false,
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Identity:               {Enabled: true},
					entitlements.OIDC:                   {Enabled: true},
					entitlements.SAML:                   {Enabled: true},
					entitlements.HSM:                    {Enabled: true},
					entitlements.JoinActiveSessions:     {Enabled: true},
					entitlements.MobileDeviceManagement: {Enabled: true},
					entitlements.ExternalAuditStorage:   {},
					entitlements.App:                    {Enabled: true},
					entitlements.CloudAuditLogRetention: {},
					entitlements.DB:                     {},
					entitlements.Desktop:                {},
					entitlements.FeatureHiding:          {},
					entitlements.K8s:                    {},
					entitlements.Policy:                 {},
					entitlements.UpsellAlert:            {},
					entitlements.UsageReporting:         {},
					entitlements.AccessRequests:         {Enabled: true},
					entitlements.DeviceTrust:            {Enabled: true},
					entitlements.AccessMonitoring:       {Enabled: true},
					entitlements.AccessLists:            {Enabled: true},
					entitlements.OktaSCIM:               {Enabled: true},
					entitlements.OktaUserSync:           {Enabled: true},
					entitlements.SessionLocks:           {Enabled: true},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := GetSelfHostedLicenseFeatures(tc.license())
			require.Equal(t, tc.expected.Entitlements, actual.Entitlements)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetLicenseFeatures_LegacyIdentityFeatures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		license func() types.License
		assert  func(*testing.T, modules.Features)
	}{
		{
			desc: "having Identity (usage) sets unlimited Access Requests, Device Trust, Access Monitoring, AccessLists, Okta SCIM, Okta User Sync & Session Locks",
			license: func() types.License {
				license, err := types.NewLicense("license", types.LicenseSpecV3{
					UsageBasedBilling:                  true,
					SupportsIdentityGovernanceSecurity: true,
				})
				require.NoError(t, err)
				return license
			},
			assert: func(t *testing.T, s modules.Features) {
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessRequests])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.DeviceTrust])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessMonitoring])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessLists])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.OktaSCIM])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.OktaUserSync])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.SessionLocks])
			},
		},
		{
			desc: "having Identity (non-usage) sets unlimited Access Requests, Device Trust, Access Monitoring, Lists, Okta SCIM, Okta User Sync & Session Locks",
			license: func() types.License {
				license, err := types.NewLicense("license", types.LicenseSpecV3{
					SupportsIdentityGovernanceSecurity: true,
				})
				require.NoError(t, err)
				return license
			},
			assert: func(t *testing.T, s modules.Features) {
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessRequests])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.DeviceTrust])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessMonitoring])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessLists])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.OktaSCIM])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.OktaUserSync])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.SessionLocks])
			},
		},
		{
			desc: "non-usage without IGS has unlimited access to access requests & device trust; limited access to Lists, Monitoring, Okta User Sync & Session Locks",
			license: func() types.License {
				license, err := types.NewLicense("license", types.LicenseSpecV3{})
				require.NoError(t, err)
				return license
			},
			assert: func(t *testing.T, s modules.Features) {
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.AccessRequests])
				require.Equal(t, modules.EntitlementInfo{Enabled: true}, s.Entitlements[entitlements.DeviceTrust])
				require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 30}, s.Entitlements[entitlements.AccessMonitoring])
				require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 1}, s.Entitlements[entitlements.AccessLists])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaSCIM])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaUserSync])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.SessionLocks])
			},
		},
		{
			desc: "usage without IGS has limited Access Requests, Device Trust, Access Lists, Access Monitoring & Session Locks",
			license: func() types.License {
				license, err := types.NewLicense("license", types.LicenseSpecV3{
					UsageBasedBilling: true,
				})
				require.NoError(t, err)
				return license
			},
			assert: func(t *testing.T, s modules.Features) {
				require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 5}, s.Entitlements[entitlements.AccessRequests])
				require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 5}, s.Entitlements[entitlements.DeviceTrust])
				require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 30}, s.Entitlements[entitlements.AccessMonitoring])
				require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 1}, s.Entitlements[entitlements.AccessLists])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaSCIM])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaUserSync])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.SessionLocks])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := GetSelfHostedLicenseFeatures(tc.license())
			tc.assert(t, actual)
		})
	}
}

func TestGetLicenseFeatures_Entitlements(t *testing.T) {
	expectedEntitlements := make(map[entitlements.EntitlementKind]modules.EntitlementInfo, len(entitlements.AllEntitlements))
	// specify all entitlements
	for _, e := range entitlements.AllEntitlements {
		expectedEntitlements[e] = modules.EntitlementInfo{
			Enabled: true,
			Limit:   222,
		}
	}
	// disable AccessGraphDemo and App
	expectedEntitlements[entitlements.AccessGraphDemoMode] = modules.EntitlementInfo{}
	expectedEntitlements[entitlements.App] = modules.EntitlementInfo{}

	expected := modules.Features{
		AccessControls:          true,
		AccessGraph:             false,
		AdvancedAccessWorkflows: true,
		AutomaticUpgrades:       false,
		Cloud:                   false,
		CustomTheme:             "",
		IsStripeManaged:         false,
		IsUsageBasedBilling:     false,
		Plugins:                 false,
		ProductType:             0,
		Questionnaire:           false,
		RecoveryCodes:           false,
		SupportType:             2,
		Entitlements:            expectedEntitlements,
	}

	newEntitlements := make(map[string]types.EntitlementInfo, len(entitlements.AllEntitlements))
	// specify all entitlements
	for _, e := range entitlements.AllEntitlements {
		newEntitlements[string(e)] = types.EntitlementInfo{
			Enabled: true,
			Limit:   222,
		}
	}
	// exclude AccessGraphDemo and App
	delete(newEntitlements, string(entitlements.AccessGraphDemoMode))
	delete(newEntitlements, string(entitlements.App))

	license, err := types.NewLicense("license", types.LicenseSpecV3{
		AccountID:                          "",
		AWSProductID:                       "",
		AWSAccountID:                       "",
		SupportsKubernetes:                 false,
		SupportsApplicationAccess:          nil,
		SupportsDatabaseAccess:             false,
		SupportsDesktopAccess:              false,
		ReportsUsage:                       false,
		SalesCenterReporting:               false,
		Cloud:                              false,
		SupportsModeratedSessions:          false,
		SupportsMachineID:                  false,
		SupportsResourceAccessRequests:     false,
		SupportsFeatureHiding:              false,
		Trial:                              false,
		FeatureSource:                      "",
		CustomTheme:                        "",
		SupportsIdentityGovernanceSecurity: false,
		UsageBasedBilling:                  false,
		AnonymizationKey:                   "",
		SupportsPolicy:                     false,
		Entitlements:                       newEntitlements,
	})
	require.NoError(t, err)

	actual := GetSelfHostedLicenseFeatures(license)
	require.Equal(t, expected, actual)
}

func TestEnterpriseModules_SetFeatures(t *testing.T) {
	tt := []struct {
		name             string
		initialConfig    EnterpriseModulesConfig
		updatedFeatures  modules.Features
		updatedLicense   *licensefile.LicenseFile
		expectedFeatures modules.Features
	}{
		{
			name: "overwrites fields except config-based ones",
			initialConfig: EnterpriseModulesConfig{
				HostedPluginsEnabled:     true,
				Cloud:                    true,
				AutomaticUpgradesEnabled: true,
				Features: modules.Features{
					RecoveryCodes:              true,
					Plugins:                    true,
					AccessGraph:                true,
					AccessMonitoringConfigured: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.AccessMonitoring: {Enabled: true},
						entitlements.Policy:           {Enabled: true},
					},
				},
			},
			updatedFeatures: modules.Features{
				RecoveryCodes:              false, // should NOT overwrite
				Plugins:                    false, // should NOT overwrite
				AccessGraph:                false, // should NOT overwrite
				AccessMonitoringConfigured: false, // should NOT overwrite
				AutomaticUpgrades:          false, // should NOT overwrite
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.AccessMonitoring: {Enabled: false}, // should NOT overwrite
					entitlements.Policy:           {Enabled: false}, // should NOT overwrite
				},
				Cloud:       true,   // should overwrite
				CustomTheme: "dark", // should overwrite
			},
			expectedFeatures: modules.Features{
				RecoveryCodes:              true, // stays true
				Plugins:                    true, // stays true
				AccessGraph:                true, // stays true
				AccessMonitoringConfigured: true, // stays true
				AutomaticUpgrades:          true, // stays true
				Cloud:                      true, // updated
				CustomTheme:                "dark",
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.AccessMonitoring: {Enabled: false},
					entitlements.Policy:           {Enabled: false},
				},
			},
		},
		{
			name:          "no config-based features, only input values",
			initialConfig: EnterpriseModulesConfig{},
			updatedFeatures: modules.Features{
				Cloud:       true,
				CustomTheme: "light",
			},
			expectedFeatures: modules.Features{
				RecoveryCodes:              false,
				Plugins:                    false,
				AccessGraph:                false,
				AccessMonitoringConfigured: false,
				Cloud:                      true,
				CustomTheme:                "light",
			},
		},
		{
			name: "updated license",
			initialConfig: EnterpriseModulesConfig{
				HostedPluginsEnabled:     true,
				Cloud:                    true,
				AutomaticUpgradesEnabled: true,
				Features: modules.Features{
					RecoveryCodes:              true,
					Plugins:                    true,
					AccessGraph:                true,
					AccessMonitoringConfigured: true,
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.AccessMonitoring: {Enabled: true},
						entitlements.Policy:           {Enabled: true},
					},
				},
				License: &licensefile.LicenseFile{
					License: &types.LicenseV3{
						Metadata: types.Metadata{
							Expires: func() *time.Time {
								t := time.Now().Add(3 * time.Hour)
								return &t
							}(),
						},
					},
				},
			},
			updatedFeatures: modules.Features{
				RecoveryCodes:              false, // should NOT overwrite
				Plugins:                    false, // should NOT overwrite
				AccessGraph:                false, // should NOT overwrite
				AccessMonitoringConfigured: false, // should NOT overwrite
				AutomaticUpgrades:          false, // should NOT overwrite
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.AccessMonitoring: {Enabled: false}, // should NOT overwrite
					entitlements.Policy:           {Enabled: false}, // should NOT overwrite
				},
				Cloud:       true,   // should overwrite
				CustomTheme: "dark", // should overwrite
			},
			updatedLicense: &licensefile.LicenseFile{
				License: &types.LicenseV3{
					Metadata: types.Metadata{
						Expires: func() *time.Time {
							t := time.Now().Add(30 * time.Hour)
							return &t
						}(),
					},
				},
			},
			expectedFeatures: modules.Features{
				RecoveryCodes:              true, // stays true
				Plugins:                    true, // stays true
				AccessGraph:                true, // stays true
				AccessMonitoringConfigured: true, // stays true
				AutomaticUpgrades:          true, // stays true
				Cloud:                      true, // updated
				CustomTheme:                "dark",
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.AccessMonitoring: {Enabled: false},
					entitlements.Policy:           {Enabled: false},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			p := NewEnterpriseModules(tc.initialConfig)

			if tc.updatedLicense != nil {
				p.UpdateModules(tc.updatedLicense, tc.updatedFeatures)
			} else {
				p.SetFeatures(tc.updatedFeatures)
			}

			require.Equal(t, tc.expectedFeatures, p.Features())
			if tc.updatedLicense != nil {
				require.Equal(t, tc.updatedLicense.License.Expiry(), p.LicenseExpiry())
			} else {
				require.True(t, p.LicenseExpiry().IsZero())
			}
		})
	}
}
