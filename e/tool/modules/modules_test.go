package modules

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

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
					entitlements.OktaSCIM:               {},
					entitlements.OktaUserSync:           {},
					entitlements.Policy:                 {},
					entitlements.SAML:                   {Enabled: true},
					entitlements.SessionLocks:           {},
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
					entitlements.OktaSCIM:               {},
					entitlements.OktaUserSync:           {},
					entitlements.SessionLocks:           {},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := getSelfHostedLicenseFeatures(tc.license())
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
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaSCIM])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaUserSync])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.SessionLocks])
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
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaSCIM])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.OktaUserSync])
				require.Equal(t, modules.EntitlementInfo{Enabled: false}, s.Entitlements[entitlements.SessionLocks])
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
			actual := getSelfHostedLicenseFeatures(tc.license())
			tc.assert(t, actual)
		})
	}
}

func TestGetLicenseFeatures_Entitlements(t *testing.T) {
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
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.AccessLists:            {Enabled: true, Limit: 222},
			entitlements.AccessMonitoring:       {Enabled: true, Limit: 222},
			entitlements.AccessRequests:         {Enabled: true, Limit: 222},
			entitlements.App:                    {Enabled: true, Limit: 222},
			entitlements.CloudAuditLogRetention: {Enabled: true, Limit: 222},
			entitlements.DB:                     {Enabled: true, Limit: 222},
			entitlements.Desktop:                {Enabled: true, Limit: 222},
			entitlements.DeviceTrust:            {Enabled: true, Limit: 222},
			entitlements.ExternalAuditStorage:   {Enabled: true, Limit: 222},
			entitlements.FeatureHiding:          {Enabled: true, Limit: 222},
			entitlements.HSM:                    {Enabled: true, Limit: 222},
			entitlements.Identity:               {Enabled: true, Limit: 222},
			entitlements.JoinActiveSessions:     {Enabled: true, Limit: 222},
			entitlements.K8s:                    {Enabled: true, Limit: 222},
			entitlements.MobileDeviceManagement: {Enabled: true, Limit: 222},
			entitlements.OIDC:                   {Enabled: true, Limit: 222},
			entitlements.OktaSCIM:               {Enabled: true, Limit: 222},
			entitlements.OktaUserSync:           {Enabled: true, Limit: 222},
			entitlements.Policy:                 {Enabled: true, Limit: 222},
			entitlements.SAML:                   {Enabled: true, Limit: 222},
			entitlements.SessionLocks:           {Enabled: true, Limit: 222},
			entitlements.UpsellAlert:            {Enabled: true, Limit: 222},
			entitlements.UsageReporting:         {Enabled: true, Limit: 222},
		},
	}

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
		Entitlements: map[string]types.EntitlementInfo{
			"AccessLists":            {Enabled: true, Limit: 222},
			"AccessMonitoring":       {Enabled: true, Limit: 222},
			"AccessRequests":         {Enabled: true, Limit: 222},
			"App":                    {Enabled: true, Limit: 222},
			"CloudAuditLogRetention": {Enabled: true, Limit: 222},
			"DB":                     {Enabled: true, Limit: 222},
			"Desktop":                {Enabled: true, Limit: 222},
			"DeviceTrust":            {Enabled: true, Limit: 222},
			"ExternalAuditStorage":   {Enabled: true, Limit: 222},
			"FeatureHiding":          {Enabled: true, Limit: 222},
			"HSM":                    {Enabled: true, Limit: 222},
			"Identity":               {Enabled: true, Limit: 222},
			"JoinActiveSessions":     {Enabled: true, Limit: 222},
			"K8s":                    {Enabled: true, Limit: 222},
			"MobileDeviceManagement": {Enabled: true, Limit: 222},
			"OIDC":                   {Enabled: true, Limit: 222},
			"OktaSCIM":               {Enabled: true, Limit: 222},
			"OktaUserSync":           {Enabled: true, Limit: 222},
			"Policy":                 {Enabled: true, Limit: 222},
			"SAML":                   {Enabled: true, Limit: 222},
			"SessionLocks":           {Enabled: true, Limit: 222},
			"UpsellAlert":            {Enabled: true, Limit: 222},
			"UsageReporting":         {Enabled: true, Limit: 222},
		},
	})
	require.NoError(t, err)

	actual := getSelfHostedLicenseFeatures(license)
	require.Equal(t, expected, actual)
}
