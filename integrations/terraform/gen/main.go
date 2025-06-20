// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/integrations/terraform/gen/strcase"
)

// payload represents template payload
type payload struct {
	// Name represents resource name (capitalized)
	Name string
	// VarName represents resource variable name (underscored)
	VarName string
	// TypeName represents api/types resource type name
	TypeName string
	// IfaceName represents api/types interface for the (usually this is the same as Name)
	IfaceName string
	// GetMethod represents API get method name
	GetMethod string
	// CreateMethod represents API create method name
	CreateMethod string
	// UpdateMethod represents the API update method name.
	// On services without conditional updates, you can use the Update method.
	// On services with conditional updates, you must use the Upsert variant.
	UpdateMethod string
	// DeleteMethod represents API reset method used in singular resources
	DeleteMethod string
	// UpsertMethodArity represents Create/Update method arity, if it's 2, then the call signature would be "_, err :="
	UpsertMethodArity int
	// WithSecrets value for a withSecrets param of Get method (empty means no param used)
	WithSecrets string
	// ID id value on create and import
	ID string
	// IDPrefix is optional for resources which are stored with an prefix in the backend.
	IDPrefix string
	// RandomMetadataName indicates that Metadata.Name must be generated (supported by plural resources only)
	RandomMetadataName bool
	// UUIDMetadataName functions similar to RandomMetadataName but generates UUID instead of
	// generating 32 bit crypto random value
	UUIDMetadataName bool
	// Kind Teleport kind for a resource
	Kind string
	// DefaultVersion represents the default resource version on create
	DefaultVersion string
	// HasStaticID states whether this particular resource has a static (usually 0) Metadata.ID
	// This is relevant to cache enabled clusters: we use Metadata.ID to check if the resource was updated
	// Currently, the resources that don't have a dynamic Metadata.ID are strong consistent: oidc, github and saml connectors
	HasStaticID bool
	// ProtoPackagePath is the path of the package where the protobuf type of
	// the resource is defined.
	ProtoPackagePath string
	// ProtoPackagePath is the name of the package where the protobuf type of
	// the resource is defined.
	ProtoPackage string
	// SchemaPackagePath is the path of the package where the resource schema
	// definitions are defined.
	SchemaPackagePath string
	// SchemaPackagePath is the name of the package where the resource schema
	// definitions are defined.
	SchemaPackage string
	// IsPlainStruct states whether the resource type used by the API methods
	// for this resource is a plain struct, rather than an interface.
	IsPlainStruct bool
	// HasCheckAndSetDefaults indicates whether the resource type has the CheckAndSetDefaults method
	HasCheckAndSetDefaults bool
	// ExtraImports contains a list of imports that are being used.
	ExtraImports []string
	// TerraformResourceType represents the resource type in Terraform code.
	// e.g. `terraform import <resource_type>.<resource_name> identifier`.
	// This is also used to name the generated files.
	TerraformResourceType string
	// WithNonce is used to force upsert behavior for nonce protected values.
	WithNonce bool
	// ConvertPackagePath is the path of the package doing the conversion between protobuf and the go types.
	ConvertPackagePath string
	// ConvertToProtoFunc is the function converting the internal struct to the protobuf
	// struct. Defaults to "ToProto" if empty.
	ConvertToProtoFunc string
	// ConvertFromProtoFunc is the function converting the protobuf struct to the internal
	// struct. Defaults to "FromProto" if empty.
	ConvertFromProtoFunc string
	// PropagatedFields is a list of fields that must be copied from the
	// existing resource when we're updating it. For example:
	// "Spec.Audit.NextAuditDate" in AccessList resource
	PropagatedFields []string
	// Namespaced indicates that the resource get and delete methods need the
	// deprecated namespace parameter (always the default namespace).
	Namespaced bool
	// ForceSetKind indicates that the resource kind must be forcefully set by the provider.
	// This is required for some special resources (ServerV2) that support multiple kinds.
	// For those resources, we must set the kind, and don't want to have the user do it.
	ForceSetKind string
	// GetCanReturnNil is used to check for nil returned value when doing a Get<Resource>.
	GetCanReturnNil bool
	// DefaultName is the default singleton resource name. This is currently only supported for 153 resources.
	DefaultName string
}

