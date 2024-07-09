package feature

import (
	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

// GetCloudEntitlements takes a cloud entitlement set and returns a modules Entitlement set
func GetCloudEntitlements(cloudEntitlements map[string]*cloudapi.EntitlementInfo) map[entitlements.EntitlementKind]modules.EntitlementInfo {
	all := entitlements.AllEntitlements
	result := make(map[entitlements.EntitlementKind]modules.EntitlementInfo, len(all))

	for _, e := range all {
		al, ok := cloudEntitlements[string(e)]
		if !ok {
			result[e] = modules.EntitlementInfo{}
			continue
		}

		result[e] = modules.EntitlementInfo{
			Enabled: al.Enabled,
			Limit:   al.Limit,
		}
	}

	return result
}

// GetLicenseEntitlements takes a license entitlement set and returns a modules Entitlement set
func GetLicenseEntitlements(licenseEntitlements map[string]types.EntitlementInfo) map[entitlements.EntitlementKind]modules.EntitlementInfo {
	all := entitlements.AllEntitlements
	result := make(map[entitlements.EntitlementKind]modules.EntitlementInfo, len(all))

	for _, e := range all {
		al, ok := licenseEntitlements[string(e)]
		if !ok {
			result[e] = modules.EntitlementInfo{}
			continue
		}

		result[e] = modules.EntitlementInfo{
			Enabled: al.Enabled.Value(),
			Limit:   al.Limit,
		}
	}

	return result
}
