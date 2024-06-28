package feature

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGetCloudEntitlement(t *testing.T) {
	f := map[string]*cloudapi.EntitlementInfo{
		string(entitlements.AccessLists):      {Enabled: true},
		string(entitlements.AccessMonitoring): {Enabled: true, Limit: 1},
		string(entitlements.HSM):              {Enabled: false},
		string(entitlements.DeviceTrust):      {Enabled: true, Limit: 3},
	}

	accessListEntitlement := GetCloudEntitlement(f, entitlements.AccessLists)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: true,
		Limit:   0,
	}, accessListEntitlement)

	accessMonitoringEntitlement := GetCloudEntitlement(f, entitlements.AccessMonitoring)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: true,
		Limit:   1,
	}, accessMonitoringEntitlement)

	HSMEntitlement := GetCloudEntitlement(f, entitlements.HSM)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: false,
		Limit:   0,
	}, HSMEntitlement)

	DeviceTrustEntitlement := GetCloudEntitlement(f, entitlements.DeviceTrust)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: true,
		Limit:   3,
	}, DeviceTrustEntitlement)

	unsetEntitlement := GetCloudEntitlement(f, entitlements.MobileDeviceManagement)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: false,
		Limit:   0,
	}, unsetEntitlement)
}

func TestGetLicenseEntitlement(t *testing.T) {
	f := map[string]types.EntitlementInfo{
		string(entitlements.AccessLists):      {Enabled: true},
		string(entitlements.AccessMonitoring): {Enabled: true, Limit: 1},
		string(entitlements.HSM):              {Enabled: false},
		string(entitlements.DeviceTrust):      {Enabled: true, Limit: 3},
	}

	accessListEntitlement := GetLicenseEntitlement(f, entitlements.AccessLists)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: true,
		Limit:   0,
	}, accessListEntitlement)

	accessMonitoringEntitlement := GetLicenseEntitlement(f, entitlements.AccessMonitoring)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: true,
		Limit:   1,
	}, accessMonitoringEntitlement)

	HSMEntitlement := GetLicenseEntitlement(f, entitlements.HSM)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: false,
		Limit:   0,
	}, HSMEntitlement)

	DeviceTrustEntitlement := GetLicenseEntitlement(f, entitlements.DeviceTrust)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: true,
		Limit:   3,
	}, DeviceTrustEntitlement)

	unsetEntitlement := GetLicenseEntitlement(f, entitlements.MobileDeviceManagement)
	require.Equal(t, modules.EntitlementInfo{
		Enabled: false,
		Limit:   0,
	}, unsetEntitlement)
}