func (p *payload) CheckAndSetDefaults() error {
	if p.ProtoPackage == "" {
		p.ProtoPackage = "apitypes"
	}
	if p.ProtoPackagePath == "" {
		p.ProtoPackagePath = "github.com/gravitational/teleport/api/types"
	}
	if p.SchemaPackage == "" {
		p.SchemaPackage = "tfschema"
	}
	if p.SchemaPackagePath == "" {
		p.SchemaPackagePath = "github.com/gravitational/teleport/integrations/terraform/tfschema"
	}
	return nil
}

const (
	pluralResource          = "plural_resource.go.tpl"
	pluralDataSource        = "plural_data_source.go.tpl"
	singularResource        = "singular_resource.go.tpl"
	singularDataSource      = "singular_data_source.go.tpl"
	outFileResourceFormat   = "provider/resource_%s.go"
	outFileDataSourceFormat = "provider/data_source_%s.go"
)

var (
	app = payload{
		Name:                   "App",
		TypeName:               "AppV3",
		VarName:                "app",
		IfaceName:              "Application",
		GetMethod:              "GetApp",
		CreateMethod:           "CreateApp",
		UpdateMethod:           "UpdateApp",
		DeleteMethod:           "DeleteApp",
		ID:                     `app.Metadata.Name`,
		Kind:                   "app",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_app",
		HasCheckAndSetDefaults: true,
	}

	authPreference = payload{
		Name:                   "AuthPreference",
		TypeName:               "AuthPreferenceV2",
		VarName:                "authPreference",
		GetMethod:              "GetAuthPreference",
		CreateMethod:           "UpsertAuthPreference",
		UpdateMethod:           "UpsertAuthPreference",
		UpsertMethodArity:      2,
		DeleteMethod:           "ResetAuthPreference",
		ID:                     `"auth_preference"`,
		Kind:                   "cluster_auth_preference",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_auth_preference",
		HasCheckAndSetDefaults: true,
	}

	clusterMaintenance = payload{
		Name:                   "ClusterMaintenanceConfig",
		TypeName:               "ClusterMaintenanceConfigV1",
		VarName:                "clusterMaintenanceConfig",
		GetMethod:              "GetClusterMaintenanceConfig",
		CreateMethod:           "UpdateClusterMaintenanceConfig",
		UpdateMethod:           "UpdateClusterMaintenanceConfig",
		DeleteMethod:           "DeleteClusterMaintenanceConfig",
		ID:                     `"cluster_maintenance_config"`,
		Kind:                   "cluster_maintenance_config",
		HasStaticID:            true,
		TerraformResourceType:  "teleport_cluster_maintenance_config",
		WithNonce:              true,
		GetCanReturnNil:        true,
		HasCheckAndSetDefaults: true,
	}

	clusterNetworking = payload{
		Name:                   "ClusterNetworkingConfig",
		TypeName:               "ClusterNetworkingConfigV2",
		VarName:                "clusterNetworkingConfig",
		GetMethod:              "GetClusterNetworkingConfig",
		CreateMethod:           "UpsertClusterNetworkingConfig",
		UpdateMethod:           "UpsertClusterNetworkingConfig",
		UpsertMethodArity:      2,
		DeleteMethod:           "ResetClusterNetworkingConfig",
		ID:                     `"cluster_networking_config"`,
		Kind:                   "cluster_networking_config",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_cluster_networking_config",
		HasCheckAndSetDefaults: true,
	}

	database = payload{
		Name:                   "Database",
		TypeName:               "DatabaseV3",
		VarName:                "database",
		GetMethod:              "GetDatabase",
		CreateMethod:           "CreateDatabase",
		UpdateMethod:           "UpdateDatabase",
		DeleteMethod:           "DeleteDatabase",
		ID:                     `database.Metadata.Name`,
		Kind:                   "db",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_database",
		HasCheckAndSetDefaults: true,
	}

	dynamicWindowsDesktop = payload{
		Name:                   "DynamicWindowsDesktop",
		TypeName:               "DynamicWindowsDesktopV1",
		VarName:                "desktop",
		IfaceName:              "DynamicWindowsDesktop",
		GetMethod:              "DynamicDesktopClient().GetDynamicWindowsDesktop",
		CreateMethod:           "DynamicDesktopClient().CreateDynamicWindowsDesktop",
		UpdateMethod:           "DynamicDesktopClient().UpsertDynamicWindowsDesktop",
		DeleteMethod:           "DynamicDesktopClient().DeleteDynamicWindowsDesktop",
		UpsertMethodArity:      2,
		ID:                     `desktop.Metadata.Name`,
		Kind:                   "dynamic_windows_desktop",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_dynamic_windows_desktop",
		HasCheckAndSetDefaults: true,
	}

	githubConnector = payload{
		Name:                   "GithubConnector",
		TypeName:               "GithubConnectorV3",
		VarName:                "githubConnector",
		GetMethod:              "GetGithubConnector",
		CreateMethod:           "CreateGithubConnector",
		UpdateMethod:           "UpsertGithubConnector",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteGithubConnector",
		WithSecrets:            "true",
		ID:                     "githubConnector.Metadata.Name",
		Kind:                   "github",
		HasStaticID:            true,
		TerraformResourceType:  "teleport_github_connector",
		HasCheckAndSetDefaults: true,
	}

	oidcConnector = payload{
		Name:                   "OIDCConnector",
		TypeName:               "OIDCConnectorV3",
		VarName:                "oidcConnector",
		GetMethod:              "GetOIDCConnector",
		CreateMethod:           "CreateOIDCConnector",
		UpdateMethod:           "UpsertOIDCConnector",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteOIDCConnector",
		WithSecrets:            "true",
		ID:                     "oidcConnector.Metadata.Name",
		Kind:                   "oidc",
		HasStaticID:            true,
		TerraformResourceType:  "teleport_oidc_connector",
		HasCheckAndSetDefaults: true,
	}

	samlConnector = payload{
		Name:                   "SAMLConnector",
		TypeName:               "SAMLConnectorV2",
		VarName:                "samlConnector",
		GetMethod:              "GetSAMLConnector",
		CreateMethod:           "CreateSAMLConnector",
		UpdateMethod:           "UpsertSAMLConnector",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteSAMLConnector",
		WithSecrets:            "true",
		ID:                     "samlConnector.Metadata.Name",
		Kind:                   "saml",
		HasStaticID:            true,
		TerraformResourceType:  "teleport_saml_connector",
		HasCheckAndSetDefaults: true,
	}

	provisionToken = payload{
		Name:                   "ProvisionToken",
		TypeName:               "ProvisionTokenV2",
		VarName:                "provisionToken",
		GetMethod:              "GetToken",
		CreateMethod:           "UpsertToken",
		UpdateMethod:           "UpsertToken",
		DeleteMethod:           "DeleteToken",
		ID:                     "provisionToken.Metadata.Revision", // must be a string
		RandomMetadataName:     true,
		Kind:                   "token",
		HasStaticID:            false,
		SchemaPackage:          "token",
		TerraformResourceType:  "teleport_provision_token",
		HasCheckAndSetDefaults: true,
	}

	role = payload{
		Name:                   "Role",
		TypeName:               "RoleV6",
		VarName:                "role",
		GetMethod:              "GetRole",
		CreateMethod:           "CreateRole",
		UpdateMethod:           "UpsertRole",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteRole",
		ID:                     "role.Metadata.Name",
		Kind:                   "role",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_role",
		HasCheckAndSetDefaults: true,
	}

	sessionRecording = payload{
		Name:                   "SessionRecordingConfig",
		TypeName:               "SessionRecordingConfigV2",
		VarName:                "sessionRecordingConfig",
		GetMethod:              "GetSessionRecordingConfig",
		CreateMethod:           "UpsertSessionRecordingConfig",
		UpdateMethod:           "UpsertSessionRecordingConfig",
		UpsertMethodArity:      2,
		DeleteMethod:           "ResetSessionRecordingConfig",
		ID:                     `"session_recording_config"`,
		Kind:                   "session_recording_config",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_session_recording_config",
		HasCheckAndSetDefaults: true,
	}

	trustedCluster = payload{
		Name:                   "TrustedCluster",
		TypeName:               "TrustedClusterV2",
		VarName:                "trustedCluster",
		GetMethod:              "GetTrustedCluster",
		CreateMethod:           "UpsertTrustedCluster",
		UpdateMethod:           "UpsertTrustedCluster",
		DeleteMethod:           "DeleteTrustedCluster",
		UpsertMethodArity:      2,
		ID:                     "trustedCluster.Metadata.Name",
		Kind:                   "trusted_cluster",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_trusted_cluster",
		HasCheckAndSetDefaults: true,
	}

	user = payload{
		Name:                   "User",
		TypeName:               "UserV2",
		VarName:                "user",
		GetMethod:              "GetUser",
		CreateMethod:           "CreateUser",
		UpdateMethod:           "UpsertUser",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteUser",
		WithSecrets:            "false",
		ID:                     "user.Metadata.Name",
		Kind:                   "user",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_user",
		HasCheckAndSetDefaults: true,
	}

	loginRule = payload{
		Name:                  "LoginRule",
		TypeName:              "LoginRule",
		VarName:               "loginRule",
		GetMethod:             "GetLoginRule",
		CreateMethod:          "UpsertLoginRule",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertLoginRule",
		DeleteMethod:          "DeleteLoginRule",
		ID:                    "loginRule.Metadata.Name",
		Kind:                  "login_rule",
		HasStaticID:           true,
		ProtoPackage:          "loginrulev1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/loginrule/v1",
		IsPlainStruct:         true,
		TerraformResourceType: "teleport_login_rule",
	}

	deviceTrust = payload{
		Name:                  "DeviceV1",
		VarName:               "trustedDevice",
		TypeName:              "DeviceV1",
		GetMethod:             "GetDeviceResource",
		CreateMethod:          "UpsertDeviceResource",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertDeviceResource",
		DeleteMethod:          "DeleteDeviceResource",
		Kind:                  "device",
		ID:                    "trustedDevice.Metadata.Name",
		HasStaticID:           true,
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/devicetrust/v1",
		IsPlainStruct:         true,
		UUIDMetadataName:      true,
		TerraformResourceType: "teleport_device_trust",
	}

	oktaImportRule = payload{
		Name:                   "OktaImportRule",
		TypeName:               "OktaImportRuleV1",
		VarName:                "oktaImportRule",
		IfaceName:              "OktaImportRule",
		GetMethod:              "OktaClient().GetOktaImportRule",
		CreateMethod:           "OktaClient().CreateOktaImportRule",
		UpdateMethod:           "OktaClient().UpdateOktaImportRule",
		DeleteMethod:           "OktaClient().DeleteOktaImportRule",
		UpsertMethodArity:      2,
		ID:                     "oktaImportRule.Metadata.Name",
		Kind:                   "okta_import_rule",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_okta_import_rule",
		HasCheckAndSetDefaults: true,
	}

	accessList = payload{
		Name:                   "AccessList",
		TypeName:               "AccessList",
		VarName:                "accessList",
		GetMethod:              "AccessListClient().GetAccessList",
		CreateMethod:           "AccessListClient().UpsertAccessList",
		UpsertMethodArity:      2,
		UpdateMethod:           "AccessListClient().UpsertAccessList",
		DeleteMethod:           "AccessListClient().DeleteAccessList",
		ID:                     "accessList.Header.Metadata.Name",
		Kind:                   "access_list",
		HasStaticID:            false,
		SchemaPackage:          "schemav1",
		SchemaPackagePath:      "github.com/gravitational/teleport/integrations/terraform/tfschema/accesslist/v1",
		ProtoPackage:           "accesslist",
		ProtoPackagePath:       "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1",
		TerraformResourceType:  "teleport_access_list",
		ConvertPackagePath:     "github.com/gravitational/teleport/api/types/accesslist/convert/v1",
		HasCheckAndSetDefaults: true,
		PropagatedFields:       []string{"Spec.Audit.NextAuditDate"},
	}

	server = payload{
		Name:                   "Server",
		TypeName:               "ServerV2",
		VarName:                "server",
		GetMethod:              "GetNode",
		CreateMethod:           "UpsertNode",
		UpdateMethod:           "UpsertNode",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteNode",
		ID:                     "server.Metadata.Name",
		Kind:                   "node",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_server",
		HasCheckAndSetDefaults: true,
		Namespaced:             true,
		ForceSetKind:           "apitypes.KindNode",
	}

	installer = payload{
		Name:                   "Installer",
		TypeName:               "InstallerV1",
		VarName:                "installer",
		GetMethod:              "GetInstaller",
		CreateMethod:           "SetInstaller",
		UpdateMethod:           "SetInstaller",
		DeleteMethod:           "DeleteInstaller",
		ID:                     `"installer"`,
		Kind:                   "installer",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_installer",
		HasCheckAndSetDefaults: true,
	}

	accessMonitoringRule = payload{
		Name:                  "AccessMonitoringRule",
		TypeName:              "AccessMonitoringRule",
		VarName:               "accessMonitoringRule",
		GetMethod:             "AccessMonitoringRulesClient().GetAccessMonitoringRule",
		CreateMethod:          "AccessMonitoringRulesClient().CreateAccessMonitoringRule",
		UpsertMethodArity:     2,
		UpdateMethod:          "AccessMonitoringRulesClient().UpdateAccessMonitoringRule",
		DeleteMethod:          "AccessMonitoringRulesClient().DeleteAccessMonitoringRule",
		ID:                    "accessMonitoringRule.Metadata.Name",
		Kind:                  "access_monitoring_rule",
		HasStaticID:           false,
		ProtoPackage:          "accessmonitoringrulesv1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/accessmonitoringrules/v1",
		TerraformResourceType: "teleport_access_monitoring_rule",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindAccessMonitoringRule",
	}

	staticHostUser = payload{
		Name:                  "StaticHostUser",
		TypeName:              "StaticHostUser",
		VarName:               "staticHostUser",
		GetMethod:             "StaticHostUserClient().GetStaticHostUser",
		CreateMethod:          "StaticHostUserClient().CreateStaticHostUser",
		UpsertMethodArity:     2,
		UpdateMethod:          "StaticHostUserClient().UpsertStaticHostUser",
		DeleteMethod:          "StaticHostUserClient().DeleteStaticHostUser",
		ID:                    "staticHostUser.Metadata.Name",
		Kind:                  "static_host_user",
		HasStaticID:           false,
		ProtoPackage:          "userprovisioningv2",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/userprovisioning/v2",
		TerraformResourceType: "teleport_static_host_user",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindStaticHostUser",
	}

	workloadIdentity = payload{
		Name:                  "WorkloadIdentity",
		TypeName:              "WorkloadIdentity",
		VarName:               "workloadIdentity",
		GetMethod:             "GetWorkloadIdentity",
		CreateMethod:          "CreateWorkloadIdentity",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertWorkloadIdentity",
		DeleteMethod:          "DeleteWorkloadIdentity",
		ID:                    "workloadIdentity.Metadata.Name",
		Kind:                  "workload_identity",
		HasStaticID:           false,
		ProtoPackage:          "workloadidentityv1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/workloadidentity/v1",
		TerraformResourceType: "teleport_workload_identity",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ForceSetKind: `"workload_identity"`,
	}

	autoUpdateVersion = payload{
		Name:                  "AutoUpdateVersion",
		TypeName:              "AutoUpdateVersion",
		VarName:               "autoUpdateVersion",
		GetMethod:             "GetAutoUpdateVersion",
		CreateMethod:          "CreateAutoUpdateVersion",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertAutoUpdateVersion",
		DeleteMethod:          "DeleteAutoUpdateVersion",
		ID:                    "autoUpdateVersion.Metadata.Name",
		Kind:                  "autoupdate_version",
		HasStaticID:           false,
		ProtoPackage:          "autoupdatev1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/autoupdate/v1",
		TerraformResourceType: "teleport_autoupdate_version",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindAutoUpdateVersion",
		DefaultName:  "apitypes.MetaNameAutoUpdateVersion",
	}

	autoUpdateConfig = payload{
		Name:                  "AutoUpdateConfig",
		TypeName:              "AutoUpdateConfig",
		VarName:               "autoUpdateConfig",
		GetMethod:             "GetAutoUpdateConfig",
		CreateMethod:          "CreateAutoUpdateConfig",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertAutoUpdateConfig",
		DeleteMethod:          "DeleteAutoUpdateConfig",
		ID:                    "autoUpdateConfig.Metadata.Name",
		Kind:                  "autoupdate_config",
		HasStaticID:           false,
		ProtoPackage:          "autoupdatev1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/autoupdate/v1",
		TerraformResourceType: "teleport_autoupdate_config",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindAutoUpdateConfig",
		DefaultName:  "apitypes.MetaNameAutoUpdateConfig",
	}

	healthCheckConfig = payload{
		Name:                  "HealthCheckConfig",
		TypeName:              "HealthCheckConfig",
		VarName:               "healthCheckConfig",
		GetMethod:             "GetHealthCheckConfig",
		CreateMethod:          "CreateHealthCheckConfig",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertHealthCheckConfig",
		DeleteMethod:          "DeleteHealthCheckConfig",
		ID:                    "healthCheckConfig.Metadata.Name",
		Kind:                  "health_check_config",
		HasStaticID:           false,
		ProtoPackage:          "healthcheckconfigv1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/healthcheckconfig/v1",
		TerraformResourceType: "teleport_health_check_config",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindHealthCheckConfig",
	}
)

