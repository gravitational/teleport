// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package main

import "google.golang.org/protobuf/reflect/protoreflect"

func newDefaultConfig() *Config {
	return &Config{
		// Existing RPCs that do not follow RFD-153 naming:
		overwrites: map[protoreflect.FullName]fieldNames{
			"teleport.accesslist.v1.AccessListService.ListAccessLists": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.accesslist.v1.AccessListService.ListAccessListReviews": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.accesslist.v1.AccessListService.ListAllAccessListReviews": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.auditlog.v1.AuditLogService.GetUnstructuredEvents": {
				size:  "limit",
				token: "start_key",
				next:  "last_key",
			},
			"teleport.autoupdate.v1.AutoUpdateService.ListAutoUpdateAgentReports": {
				token: "next_token",
				next:  "next_key",
			},
			"teleport.discoveryconfig.v1.DiscoveryConfigService.ListDiscoveryConfigs": {
				token: "next_token",
				next:  "next_key",
			},

			"teleport.integration.v1.AWSOIDCService.ListDatabases": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.integration.v1.AWSOIDCService.ListSecurityGroups": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.integration.v1.AWSOIDCService.ListSubnets": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.integration.v1.AWSOIDCService.ListVPCs": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.integration.v1.AWSOIDCService.ListDeployedDatabaseServices": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.integration.v1.AWSOIDCService.ListEKSClusters": {
				token: "next_token",
				next:  "next_token",
			},
			"teleport.integration.v1.AWSRolesAnywhereService.ListRolesAnywhereProfiles": {
				size:  "page_size",
				token: "next_page_token",
			},
			"teleport.integration.v1.IntegrationService.ListIntegrations": {
				size:  "limit",
				token: "next_key",
				next:  "next_key",
			},
			"teleport.kube.v1.KubeService.ListKubernetesResources": {
				size:  "limit",
				token: "start_key",
				next:  "next_key",
			},
			"teleport.plugins.v1.PluginService.ListPlugins": {
				token: "start_key",
				next:  "next_key",
			},
			"teleport.scim.v1.SCIMService.ListSCIMResources": {
				// This rpc uses a different scheme for pagination (index based).
				token: "page",
				size:  "page",
				next:  "start_index",
			},
			"teleport.scopes.joining.v1.ScopedJoiningService.ListScopedTokens": {
				token: "cursor",
				size:  "limit",
				next:  "cursor",
			},
			"teleport.lib.teleterm.v1.TerminalService.ListKubernetesResources": {
				size:  "limit",
				token: "next_key",
				next:  "next_key",
			},
			"teleport.lib.teleterm.v1.TerminalService.ListDatabaseServers": {
				size:  "params", // params encodes the limit
				token: "params", // params encodes the token
				next:  "next_key",
			},
			"teleport.lib.teleterm.v1.TerminalService.ListUnifiedResources": {
				size:  "limit",
				token: "start_key",
				next:  "next_key",
			},
			"proto.AuthService.GetEvents": {
				size:  "Limit",
				token: "StartKey",
				next:  "LastKey",
			},
			"proto.AuthService.ListApps": {
				size:  "limit",
				token: "start_key",
				next:  "next_key",
			},
			"proto.AuthService.GetSessionEvents": {
				size:  "Limit",
				token: "StartKey",
				next:  "LastKey",
			},
			"proto.AuthService.ListAccessRequests": {
				size:  "Limit",
				token: "StartKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListProvisionTokens": {
				size:  "Limit",
				token: "StartKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListResources": {
				size:  "Limit",
				token: "StartKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListRoles": {
				size:  "Limit",
				token: "StartKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListSAMLIdPServiceProviders": {
				size:  "Limit",
				token: "NextKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListUnifiedResources": {
				size:  "Limit",
				token: "StartKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListUserGroups": {
				size:  "Limit",
				token: "NextKey",
				next:  "NextKey",
			},
			"proto.AuthService.ListWindowsDesktops": {
				size:  "limit",
				token: "start_key",
				next:  "next_key",
			},

			// Testing only:
			"test.foo.bar.v1.config.FooService.SearchFoos": {
				size:  "max",
				token: "token",
				next:  "next",
			},
		},
		skips: map[protoreflect.FullName]struct{}{
			// TODO(okraport): Add paginated RPC alternative and deprecate:
			"proto.AuthService.GetInventoryStatus":                                {},
			"proto.AuthService.GetPluginData":                                     {},
			"teleport.accesslist.v1.AccessListService.GetAccessListOwners":        {},
			"teleport.accesslist.v1.AccessListService.GetAccessListsToReview":     {},
			"teleport.accesslist.v1.AccessListService.GetSuggestedAccessLists":    {},
			"teleport.okta.v1.OktaService.GetApps":                                {},
			"teleport.okta.v1.OktaService.GetGroups":                              {},
			"teleport.userloginstate.v1.UserLoginStateService.GetUserLoginStates": {},

			// TerminalService
			"teleport.lib.teleterm.v1.TerminalService.GetAccessRequests":       {},
			"teleport.lib.teleterm.v1.TerminalService.GetRequestableRoles":     {},
			"teleport.lib.teleterm.v1.TerminalService.GetSuggestedAccessLists": {},
			"teleport.lib.teleterm.v1.TerminalService.ListDatabaseUsers":       {},
			"teleport.lib.teleterm.v1.TerminalService.ListGateways":            {},
			"teleport.lib.teleterm.v1.TerminalService.ListKubernetesResources": {},
			"teleport.lib.teleterm.v1.TerminalService.ListLeafClusters":        {},
			"teleport.lib.teleterm.v1.TerminalService.ListRootClusters":        {},
			"teleport.lib.teleterm.v1.TerminalService.GetAuthSettings":         {},

			// Alternative implemented but needs to be deprecated in proto:
			"teleport.accesslist.v1.AccessListService.GetAccessLists": {},
			"proto.AuthService.GetWindowsDesktopServices":             {},
			"proto.AuthService.GetWindowsDesktops":                    {},

			// Paginated but does not support page sizes:
			"teleport.integration.v1.AWSOIDCService.ListDatabases":                {},
			"teleport.integration.v1.AWSOIDCService.ListDeployedDatabaseServices": {},
			"teleport.integration.v1.AWSOIDCService.ListEKSClusters":              {},
			"teleport.integration.v1.AWSOIDCService.ListSecurityGroups":           {},
			"teleport.integration.v1.AWSOIDCService.ListSubnets":                  {},
			"teleport.integration.v1.AWSOIDCService.ListVPCs":                     {},

			// RPCs unlikely to exceed grpc message limits:
			"proto.AuthService.GetAccessCapabilities":                                   {},
			"proto.AuthService.GetAccountRecoveryCodes":                                 {},
			"proto.AuthService.GetMFADevices":                                           {},
			"proto.AuthService.GetSSHTargets":                                           {},
			"proto.AuthService.ListReleases":                                            {},
			"teleport.plugins.v1.PluginService.GetAvailablePluginTypes":                 {},
			"teleport.trust.v1.TrustService.GetCertAuthorities":                         {},
			"proto.AuthService.GetAlertAcks":                                            {},
			"proto.AuthService.GetClusterAlerts":                                        {},
			"teleport.lib.teleterm.auto_update.v1.AutoUpdateService.GetClusterVersions": {},

			// Nested fields contain repeated fields or maps but unlikely to require pagination:
			"proto.AuthService.GetSSODiagnosticInfo":                          {},
			"teleport.devicetrust.v1.DeviceTrustService.GetDevice":            {},
			"teleport.lib.teleterm.vnet.v1.VnetService.GetServiceInfo":        {},
			"teleport.plugins.v1.PluginService.SearchPluginStaticCredentials": {},
			"teleport.scim.v1.SCIMService.GetSCIMResource":                    {},
			"teleport.secreports.v1.SecReportsService.GetSchema":              {},
		},
	}
}
