// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package legacy

import "github.com/hashicorp/terraform-plugin-framework/tfsdk"

// DataSourceTypes returns all legacy Teleport data source types.
func DataSourceTypes() map[string]tfsdk.DataSourceType {
	return map[string]tfsdk.DataSourceType{
		"teleport_app_auth_config":            dataSourceTeleportAppAuthConfigType{},
		"teleport_auth_preference":            dataSourceTeleportAuthPreferenceType{},
		"teleport_cluster_maintenance_config": dataSourceTeleportClusterMaintenanceConfigType{},
		"teleport_cluster_networking_config":  dataSourceTeleportClusterNetworkingConfigType{},
		"teleport_database":                   dataSourceTeleportDatabaseType{},
		"teleport_discovery_config":           dataSourceTeleportDiscoveryConfigType{},
		"teleport_dynamic_windows_desktop":    dataSourceTeleportDynamicWindowsDesktopType{},
		"teleport_github_connector":           dataSourceTeleportGithubConnectorType{},
		"teleport_lock":                       dataSourceTeleportLockType{},
		"teleport_provision_token":            dataSourceTeleportProvisionTokenType{},
		"teleport_oidc_connector":             dataSourceTeleportOIDCConnectorType{},
		"teleport_role":                       dataSourceTeleportRoleType{},
		"teleport_saml_connector":             dataSourceTeleportSAMLConnectorType{},
		"teleport_saml_idp_service_provider":  dataSourceTeleportSAMLIdPServiceProviderType{},
		"teleport_session_recording_config":   dataSourceTeleportSessionRecordingConfigType{},
		"teleport_trusted_cluster":            dataSourceTeleportTrustedClusterType{},
		"teleport_ui_config":                  dataSourceTeleportUIConfigType{},
		"teleport_user":                       dataSourceTeleportUserType{},
		"teleport_login_rule":                 dataSourceTeleportLoginRuleType{},
		"teleport_trusted_device":             dataSourceTeleportDeviceV1Type{},
		"teleport_okta_import_rule":           dataSourceTeleportOktaImportRuleType{},
		"teleport_installer":                  dataSourceTeleportInstallerType{},
		"teleport_access_monitoring_rule":     dataSourceTeleportAccessMonitoringRuleType{},
		"teleport_static_host_user":           dataSourceTeleportStaticHostUserType{},
		"teleport_workload_identity":          dataSourceTeleportWorkloadIdentityType{},
		"teleport_autoupdate_version":         dataSourceTeleportAutoUpdateVersionType{},
		"teleport_autoupdate_config":          dataSourceTeleportAutoUpdateConfigType{},
		"teleport_health_check_config":        dataSourceTeleportHealthCheckConfigType{},
		"teleport_vnet_config":                dataSourceTeleportVnetConfigType{},
		"teleport_integration":                dataSourceTeleportIntegrationType{},
		"teleport_db_object_import_rule":      dataSourceTeleportDatabaseObjectImportRuleType{},
		"teleport_classifier":                 dataSourceTeleportClassifierType{},
		// TODO(bl-nero): Add teleport_inference_* data sources after data sources
		// are fixed. The current problems with data sources include:
		// - Data sources only perform a "shallow fill", which means only setting
		//   leaf-level fields.
		// - Data sources use the same schema as resources, which means that fields
		//   required on a resource also need to be set on the data source
		//   definition.
		"teleport_workload_cluster":      dataSourceTeleportWorkloadClusterType{},
		"teleport_client_ip_restriction": dataSourceTeleportClientIPRestrictionType{},
	}
}

// ResourceTypes returns all legacy Teleport resource types.
func ResourceTypes() map[string]tfsdk.ResourceType {
	return map[string]tfsdk.ResourceType{
		"teleport_app_auth_config":            resourceTeleportAppAuthConfigType{},
		"teleport_auth_preference":            resourceTeleportAuthPreferenceType{},
		"teleport_cluster_maintenance_config": resourceTeleportClusterMaintenanceConfigType{},
		"teleport_cluster_networking_config":  resourceTeleportClusterNetworkingConfigType{},
		"teleport_database":                   resourceTeleportDatabaseType{},
		"teleport_discovery_config":           resourceTeleportDiscoveryConfigType{},
		"teleport_dynamic_windows_desktop":    resourceTeleportDynamicWindowsDesktopType{},
		"teleport_github_connector":           resourceTeleportGithubConnectorType{},
		"teleport_lock":                       resourceTeleportLockType{},
		"teleport_provision_token":            resourceTeleportProvisionTokenType{},
		"teleport_oidc_connector":             resourceTeleportOIDCConnectorType{},
		"teleport_role":                       resourceTeleportRoleType{},
		"teleport_saml_connector":             resourceTeleportSAMLConnectorType{},
		"teleport_saml_idp_service_provider":  resourceTeleportSAMLIdPServiceProviderType{},
		"teleport_session_recording_config":   resourceTeleportSessionRecordingConfigType{},
		"teleport_trusted_cluster":            resourceTeleportTrustedClusterType{},
		"teleport_ui_config":                  resourceTeleportUIConfigType{},
		"teleport_user":                       resourceTeleportUserType{},
		"teleport_bot":                        resourceTeleportBotType{},
		"teleport_login_rule":                 resourceTeleportLoginRuleType{},
		"teleport_trusted_device":             resourceTeleportDeviceV1Type{},
		"teleport_okta_import_rule":           resourceTeleportOktaImportRuleType{},
		"teleport_server":                     resourceTeleportServerType{},
		"teleport_installer":                  resourceTeleportInstallerType{},
		"teleport_access_monitoring_rule":     resourceTeleportAccessMonitoringRuleType{},
		"teleport_static_host_user":           resourceTeleportStaticHostUserType{},
		"teleport_workload_identity":          resourceTeleportWorkloadIdentityType{},
		"teleport_autoupdate_version":         resourceTeleportAutoUpdateVersionType{},
		"teleport_autoupdate_config":          resourceTeleportAutoUpdateConfigType{},
		"teleport_health_check_config":        resourceTeleportHealthCheckConfigType{},
		"teleport_vnet_config":                resourceTeleportVnetConfigType{},
		"teleport_integration":                resourceTeleportIntegrationType{},
		"teleport_inference_model":            resourceTeleportInferenceModelType{},
		"teleport_inference_secret":           resourceTeleportInferenceSecretType{},
		"teleport_inference_policy":           resourceTeleportInferencePolicyType{},
		"teleport_classifier":                 resourceTeleportClassifierType{},
		"teleport_retrieval_model":            resourceTeleportRetrievalModelType{},
		"teleport_workload_cluster":           resourceTeleportWorkloadClusterType{},
		"teleport_db_object_import_rule":      resourceTeleportDatabaseObjectImportRuleType{},
		"teleport_client_ip_restriction":      resourceTeleportClientIPRestrictionType{},
	}
}