func main() {
	genTFSchema()
}

func genTFSchema() {
	generateResource(app, pluralResource)
	generateDataSource(app, pluralDataSource)
	generateResource(authPreference, singularResource)
	generateDataSource(authPreference, singularDataSource)
	generateResource(clusterMaintenance, singularResource)
	generateDataSource(clusterMaintenance, singularDataSource)
	generateResource(clusterNetworking, singularResource)
	generateDataSource(clusterNetworking, singularDataSource)
	generateResource(database, pluralResource)
	generateDataSource(database, pluralDataSource)
	generateResource(dynamicWindowsDesktop, pluralResource)
	generateDataSource(dynamicWindowsDesktop, pluralDataSource)
	generateResource(githubConnector, pluralResource)
	generateDataSource(githubConnector, pluralDataSource)
	generateResource(oidcConnector, pluralResource)
	generateDataSource(oidcConnector, pluralDataSource)
	generateResource(samlConnector, pluralResource)
	generateDataSource(samlConnector, pluralDataSource)
	generateResource(provisionToken, pluralResource)
	generateDataSource(provisionToken, pluralDataSource)
	generateResource(role, pluralResource)
	generateDataSource(role, pluralDataSource)
	generateResource(trustedCluster, pluralResource)
	generateDataSource(trustedCluster, pluralDataSource)
	generateResource(sessionRecording, singularResource)
	generateDataSource(sessionRecording, singularDataSource)
	generateResource(user, pluralResource)
	generateDataSource(user, pluralDataSource)
	generateResource(loginRule, pluralResource)
	generateDataSource(loginRule, pluralDataSource)
	generateResource(deviceTrust, pluralResource)
	generateDataSource(deviceTrust, pluralDataSource)
	generateResource(oktaImportRule, pluralResource)
	generateDataSource(oktaImportRule, pluralDataSource)
	generateResource(accessList, pluralResource)
	generateDataSource(accessList, pluralDataSource)
	generateResource(server, pluralResource)
	generateDataSource(server, pluralDataSource)
	generateResource(installer, pluralResource)
	generateDataSource(installer, pluralDataSource)
	generateResource(accessMonitoringRule, pluralResource)
	generateDataSource(accessMonitoringRule, pluralDataSource)
	generateResource(staticHostUser, pluralResource)
	generateDataSource(staticHostUser, pluralDataSource)
	generateResource(workloadIdentity, pluralResource)
	generateDataSource(workloadIdentity, pluralDataSource)
	generateResource(autoUpdateVersion, singularResource)
	generateDataSource(autoUpdateVersion, singularDataSource)
	generateResource(autoUpdateConfig, singularResource)
	generateDataSource(autoUpdateConfig, singularDataSource)
	generateResource(healthCheckConfig, pluralResource)
	generateDataSource(healthCheckConfig, pluralDataSource)
}

