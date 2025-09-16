// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package entitlements

import "github.com/gravitational/teleport/api/client/proto"

type EntitlementKind string

// The EntitlementKind list should be 1:1 with the Features & FeatureStrings in salescenter/product/product.go,
// except CustomTheme which is dropped. CustomTheme entitlement only toggles the ability to "set" a theme;
// the value of that theme, if set, is stored and accessed outside of entitlements.
//
// All EntitlementKinds added here should also be added to AllEntitlements below and defaultEntitlements in
// web/packages/teleport/src/entitlement.ts.
const (
	AccessLists            EntitlementKind = "AccessLists"
	AccessMonitoring       EntitlementKind = "AccessMonitoring"
	AccessRequests         EntitlementKind = "AccessRequests"
	App                    EntitlementKind = "App"
	CloudAuditLogRetention EntitlementKind = "CloudAuditLogRetention"
	DB                     EntitlementKind = "DB"
	Desktop                EntitlementKind = "Desktop"
	DeviceTrust            EntitlementKind = "DeviceTrust"
	ExternalAuditStorage   EntitlementKind = "ExternalAuditStorage"
	FeatureHiding          EntitlementKind = "FeatureHiding"
	HSM                    EntitlementKind = "HSM"
	Identity               EntitlementKind = "Identity"
	JoinActiveSessions     EntitlementKind = "JoinActiveSessions"
	K8s                    EntitlementKind = "K8s"
	MobileDeviceManagement EntitlementKind = "MobileDeviceManagement"
	OIDC                   EntitlementKind = "OIDC"
	OktaSCIM               EntitlementKind = "OktaSCIM"
	OktaUserSync           EntitlementKind = "OktaUserSync"
	Policy                 EntitlementKind = "Policy"
	SAML                   EntitlementKind = "SAML"
	SessionLocks           EntitlementKind = "SessionLocks"
	UpsellAlert            EntitlementKind = "UpsellAlert"
	UsageReporting         EntitlementKind = "UsageReporting"
	LicenseAutoUpdate      EntitlementKind = "LicenseAutoUpdate"
)

// AllEntitlements returns all Entitlements; should be 1:1 with the const declared above.
var AllEntitlements = []EntitlementKind{
	AccessLists, AccessMonitoring, AccessRequests, App, CloudAuditLogRetention, DB, Desktop, DeviceTrust,
	ExternalAuditStorage, FeatureHiding, HSM, Identity, JoinActiveSessions, K8s, MobileDeviceManagement, OIDC, OktaSCIM,
	OktaUserSync, Policy, SAML, SessionLocks, UpsellAlert, UsageReporting, LicenseAutoUpdate,
}

// BackfillFeatures ensures entitlements are backwards compatible.
// If Entitlements are present, there are no changes.
// If Entitlements are not present, it sets the entitlements based on legacy field values.
// TODO(michellescripts) DELETE IN 18.0.0
func BackfillFeatures(features *proto.Features) {
	if len(features.Entitlements) > 0 {
		return
	}

	features.Entitlements = getBaseEntitlements(features.GetEntitlements())

	// Entitlements: All records are {enabled: false}; update to equal legacy feature value
	features.Entitlements[string(ExternalAuditStorage)] = &proto.EntitlementInfo{Enabled: features.GetExternalAuditStorage()}
	features.Entitlements[string(FeatureHiding)] = &proto.EntitlementInfo{Enabled: features.GetFeatureHiding()}
	features.Entitlements[string(Identity)] = &proto.EntitlementInfo{Enabled: features.GetIdentityGovernance()}
	features.Entitlements[string(JoinActiveSessions)] = &proto.EntitlementInfo{Enabled: features.GetJoinActiveSessions()}
	features.Entitlements[string(MobileDeviceManagement)] = &proto.EntitlementInfo{Enabled: features.GetMobileDeviceManagement()}
	features.Entitlements[string(OIDC)] = &proto.EntitlementInfo{Enabled: features.GetOIDC()}
	features.Entitlements[string(Policy)] = &proto.EntitlementInfo{Enabled: features.GetPolicy().GetEnabled()}
	features.Entitlements[string(SAML)] = &proto.EntitlementInfo{Enabled: features.GetSAML()}
	features.Entitlements[string(K8s)] = &proto.EntitlementInfo{Enabled: features.GetKubernetes()}
	features.Entitlements[string(App)] = &proto.EntitlementInfo{Enabled: features.GetApp()}
	features.Entitlements[string(DB)] = &proto.EntitlementInfo{Enabled: features.GetDB()}
	features.Entitlements[string(Desktop)] = &proto.EntitlementInfo{Enabled: features.GetDesktop()}
	features.Entitlements[string(HSM)] = &proto.EntitlementInfo{Enabled: features.GetHSM()}

	// set default Identity fields to legacy feature value
	features.Entitlements[string(AccessLists)] = &proto.EntitlementInfo{Enabled: true, Limit: features.GetAccessList().GetCreateLimit()}
	features.Entitlements[string(AccessMonitoring)] = &proto.EntitlementInfo{Enabled: features.GetAccessMonitoring().GetEnabled(), Limit: features.GetAccessMonitoring().GetMaxReportRangeLimit()}
	features.Entitlements[string(AccessRequests)] = &proto.EntitlementInfo{Enabled: features.GetAccessRequests().MonthlyRequestLimit > 0, Limit: features.GetAccessRequests().GetMonthlyRequestLimit()}
	features.Entitlements[string(DeviceTrust)] = &proto.EntitlementInfo{Enabled: features.GetDeviceTrust().GetEnabled(), Limit: features.GetDeviceTrust().GetDevicesUsageLimit()}
	// override Identity Package features if Identity is enabled: set true and clear limit
	if features.GetIdentityGovernance() {
		features.Entitlements[string(AccessLists)] = &proto.EntitlementInfo{Enabled: true}
		features.Entitlements[string(AccessMonitoring)] = &proto.EntitlementInfo{Enabled: true}
		features.Entitlements[string(AccessRequests)] = &proto.EntitlementInfo{Enabled: true}
		features.Entitlements[string(DeviceTrust)] = &proto.EntitlementInfo{Enabled: true}
		features.Entitlements[string(OktaSCIM)] = &proto.EntitlementInfo{Enabled: true}
		features.Entitlements[string(OktaUserSync)] = &proto.EntitlementInfo{Enabled: true}
		features.Entitlements[string(SessionLocks)] = &proto.EntitlementInfo{Enabled: true}
	}
}

// getBaseEntitlements takes a cloud entitlement set and returns a modules Entitlement set
func getBaseEntitlements(protoEntitlements map[string]*proto.EntitlementInfo) map[string]*proto.EntitlementInfo {
	all := AllEntitlements
	result := make(map[string]*proto.EntitlementInfo, len(all))

	for _, e := range all {
		al, ok := protoEntitlements[string(e)]
		if !ok {
			result[string(e)] = &proto.EntitlementInfo{}
			continue
		}

		result[string(e)] = &proto.EntitlementInfo{
			Enabled: al.Enabled,
			Limit:   al.Limit,
		}
	}

	return result
}
