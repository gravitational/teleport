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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	yaml "gopkg.in/yaml.v3"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	vnet "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	workloadcluster "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	convertv1 "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	discoveryConfigConvertv1 "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/utils"
)

// kindVersionObject is used for unmarshaling Teleport resources so we can branch on
// their kinds and/or versions.
type kindVersionObject struct {
	Kind    string
	Version string
}

// jsonConverter is a function that transforms JSON bytes into a Teleport
// resource. The input is JSON rather than YAML because the script converts
// various resource representations (e.g., gogo-proto and RFD 153) into JSON
// before carrying out the conversion.
type jsonConverter func(data []byte) (tfgen.Resource, error)

// kubeConversionAttributes configures the way the converter transforms a tctl
// resource into a Kubernetes resource.
type kubeConversionAttributes struct {
	// subKindToResourceKind is a map of sub_kind values that, if present on
	// a tctl resource, determine the kind of the corresponding Kubernetes
	// resource.
	subKindToResourceKind map[string]string
	// apiVersion specifies the value of apiVersion in a Kubernetes
	// resource.
	apiVersion string
	// kind specifies the value of kind in a Kubernetes resource.
	kind string
	// ignoredSpecFields are fields in a tctl resource spec that the
	// converter removes from the final Kubernetes resource.
	ignoredSpecFields []string
	// optional version to require tctl resources to have before we convert
	// them. We use this for Kubernetes CRDs that are pinned to a specific
	// tctl resource version.
	requiredVersion string
}

// conversionRule is a configuration for how to transform a given kind of tctl
// resource into a Terraform provider and Kubernetes operator resource.
type conversionRule struct {
	// toTeleport is a function for converted JSON bytes (returned by
	// normalizing resource YAML) into a Teleport resource. Used for both
	// Terraform and Kubernetes resources.
	toTeleport jsonConverter
	// kubernetes is a set of configuration options for converting a tctl
	// resource into a Kubernetes resource.
	kubernetes kubeConversionAttributes
	// terraformResourceType is the type of the Terraform resource if it
	// differs from the value of kind in the corresponding tctl resource.
	terraformResourceType string
}

// unsupportedResource is an error indicating that a tctl resource does not have
// a corresponding resource for a particular infrastructure as code tool. Errors
// for unsupported resources must use this type for consistency.
type unsupportedResource struct {
	// kind is the resource kind, e.g., "role".
	kind string
	// version is an optional resource version. If empty, the message
	// ignores it.
	version string
	// tool is the infrastructure as code tool. Used only for printing error
	// messages.
	tool string
}

// Error prints the error message for unsupportedResource.
func (r unsupportedResource) Error() string {
	var verSuffix string
	if r.version != "" {
		verSuffix = " (" + r.version + ")"
	}
	return fmt.Sprintf(`%v does not support resource kind %v%v`, r.tool, r.kind, verSuffix)
}

