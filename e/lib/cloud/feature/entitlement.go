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

	applyLegacyPolicyEntitlementsFallback(
		result,
		cloudEntitlements,
	)

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

	applyLegacyPolicyEntitlementsFallback(
		result,
		licenseEntitlements,
	)

	return result
}

// applyLegacyPolicyEntitlementsFallback infers AccessGraph, SessionSummaries,
// and ActivityCenter from Policy for payloads predating their split: if Policy
// is enabled and none appear explicitly, all three are enabled. If any is
// present, the fallback is skipped and explicit values take precedence.
func applyLegacyPolicyEntitlementsFallback[T any](
	entitlementInfo map[entitlements.EntitlementKind]modules.EntitlementInfo,
	incomingEntitlements map[string]T,
) {
	newAccessGraphEntitlements := []entitlements.EntitlementKind{
		entitlements.AccessGraph,
		entitlements.ActivityCenter,
		entitlements.SessionSummaries,
	}

	for _, e := range newAccessGraphEntitlements {
		if hasEntitlement(incomingEntitlements, e) {
			return
		}
	}

	if !entitlementInfo[entitlements.Policy].Enabled {
		return
	}

	entitlementInfo[entitlements.AccessGraph] = modules.EntitlementInfo{Enabled: true}
	entitlementInfo[entitlements.SessionSummaries] = modules.EntitlementInfo{Enabled: true}
	entitlementInfo[entitlements.ActivityCenter] = modules.EntitlementInfo{Enabled: true}
}

func hasEntitlement[T any](entitlements map[string]T, entitlement entitlements.EntitlementKind) bool {
	_, ok := entitlements[string(entitlement)]
	return ok
}
