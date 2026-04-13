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

type EntitlementKind string

// The EntitlementKind list should be 1:1 with the Features & FeatureStrings in cloud/cloud/product/product.go,
// except CustomTheme which is dropped. CustomTheme entitlement only toggles the ability to "set" a theme;
// the value of that theme, if set, is stored and accessed outside of entitlements.
//
// All EntitlementKinds added here should also be added to AllEntitlements below and defaultEntitlements in
// web/packages/teleport/src/entitlement.ts.
const (
	AccessGraph                EntitlementKind = "AccessGraph"
	AccessGraphDemoMode        EntitlementKind = "AccessGraphDemoMode"
	AccessLists                EntitlementKind = "AccessLists"
	AccessMonitoring           EntitlementKind = "AccessMonitoring"
	AccessRequests             EntitlementKind = "AccessRequests"
	App                        EntitlementKind = "App"
	Beams                      EntitlementKind = "Beams"
	ClientIPRestrictions       EntitlementKind = "ClientIPRestrictions"
	CloudAuditLogRetention     EntitlementKind = "CloudAuditLogRetention"
	DB                         EntitlementKind = "DB"
	Desktop                    EntitlementKind = "Desktop"
	DeviceTrust                EntitlementKind = "DeviceTrust"
	ExternalAuditStorage       EntitlementKind = "ExternalAuditStorage"
	FeatureHiding              EntitlementKind = "FeatureHiding"
	HSM                        EntitlementKind = "HSM"
	Identity                   EntitlementKind = "Identity"
	JoinActiveSessions         EntitlementKind = "JoinActiveSessions"
	K8s                        EntitlementKind = "K8s"
	LicenseAutoUpdate          EntitlementKind = "LicenseAutoUpdate"
	MobileDeviceManagement     EntitlementKind = "MobileDeviceManagement"
	OIDC                       EntitlementKind = "OIDC"
	OktaSCIM                   EntitlementKind = "OktaSCIM"
	OktaUserSync               EntitlementKind = "OktaUserSync"
	Policy                     EntitlementKind = "Policy"
	SAML                       EntitlementKind = "SAML"
	SessionLocks               EntitlementKind = "SessionLocks"
	UnrestrictedManagedUpdates EntitlementKind = "UnrestrictedManagedUpdates"
	UpsellAlert                EntitlementKind = "UpsellAlert"
	UsageReporting             EntitlementKind = "UsageReporting"
	WorkloadClusters           EntitlementKind = "WorkloadClusters"
)

// AllEntitlements returns all Entitlements; should be 1:1 with the const declared above.
var AllEntitlements = []EntitlementKind{
	AccessGraph, AccessGraphDemoMode, AccessLists, AccessMonitoring, AccessRequests, App, Beams,
	ClientIPRestrictions, CloudAuditLogRetention, DB, Desktop, DeviceTrust,
	ExternalAuditStorage, FeatureHiding, HSM, Identity, JoinActiveSessions, K8s, LicenseAutoUpdate,
	MobileDeviceManagement, OIDC, OktaSCIM, OktaUserSync, Policy, SAML, SessionLocks,
	UnrestrictedManagedUpdates, UpsellAlert, UsageReporting, WorkloadClusters,
}
