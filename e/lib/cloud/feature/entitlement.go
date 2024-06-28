package feature

import (
	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
)

// GetCloudEntitlement takes a cloud entitlement set and a feature, and returns an Entitlement for that feature populated with
// information from the entitlement set
func GetCloudEntitlement(f map[string]*cloudapi.EntitlementInfo, e entitlements.EntitlementKind) modules.EntitlementInfo {
	al, ok := f[string(e)]
	if !ok {
		return modules.EntitlementInfo{}
	}

	return modules.EntitlementInfo{
		Enabled: al.Enabled,
		Limit:   al.Limit,
	}
}

// GetLicenseEntitlement takes a license entitlement set and a feature, and returns an Entitlement for that feature populated with
// information from the entitlement set
func GetLicenseEntitlement(f map[string]types.EntitlementInfo, e entitlements.EntitlementKind) modules.EntitlementInfo {
	al, ok := f[string(e)]
	if !ok {
		return modules.EntitlementInfo{}
	}

	return modules.EntitlementInfo{
		Enabled: al.Enabled.Value(),
		Limit:   al.Limit,
	}
}