// resourceConfig maps the kind values of resources supported by the Terraform
// provider to functions for converting JSON to HCL, as well as to functions for
// converting Teleport resource types to Kubernetes resources. There are three
// patterns for applying the conversion:
//  1. For legacy gogo-proto types, the YAML/JSON type directly maps to the
//     Protobuf-generated type, which includes json struct tags, so we can
//     unmarshal directly using utils.FastUnmarshal.
//  2. For types based on non-gogo Protobuf messages, unmarshal using
//     protojson.Unmarshal.
//  3. For resources that include a header, call utils.FastUnmarshal into the
//     internal representation of the type, then convert to a Protobuf-based type
//     and wrap with a header.
var resourceConfig = map[string]conversionRule{
	"role": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.RoleV6
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid Teleport role: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion:      "resources.teleport.dev/v1",
			kind:            "TeleportRoleV8",
			requiredVersion: "v8",
		},
	},
	"user": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.UserV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid user: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion:        "resources.teleport.dev/v2",
			kind:              "TeleportUser",
			ignoredSpecFields: []string{"local_auth", "expires", "created_by", "status"},
		},
	},
	"trusted_cluster": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.TrustedClusterV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid trusted_cluster: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion:        "resources.teleport.dev/v1",
			kind:              "TeleportTrustedClusterV2",
			ignoredSpecFields: []string{"roles"},
		},
	},
	"github": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.GithubConnectorV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid github connector: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion:        "resources.teleport.dev/v3",
			kind:              "TeleportGithubConnector",
			ignoredSpecFields: []string{"teams_to_logins"},
		},
		terraformResourceType: "teleport_github_connector",
	},
	"saml": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.SAMLConnectorV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid saml connector: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v2",
			kind:       "TeleportSAMLConnector",
		},
		terraformResourceType: "teleport_saml_connector",
	},
	"oidc": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.OIDCConnectorV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid oidc connector: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v3",
			kind:       "TeleportOIDCConnector",
		},
		terraformResourceType: "teleport_oidc_connector",
	},
	"token": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ProvisionTokenV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid token: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v2",
			kind:       "TeleportProvisionToken",
		},
		terraformResourceType: "teleport_provision_token",
	},
	"lock": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.LockV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid lock: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportLockV2",
		},
	},
	"cluster_networking_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ClusterNetworkingConfigV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_networking_config: %w", err)
			}
			return &r, nil
		},
	},
	"cluster_auth_preference": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.AuthPreferenceV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_auth_preference: %w", err)
			}
			return &r, nil
		},
		terraformResourceType: "teleport_auth_preference",
	},
	"bot": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r machineidv1.Bot
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid bot: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportBotV1",
		},
	},
	"autoupdate_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r autoupdatev1pb.AutoUpdateConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid autoupdate_config: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportAutoupdateConfigV1",
		},
	},
	"autoupdate_version": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r autoupdatev1pb.AutoUpdateVersion
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid autoupdate_version: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportAutoupdateVersionV1",
		},
	},
	"health_check_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r healthcheckconfigv1.HealthCheckConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid health_check_config: %w", err)
			}
			return &r, nil
		},
	},
	"workload_identity": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r workloadidentityv1.WorkloadIdentity
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid workload_identity: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportWorkloadIdentityV1",
		},
	},
	"app": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.AppV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid app: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportAppV3",
		},
	},
	"db": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.DatabaseV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid db: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportDatabaseV3",
		},
		terraformResourceType: "teleport_database",
	},
	"kube_cluster": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.KubernetesClusterV3
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid kube_cluster: %w", err)
			}
			return &r, nil
		},
	},
	"node": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ServerV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid node: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			subKindToResourceKind: map[string]string{
				"openssh":         "TeleportOpenSSHServerV2",
				"openssh-ec2-ice": "TeleportOpenSSHEICEServerV2",
			},
			ignoredSpecFields: []string{"cmd_labels", "component_features"},
		},
		terraformResourceType: "teleport_server",
	},
	"saml_idp_service_provider": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.SAMLIdPServiceProviderV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid saml_idp_service_provider: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportSAMLIdPServiceProviderV1",
		},
	},
	"access_list": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var al accesslist.AccessList
			if err := utils.FastUnmarshal(data, &al); err != nil {
				return nil, trace.Errorf("invalid access_list: %w", err)
			}
			return tfgen.WrapHeaderResource(convertv1.ToProto(&al)), nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportAccessList",
		},
	},
	"access_list_member": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var m accesslist.AccessListMember
			if err := utils.FastUnmarshal(data, &m); err != nil {
				return nil, trace.Errorf("invalid access_list_member: %w", err)
			}
			return tfgen.WrapHeaderResource(convertv1.ToMemberProto(&m)), nil
		},
	},
	"access_monitoring_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r accessmonitoringrulesv1.AccessMonitoringRule
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid access_monitoring_rule: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportAccessMonitoringRuleV1",
		},
	},
	"discovery_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var dc discoveryconfig.DiscoveryConfig
			if err := utils.FastUnmarshal(data, &dc); err != nil {
				return nil, trace.Errorf("invalid discovery_config: %w", err)
			}
			return tfgen.WrapHeaderResource(discoveryConfigConvertv1.ToProto(&dc)), nil
		},
	},
	"integration": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.IntegrationV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid integration: %w", err)
			}
			return &r, nil
		},
	},
	"okta_import_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.OktaImportRuleV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid okta_import_rule: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportOktaImportRule",
		},
	},
	"device": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.DeviceV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid device: %w", err)
			}
			return &r, nil
		},
		terraformResourceType: "teleport_device_trust",
	},
	"installer": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.InstallerV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid installer: %w", err)
			}
			return &r, nil
		},
	},
	"session_recording_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.SessionRecordingConfigV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid session_recording_config: %w", err)
			}
			return &r, nil
		},
	},
	"ui_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.UIConfigV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid ui_config: %w", err)
			}
			return &r, nil
		},
	},
	"cluster_maintenance_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.ClusterMaintenanceConfigV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_maintenance_config: %w", err)
			}
			return &r, nil
		},
	},
	"dynamic_windows_desktop": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r types.DynamicWindowsDesktopV1
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid dynamic_windows_desktop: %w", err)
			}
			return &r, nil
		},
	},
	"static_host_user": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r userprovisioningpb.StaticHostUser
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid static_host_user: %w", err)
			}
			return &r, nil
		},
	},
	"vnet_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r vnet.VnetConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid vnet_config: %w", err)
			}
			return &r, nil
		},
	},
	"app_auth_config": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r appauthconfigv1.AppAuthConfig
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid app_auth_config: %w", err)
			}
			return &r, nil
		},
	},
	"db_object_import_rule": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r dbobjectimportrulev1.DatabaseObjectImportRule
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid db_object_import_rule: %w", err)
			}
			return &r, nil
		},
	},
	"workload_cluster": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r workloadcluster.WorkloadCluster
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid workload_cluster: %w", err)
			}
			return &r, nil
		},
	},
	"inference_model": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.InferenceModel
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid inference_model: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportInferenceModel",
		},
	},
	"inference_secret": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.InferenceSecret
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid inference_secret: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportInferenceSecret",
		},
	},
	"inference_policy": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.InferencePolicy
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid inference_policy: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportInferencePolicy",
		},
	},
	"retrieval_model": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r summarizerv1.RetrievalModel
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid retrieval_model: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportRetrievalModelV1",
		},
	},
	"scoped_role": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r scopedaccessv1.ScopedRole
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid scoped_role: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportScopedRoleV1",
		},
	},
	"scoped_role_assignment": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r scopedaccessv1.ScopedRoleAssignment
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid scoped_role_assignment: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportScopedRoleAssignmentV1",
		},
	},
	"scoped_token": {
		toTeleport: func(data []byte) (tfgen.Resource, error) {
			var r joiningv1.ScopedToken
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid scoped_token: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v1",
			kind:       "TeleportScopedTokenV1",
		},
	},
}

