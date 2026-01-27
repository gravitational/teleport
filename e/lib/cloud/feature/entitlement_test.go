package feature

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGetCloudEntitlements(t *testing.T) {
	expected := getPopulatedEntitlements(map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.AccessLists:      {Enabled: true},
		entitlements.AccessMonitoring: {Enabled: true, Limit: 1},
		entitlements.DeviceTrust:      {Enabled: true, Limit: 3},
	})

	e := map[string]*cloudapi.EntitlementInfo{
		string(entitlements.AccessLists):      {Enabled: true},
		string(entitlements.AccessMonitoring): {Enabled: true, Limit: 1},
		string(entitlements.HSM):              {Enabled: false},
		string(entitlements.DeviceTrust):      {Enabled: true, Limit: 3},
	}

	actual := GetCloudEntitlements(e)
	require.Equal(t, expected, actual)
}

func TestGetLicenseEntitlements(t *testing.T) {
	expected := getPopulatedEntitlements(map[entitlements.EntitlementKind]modules.EntitlementInfo{
		entitlements.AccessLists:      {Enabled: true},
		entitlements.AccessMonitoring: {Enabled: true, Limit: 11},
		entitlements.DeviceTrust:      {Enabled: true, Limit: 33},
	})

	e := map[string]types.EntitlementInfo{
		string(entitlements.AccessLists):      {Enabled: true},
		string(entitlements.AccessMonitoring): {Enabled: true, Limit: 11},
		string(entitlements.HSM):              {Enabled: false},
		string(entitlements.DeviceTrust):      {Enabled: true, Limit: 33},
	}

	actual := GetLicenseEntitlements(e)
	require.Equal(t, expected, actual)
}

// getPopulatedEntitlements returns a map with all [entitlements.EntitlementKind]s specified
//
// Configured entitlements may be provided, otherwise a zero value is used. Typically used for tests.
func getPopulatedEntitlements(configured map[entitlements.EntitlementKind]modules.EntitlementInfo) map[entitlements.EntitlementKind]modules.EntitlementInfo {
	populated := make(map[entitlements.EntitlementKind]modules.EntitlementInfo, len(entitlements.AllEntitlements))

	for _, e := range entitlements.AllEntitlements {
		c, ok := configured[e]
		if !ok {
			populated[e] = modules.EntitlementInfo{}

			continue
		}

		populated[e] = c
	}

	return populated
}