func generateResource(p payload, tpl string) {
	outFile := fmt.Sprintf(outFileResourceFormat, p.TerraformResourceType)
	generate(p, tpl, outFile)
}
func generateDataSource(p payload, tpl string) {
	outFile := fmt.Sprintf(outFileDataSourceFormat, p.TerraformResourceType)
	generate(p, tpl, outFile)
}

func generate(p payload, tpl, outFile string) {
	if err := p.CheckAndSetDefaults(); err != nil {
		log.Fatal(err)
	}

	funcs := template.FuncMap{
		"join":    strings.Join,
		"split":   strings.Split,
		"toSnake": toSnake,
		"schemaImport": func(p payload) string {
			if p.SchemaPackage == "tfschema" {
				return `"` + p.SchemaPackagePath + `"`
			}

			return p.SchemaPackage + ` "` + p.SchemaPackagePath + `"`
		},
		"protoImport": func(p payload) string {
			if p.ConvertPackagePath != "" {
				return "convert" + ` "` + p.ConvertPackagePath + `"`
			}

			return p.ProtoPackage + ` "` + p.ProtoPackagePath + `"`
		},
	}

	t, err := template.New(p.Name).Funcs(funcs).ParseFiles(path.Join("gen", tpl))
	if err != nil {
		log.Fatal(err)
	}

	var b bytes.Buffer
	err = t.ExecuteTemplate(&b, tpl, p)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(outFile, b.Bytes(), 0777)
	if err != nil {
		log.Fatal(err)
	}
}

// ToSnake converts a string to snake_case ignoring "." characters.
func toSnake(s string) string {
	return strcase.ToScreamingDelimited(s, '_', ".", false)
}