// convertYAMLToHCL takes a single tctl resource YAML document, converts it to
// an HCL resource configuration, writing out the resulting HCL object to w.
func convertYAMLToHCL(w io.Writer, r io.Reader) error {
	var yamlBuf bytes.Buffer
	if _, err := io.Copy(&yamlBuf, r); err != nil {
		return trace.Errorf("unable to read input YAML: %w", err)
	}

	jsonbytes, err := utils.ToJSON(yamlBuf.Bytes())
	if err != nil {
		return trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)
	}

	var o kindVersionObject
	if err = json.Unmarshal(jsonbytes, &o); err != nil {
		return trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	convert, ok := resourceConfig[o.Kind]
	if !ok {
		return unsupportedResource{
			kind: o.Kind,
			tool: "Terraform",
		}
	}

	res, err := convert.toTeleport(jsonbytes)
	if err != nil {
		return err
	}

	var opts []tfgen.GenerateOpt
	if convert.terraformResourceType != "" {
		opts = append(opts, tfgen.WithResourceType(convert.terraformResourceType))
	}

	outbytes, err := tfgen.Generate(res, opts...)
	if err != nil {
		return trace.Errorf("unable to convert the provided YAML manifest into HCL: %w", err)
	}
	if _, err := w.Write(outbytes); err != nil {
		return trace.Errorf("unable to process the converted HCL: %w", err)
	}
	return nil
}

// sepPattern represents a YAML document separator. Used for splitting YAML
// documents to convert individual resources.
var sepPattern = regexp.MustCompile(`(?m)^---\s*$`)

// convertAllYAMLToHCL takes one or more tctl resource YAML documents with
// possible document separators in r, converts them to HCL resource
// configurations in a single document, writing out the document to w.
func convertAllYAMLToHCL(w io.Writer, r io.Reader) error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return trace.Errorf("could not read input document: %w", err)
	}

	docs := sepPattern.Split(buf.String(), -1)
	for i, doc := range docs {
		if doc == "" {
			// Skip empty documents, e.g., because of leading separator
			continue
		}
		if err := convertYAMLToHCL(w, strings.NewReader(doc)); err != nil {
			return err
		}

		if i+1 == len(docs) {
			break
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return trace.Errorf("unable to write out a document separator: %w", err)
		}
	}

	return nil
}

