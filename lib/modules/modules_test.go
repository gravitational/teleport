/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package modules_test

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestOSSModules(t *testing.T) {
	require.False(t, modules.GetModules().IsBoringBinary())
	require.Equal(t, modules.BuildOSS, modules.GetModules().BuildType())
}

func TestValidateAuthPreferenceOnCloud(t *testing.T) {
	ctx := context.Background()
	testServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	authPref, err := testServer.AuthServer.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)

	authPref.SetSecondFactor(constants.SecondFactorOff)
	_, err = testServer.AuthServer.UpdateAuthPreference(ctx, authPref)
	require.EqualError(t, err, modules.ErrCannotDisableSecondFactor.Error())
}

func TestValidateSessionRecordingConfigOnCloud(t *testing.T) {
	ctx := context.Background()

	testServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	recConfig := types.DefaultSessionRecordingConfig()
	_, err = testServer.AuthServer.UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	recConfig.SetMode(types.RecordAtProxy)
	_, err = testServer.AuthServer.UpsertSessionRecordingConfig(ctx, recConfig)
	require.EqualError(t, err, "cannot set proxy recording mode on Cloud")
}

func TestFeatures_ToProto(t *testing.T) {
	expected := &proto.Features{
		CustomTheme:                "dark",
		ProductType:                1,
		SupportType:                1,
		AccessControls:             true,
		AccessGraph:                true,
		AdvancedAccessWorkflows:    true,
		AutomaticUpgrades:          true,
		Cloud:                      true,
		IsStripeManaged:            true,
		IsUsageBased:               true,
		Plugins:                    true,
		Questionnaire:              true,
		RecoveryCodes:              true,
		AccessMonitoringConfigured: false,
		CloudAnonymizationKey:      []byte("001"),
		Entitlements: map[string]*proto.EntitlementInfo{
			string(entitlements.AccessLists):            {Enabled: true, Limit: 111},
			string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 2113},
			string(entitlements.AccessRequests):         {Enabled: true, Limit: 39},
			string(entitlements.App):                    {Enabled: false},
			string(entitlements.CloudAuditLogRetention): {Enabled: true},
			string(entitlements.DB):                     {Enabled: true},
			string(entitlements.Desktop):                {Enabled: true},
			string(entitlements.DeviceTrust):            {Enabled: true, Limit: 103},
			string(entitlements.ExternalAuditStorage):   {Enabled: true},
			string(entitlements.FeatureHiding):          {Enabled: true},
			string(entitlements.HSM):                    {Enabled: true},
			string(entitlements.Identity):               {Enabled: true},
			string(entitlements.JoinActiveSessions):     {Enabled: true},
			string(entitlements.K8s):                    {Enabled: true},
			string(entitlements.MobileDeviceManagement): {Enabled: true},
			string(entitlements.OIDC):                   {Enabled: true},
			string(entitlements.OktaSCIM):               {Enabled: true},
			string(entitlements.OktaUserSync):           {Enabled: true},
			string(entitlements.Policy):                 {Enabled: true},
			string(entitlements.SAML):                   {Enabled: true},
			string(entitlements.SessionLocks):           {Enabled: true},
			string(entitlements.UpsellAlert):            {Enabled: true},
			string(entitlements.UsageReporting):         {Enabled: true},
			string(entitlements.LicenseAutoUpdate):      {Enabled: true},
		},
		//	 Legacy Fields; remove in v18
		Kubernetes:             true,
		App:                    false,
		DB:                     true,
		OIDC:                   true,
		SAML:                   true,
		HSM:                    true,
		Desktop:                true,
		FeatureHiding:          true,
		IdentityGovernance:     true,
		ExternalAuditStorage:   true,
		JoinActiveSessions:     true,
		MobileDeviceManagement: true,
		DeviceTrust: &proto.DeviceTrustFeature{
			Enabled:           true,
			DevicesUsageLimit: 103,
		},
		AccessRequests: &proto.AccessRequestsFeature{
			MonthlyRequestLimit: 39,
		},
		AccessMonitoring: &proto.AccessMonitoringFeature{
			Enabled:             false, // set to value of AccessMonitoringConfigured
			MaxReportRangeLimit: 2113,
		},
		AccessList: &proto.AccessListFeature{
			CreateLimit: 111,
		},
		Policy: &proto.PolicyFeature{
			Enabled: true,
		},
	}

	f := modules.Features{
		CustomTheme:                "dark",
		ProductType:                1,
		SupportType:                1,
		AccessControls:             true,
		AccessGraph:                true,
		AdvancedAccessWorkflows:    true,
		Assist:                     true,
		AutomaticUpgrades:          true,
		Cloud:                      true,
		IsStripeManaged:            true,
		IsUsageBasedBilling:        true,
		Plugins:                    true,
		Questionnaire:              true,
		RecoveryCodes:              true,
		AccessMonitoringConfigured: false,
		CloudAnonymizationKey:      []byte("001"),
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.AccessLists:            {Enabled: true, Limit: 111},
			entitlements.AccessMonitoring:       {Enabled: true, Limit: 2113},
			entitlements.AccessRequests:         {Enabled: true, Limit: 39},
			entitlements.App:                    {Enabled: false, Limit: 0},
			entitlements.CloudAuditLogRetention: {Enabled: true, Limit: 0},
			entitlements.DB:                     {Enabled: true, Limit: 0},
			entitlements.Desktop:                {Enabled: true, Limit: 0},
			entitlements.DeviceTrust:            {Enabled: true, Limit: 103},
			entitlements.ExternalAuditStorage:   {Enabled: true, Limit: 0},
			entitlements.FeatureHiding:          {Enabled: true, Limit: 0},
			entitlements.HSM:                    {Enabled: true, Limit: 0},
			entitlements.Identity:               {Enabled: true, Limit: 0},
			entitlements.JoinActiveSessions:     {Enabled: true, Limit: 0},
			entitlements.K8s:                    {Enabled: true, Limit: 0},
			entitlements.MobileDeviceManagement: {Enabled: true, Limit: 0},
			entitlements.OIDC:                   {Enabled: true, Limit: 0},
			entitlements.OktaSCIM:               {Enabled: true, Limit: 0},
			entitlements.OktaUserSync:           {Enabled: true, Limit: 0},
			entitlements.Policy:                 {Enabled: true, Limit: 0},
			entitlements.SAML:                   {Enabled: true, Limit: 0},
			entitlements.SessionLocks:           {Enabled: true, Limit: 0},
			entitlements.UpsellAlert:            {Enabled: true, Limit: 0},
			entitlements.UsageReporting:         {Enabled: true, Limit: 0},
			entitlements.LicenseAutoUpdate:      {Enabled: true, Limit: 0},
		},
	}

	actual := f.ToProto()
	require.Equal(t, expected, actual)
}