// convertAllYAMLToKubernetes takes one or more tctl resource YAML documents
// with possible document separators in r, converts them to Kubernetes resource
// manifests in a single document, writing out the document to w.
func convertAllYAMLToKubernetes(w io.Writer, r io.Reader) error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return trace.Errorf("could not read input document: %w", err)
	}

	docs := sepPattern.Split(buf.String(), -1)
	for i, doc := range docs {
		if doc == "" {
			// Skip empty documents, e.g., because of leading separator
			continue
		}
		if err := convertYAMLToKubernetes(w, strings.NewReader(doc)); err != nil {
			return err
		}

		if i+1 == len(docs) {
			break
		}
		if _, err := w.Write([]byte("---\n")); err != nil {
			return trace.Errorf("unable to write out a document separator: %w", err)
		}
	}

	return nil
}

// convertYAMLToKubernetes takes a single tctl resource YAML document and
// converts it to a Kubernetes resource manifest, writing out the resulting HCL
// object to w.
func convertYAMLToKubernetes(w io.Writer, r io.Reader) error {
	var yamlBuf bytes.Buffer
	if _, err := io.Copy(&yamlBuf, r); err != nil {
		return trace.Errorf("unable to read input YAML: %w", err)
	}

	jsonbytes, err := utils.ToJSON(yamlBuf.Bytes())
	if err != nil {
		return trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)
	}

	var original map[string]any
	if err = json.Unmarshal(jsonbytes, &original); err != nil {
		return trace.Errorf("unable to convert the input resource to a mapping: %w", err)
	}

	var o kindVersionObject
	if err = yaml.Unmarshal(jsonbytes, &o); err != nil {
		return trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	convert, ok := resourceConfig[o.Kind]
	if !ok || (convert.kubernetes.apiVersion == "" && convert.kubernetes.kind == "") {
		return unsupportedResource{
			kind: o.Kind,
			tool: "Kubernetes",
		}
	}

	// If there's a strict version requirement, reject the resource as
	// unsupported.
	if convert.kubernetes.requiredVersion != "" && o.Version != convert.kubernetes.requiredVersion {
		return unsupportedResource{
			kind:    o.Kind,
			tool:    "Kubernetes",
			version: o.Version,
		}
	}

	// Kubernetes resources have the same structure as tctl resources with a
	// few exceptions. To convert a tctl resource to a Kubernetes resource,
	// we add an apiVersion and kind suitable for Kubernetes, remove the
	// version (which is encoded in the kind), and handle two edge cases:
	//
	// - Some resources have a sub-kind that determines the CRD kind
	// - Some resources have fields that the Teleport Kubernetes operator
	//    ignores

	original["kind"] = convert.kubernetes.kind
	original["apiVersion"] = convert.kubernetes.apiVersion
	delete(original, "version")

	if convert.kubernetes.subKindToResourceKind != nil {
		sk, ok := original["sub_kind"]
		if !ok || sk == "" {
			return trace.Errorf("resource %v needs a sub_kind", o.Kind)
		}
		skval, ok := convert.kubernetes.subKindToResourceKind[sk.(string)]
		if !ok {
			return trace.Errorf("unrecognized sub_kind in resource %v", o.Kind)
		}
		original["kind"] = skval
	}
	delete(original, "sub_kind")

	spec, ok := original["spec"]
	specmap, mok := spec.(map[string]any)
	if ok && mok {
		for _, f := range convert.kubernetes.ignoredSpecFields {
			delete(specmap, f)
		}
	}

	// Kubernetes resources move the metadata.description in the tctl
	// resource to metadata.annotations.description
	if meta, ok := original["metadata"].(map[string]any); ok {
		if desc, ok := meta["description"]; ok {
			annotations, _ := meta["annotations"].(map[string]any)
			if annotations == nil {
				annotations = make(map[string]any)
			}
			annotations["description"] = desc
			meta["annotations"] = annotations
			delete(meta, "description")
		}
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(&original); err != nil {
		return trace.Errorf("unable to convert %v to Kubernetes YAML: %w", o.Kind, err)
	}

	return nil
}

func main() {
	format := flag.String("format", "", "hcl or kube")
	flag.Parse()

	var err error
	switch *format {
	case "kube":
		err = convertAllYAMLToKubernetes(os.Stdout, os.Stdin)

	case "hcl":
		err = convertAllYAMLToHCL(os.Stdout, os.Stdin)
	default:
		fmt.Fprintf(os.Stderr, `The format flag must be hcl or kube. Got: %v`, *format)
		os.Exit(1)
	}

	if err == nil {
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Cannot convert resource(s): %v", err)
	if errors.As(err, &unsupportedResource{}) {
		// We reserve 2 for unsupported resources so the invoking shell
		// knows that execution otherwise took place as expected.
		os.Exit(2)
	}

	os.Exit(1)
}