func TestFeatures_GetEntitlement(t *testing.T) {
	f := modules.Features{
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.AccessLists: {Enabled: true, Limit: 111},
			entitlements.K8s:         {Enabled: false},
			entitlements.SAML:        {},
		},
	}

	actual := f.GetEntitlement(entitlements.AccessLists)
	require.Equal(t, modules.EntitlementInfo{Enabled: true, Limit: 111}, actual)

	actual = f.GetEntitlement(entitlements.K8s)
	require.Equal(t, modules.EntitlementInfo{Enabled: false}, actual)

	actual = f.GetEntitlement(entitlements.SAML)
	require.Equal(t, modules.EntitlementInfo{}, actual)

	actual = f.GetEntitlement(entitlements.UsageReporting)
	require.Equal(t, modules.EntitlementInfo{}, actual)
}

func TestEntitlementInfo_UnderLimit(t *testing.T) {
	testCases := []struct {
		name   string
		count  int
		uut    modules.EntitlementInfo
		assert require.BoolAssertionFunc
	}{
		{
			name:   "disabled is always out of limit",
			count:  10,
			uut:    modules.EntitlementInfo{Enabled: false, Limit: 10000},
			assert: require.False,
		},
		{
			name:   "within limits returns true",
			count:  10,
			uut:    modules.EntitlementInfo{Enabled: true, Limit: 10000},
			assert: require.True,
		},
		{
			name:   "at limits returns false",
			count:  10000,
			uut:    modules.EntitlementInfo{Enabled: true, Limit: 10000},
			assert: require.False,
		},
		{
			name:   "above limits returns false",
			count:  10001,
			uut:    modules.EntitlementInfo{Enabled: true, Limit: 10000},
			assert: require.False,
		},
		{
			name:   "zero limit implies max",
			count:  math.MaxInt64,
			uut:    modules.EntitlementInfo{Enabled: true, Limit: 0},
			assert: require.True,
		},
		{
			name:   "handles above MaxInt32",
			count:  math.MaxInt32 + 1,
			uut:    modules.EntitlementInfo{Enabled: true, Limit: math.MaxInt32},
			assert: require.False,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, tc.uut.UnderLimit(tc.count))
		})
	}
}
