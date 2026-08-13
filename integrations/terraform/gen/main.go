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
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	template "github.com/DataDog/datadog-agent/pkg/template/text"

	"github.com/gravitational/teleport/integrations/terraform/gen/strcase"
)

// payload represents template payload
type payload struct {
	// Name represents resource name (PascalCase)
	Name string
	// VarName represents resource variable name (camelCase)
	VarName string
	// TypeName represents api/types resource type name
	TypeName string
	// IfaceName represents the name of the api/types interface. Relevant only if
	// the API methods used operate on interfaces instead of concrete types;
	// otherwise, this should be empty.
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
	// ProtoPackage is the local name alias of the package where the protobuf
	// type of the resource is defined.
	ProtoPackage string
	// SchemaPackagePath is the path of the package where the resource schema
	// definitions are defined.
	SchemaPackagePath string
	// SchemaPackage is the local name alias of the package where the resource
	// schema definitions are defined.
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
	// ForceSetKind indicates that the resource kind must be forcefully set by
	// the provider. This is required for some special resources (ServerV2) that
	// support multiple kinds or for [RFD 153] resources. For those resources, we
	// must set the kind, and don't want to have the user do it.
	//
	// [RFD 153]: https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md
	ForceSetKind string
	// GetCanReturnNil is used to check for nil returned value when doing a Get<Resource>.
	GetCanReturnNil bool
	// DefaultName is the default singleton resource name. This is currently only supported for 153 resources.
	DefaultName string
	// SaveSpecStateFromPlan indicates whether the spec field state should be
	// taken from plan (true) or from the value returned by the GetMethod after
	// the resource has been created. This is needed if the resource contains
	// write-only fields that the server never returns. (We can't use Terraform's
	// built-in write-only fields, since these require Terraform v1.11.)
	SaveSpecStateFromPlan bool
	// WithoutImportState skips generating the ImportState function, which may be
	// not supported for resources with write-only fields.
	WithoutImportState bool
	// StatePoll optionally configures polling for state changes when creating or updating resources.
	StatePoll *statePoll
	// RequestWrapper optionally configures envelope request/response types for CRUD
	// operations following the RFD 153 resource guidelines.
	// This must only be set for resources whose client use envelope structs.
	RequestWrapper *RequestWrapper
	// DefaultSubKind is the Teleport sub_kind that is used when the user does
	// not specify one in the resource config. The user-provided sub_kind (on
	// the resource or in state) takes precedence; this is only a fallback.
	DefaultSubKind string
	// WithoutModifyPlan skips generation of the ModifyPlan function, which may
	// not be supported, or may have been manually implemented.
	WithoutModifyPlan bool
}

// statePoll configures polling for state changes when creating or updating resources.
type statePoll struct {
	// StatePath is the object path to the current state to observe from the object returned from the provided GetMethod.
	// StatePath provides a path to lookup the current state from the returned object from GetMethod.
	// Each string is treated as a field name for a nested object. It's expected
	// the combined path points to a string value. All other paths along the way must be pointers
	// to structs.
	// Example: []string{"Status", "State"} would expect the object returned from GetMethod to have a Status
	// field that is a pointer to a struct. That struct must have a State field. The State field must
	// be a string.
	// TODO(dustinspecker): this is only supported for Create methods. Consider adding for Update methods in the future.
	// TODO(dustinspecker): support slices and other types besides pointers to structs
	StatePath []string
	// PendingStates is a list of states that are valid while polling the resource to reach a target state. Any state
	// that is found that is not in PendingStates or TargetStates is considered a terminal error.
	PendingStates []string
	// TargetStates is a list of possible states that indicate a resource is ready for usage while polling. Any state
	// that is found that is not in PendingStates or TargetStates is considered a terminal error.
	TargetStates []string
	// StatePollIntervalSeconds is how long to wait before polling the pending resource again.
	StatePollIntervalSeconds int
	// StateTimeoutSeconds is the maximum amount of seconds to wait for a resource to reach a target state.
	StateTimeoutSeconds int
}

// RequestWrapper will wrap the resource types defined in RFD 153 suggested conventions for the client.
// RFD 153 specifies suggests using request/response wrappers and consistent naming conventions for those types.
//   - Request types:  {MethodName}Request  ex) CreateFooRequest, GetFooRequest, UpsertFooRequest, DeleteFooRequest
//   - Response types: {MethodName}Response ex) CreateFooResponse, GetFooResponse, UpsertFooResponse, DeleteFooResponse
//   - Resource field: The inner resource type itself that the request/response types wrap.
//   - Response getter: Get + field name to get the inner resource.
type RequestWrapper struct {
	// RequestResourceField is the field name in request
	// types that holds the resource object - ex) "Role" in ScopedRoles.
	RequestResourceField string
	// ReturnsUnwrappedResource describes whether the response needs to be unwrapped.
	// Should be false if `GetRequest` returns a GetResponse struct.
	// Should be true if `GetRequest` returns the resource itself.
	ReturnsUnwrappedResource bool

	// GetRequest is the type name for the Get request - ex) "GetScopedRoleRequest".
	GetRequest string
	// CreateRequest is the type name for the Create request - ex) "CreateScopedRoleRequest".
	CreateRequest string
	// UpdateRequest is the type name for the Update/Upsert request -ex) "UpsertScopedRoleRequest".
	UpdateRequest string
	// DeleteRequest is the type name for the Delete request - ex) "DeleteScopedRoleRequest".
	DeleteRequest string
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
	if p.StatePoll != nil {
		if len(p.StatePoll.StatePath) == 0 {
			return errors.New("StatePath must be provided when StatePoll is set")
		}

		if len(p.StatePoll.PendingStates) == 0 {
			return errors.New("PendingStates must be provided when StatePoll is set")
		}

		if len(p.StatePoll.TargetStates) == 0 {
			return errors.New("TargetStates must be provided when StatePoll is set")
		}

		if p.StatePoll.StatePollIntervalSeconds == 0 {
			return errors.New("StatePollIntervalSeconds must be provided when StatePoll is set")
		}

		if p.StatePoll.StateTimeoutSeconds == 0 {
			return errors.New("StateTimeoutSeconds must be provided when StatePoll is set")
		}
	}
	if p.RequestWrapper != nil {
		if p.RequestWrapper.RequestResourceField == "" {
			return errors.New("RequestResourceField must be provided when RequestWrapper is set")
		}
		if p.RequestWrapper.GetRequest == "" {
			return errors.New("GetRequest must be provided when RequestWrapper is set")
		}
		if p.RequestWrapper.CreateRequest == "" {
			return errors.New("CreateRequest must be provided when RequestWrapper is set")
		}
		if p.RequestWrapper.UpdateRequest == "" {
			return errors.New("UpdateRequest must be provided when RequestWrapper is set")
		}
		if p.RequestWrapper.DeleteRequest == "" {
			return errors.New("DeleteRequest must be provided when RequestWrapper is set")
		}
	}
	return nil
}

const (
	pluralResource          = "plural_resource.go.tpl"
	pluralDataSource        = "plural_data_source.go.tpl"
	singularResource        = "singular_resource.go.tpl"
	singularDataSource      = "singular_data_source.go.tpl"
	outFileResourceFormat   = "provider/internal/legacy/resource_%s.go"
	outFileDataSourceFormat = "provider/internal/legacy/data_source_%s.go"
)

var (
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

	samlIdPServiceProvider = payload{
		Name:                   "SAMLIdPServiceProvider",
		TypeName:               "SAMLIdPServiceProviderV1",
		VarName:                "samlIdPServiceProvider",
		IfaceName:              "SAMLIdPServiceProvider",
		GetMethod:              "GetSAMLIdPServiceProvider",
		CreateMethod:           "CreateSAMLIdPServiceProvider",
		UpdateMethod:           "UpdateSAMLIdPServiceProvider",
		DeleteMethod:           "DeleteSAMLIdPServiceProvider",
		ID:                     "samlIdPServiceProvider.Metadata.Name",
		Kind:                   "saml_idp_service_provider",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_saml_idp_service_provider",
		HasCheckAndSetDefaults: true,
		// TODO: The Teleport SAML IdP API mutates the generated
		// `spec.entity_descriptor` based on `spec.attribute_mapping`. This can
		// result in `inconsistent state after apply` errors.
		SaveSpecStateFromPlan: true,
		WithoutModifyPlan:     true,
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

	discoveryConfig = payload{
		Name:                  "DiscoveryConfig",
		TypeName:              "DiscoveryConfig",
		VarName:               "discoveryConfig",
		GetMethod:             "DiscoveryConfigClient().GetDiscoveryConfig",
		CreateMethod:          "DiscoveryConfigClient().CreateDiscoveryConfig",
		UpsertMethodArity:     2,
		UpdateMethod:          "DiscoveryConfigClient().UpsertDiscoveryConfig",
		DeleteMethod:          "DiscoveryConfigClient().DeleteDiscoveryConfig",
		ID:                    "discoveryConfig.Header.Metadata.Name",
		Kind:                  "discovery_config",
		HasStaticID:           false,
		ProtoPackage:          "discoveryconfigv1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/discoveryconfig/v1",
		TerraformResourceType: "teleport_discovery_config",
		ExtraImports:          []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind:          "apitypes.KindDiscoveryConfig",
		ConvertPackagePath:    "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1",
	}

	vnetConfig = payload{
		Name:                  "VnetConfig",
		TypeName:              "VnetConfig",
		VarName:               "vnetConfig",
		GetMethod:             "VnetConfigClient().GetVnetConfig",
		CreateMethod:          "VnetConfigClient().UpsertVnetConfig",
		UpsertMethodArity:     2,
		UpdateMethod:          "VnetConfigClient().UpsertVnetConfig",
		DeleteMethod:          "VnetConfigClient().ResetVnetConfig",
		ID:                    "apitypes.MetaNameVnetConfig",
		Kind:                  "vnet_config",
		HasStaticID:           false,
		ProtoPackage:          "vnet",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/vnet/v1",
		TerraformResourceType: "teleport_vnet_config",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		DefaultName:  "apitypes.MetaNameVnetConfig",
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindVnetConfig",
	}

	integration = payload{
		Name:                   "Integration",
		VarName:                "integration",
		TypeName:               "IntegrationV1",
		IfaceName:              "Integration",
		GetMethod:              "GetIntegration",
		CreateMethod:           "CreateIntegration",
		UpdateMethod:           "UpdateIntegration",
		UpsertMethodArity:      2,
		DeleteMethod:           "DeleteIntegration",
		ID:                     "integration.Metadata.Name",
		Kind:                   "integration",
		HasStaticID:            false,
		TerraformResourceType:  "teleport_integration",
		HasCheckAndSetDefaults: true,
	}

	inferenceModel = payload{
		Name:                  "InferenceModel",
		VarName:               "inferenceModel",
		TypeName:              "InferenceModel",
		GetMethod:             "SummarizerClient().GetInferenceModel",
		CreateMethod:          "SummarizerClient().CreateInferenceModel",
		UpdateMethod:          "SummarizerClient().UpsertInferenceModel",
		UpsertMethodArity:     2,
		DeleteMethod:          "SummarizerClient().DeleteInferenceModel",
		ID:                    "inferenceModel.Metadata.Name",
		Kind:                  "inference_model",
		HasStaticID:           false,
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1",
		ProtoPackage:          "summarizerv1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/summarizer/v1",
		SchemaPackage:         "schemav1",
		TerraformResourceType: "teleport_inference_model",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindInferenceModel",
	}

	inferenceSecret = payload{
		Name:                  "InferenceSecret",
		VarName:               "inferenceSecret",
		TypeName:              "InferenceSecret",
		GetMethod:             "SummarizerClient().GetInferenceSecret",
		CreateMethod:          "SummarizerClient().CreateInferenceSecret",
		UpdateMethod:          "SummarizerClient().UpsertInferenceSecret",
		UpsertMethodArity:     2,
		DeleteMethod:          "SummarizerClient().DeleteInferenceSecret",
		ID:                    "inferenceSecret.Metadata.Name",
		Kind:                  "inference_secret",
		HasStaticID:           false,
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1",
		ProtoPackage:          "summarizerv1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/summarizer/v1",
		SchemaPackage:         "schemav1",
		TerraformResourceType: "teleport_inference_secret",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		// ExtraImports: []string{
		// 	"apitypes \"github.com/gravitational/teleport/api/types\"",
		// 	"\"github.com/hashicorp/terraform-plugin-framework/attr\"",
		// },
		ForceSetKind: "apitypes.KindInferenceSecret",
		// This resource's spec can't be fetched from the server, so its spec state
		// is save from the plan. It also means we can't support importing the
		// state from an existing configuration.
		SaveSpecStateFromPlan: true,
		WithoutImportState:    true,
	}

	retrievalModel = payload{
		Name:                  "RetrievalModel",
		VarName:               "retrievalModel",
		TypeName:              "RetrievalModel",
		GetMethod:             "SummarizerClient().GetRetrievalModel",
		CreateMethod:          "SummarizerClient().CreateRetrievalModel",
		UpdateMethod:          "SummarizerClient().UpsertRetrievalModel",
		UpsertMethodArity:     2,
		DeleteMethod:          "SummarizerClient().DeleteRetrievalModel",
		ID:                    "apitypes.MetaNameRetrievalModel",
		DefaultName:           "apitypes.MetaNameRetrievalModel",
		Kind:                  "retrieval_model",
		HasStaticID:           false,
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1",
		ProtoPackage:          "summarizerv1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/summarizer/v1",
		SchemaPackage:         "schemav1",
		TerraformResourceType: "teleport_retrieval_model",
		IsPlainStruct:         true,
		ExtraImports:          []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind:          "apitypes.KindRetrievalModel",
	}

	inferencePolicy = payload{
		Name:                  "InferencePolicy",
		VarName:               "inferencePolicy",
		TypeName:              "InferencePolicy",
		GetMethod:             "SummarizerClient().GetInferencePolicy",
		CreateMethod:          "SummarizerClient().CreateInferencePolicy",
		UpdateMethod:          "SummarizerClient().UpsertInferencePolicy",
		UpsertMethodArity:     2,
		DeleteMethod:          "SummarizerClient().DeleteInferencePolicy",
		ID:                    "inferencePolicy.Metadata.Name",
		Kind:                  "inference_policy",
		HasStaticID:           false,
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1",
		ProtoPackage:          "summarizerv1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/summarizer/v1",
		SchemaPackage:         "schemav1",
		TerraformResourceType: "teleport_inference_policy",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindInferencePolicy",
	}

	workloadCluster = payload{
		Name:                  "WorkloadCluster",
		TypeName:              "WorkloadCluster",
		VarName:               "workloadcluster",
		GetMethod:             "GetWorkloadCluster",
		CreateMethod:          "CreateWorkloadCluster",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertWorkloadCluster",
		DeleteMethod:          "DeleteWorkloadCluster",
		ID:                    "workloadcluster.Metadata.Name",
		Kind:                  "workload_cluster",
		HasStaticID:           false,
		ProtoPackage:          "workloadclusterv1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/workloadcluster/v1",
		TerraformResourceType: "teleport_workload_cluster",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindWorkloadCluster",
		StatePoll: &statePoll{
			StatePath: []string{
				"Status",
				"State",
			},
			PendingStates:            []string{"creating"},
			TargetStates:             []string{"active"},
			StatePollIntervalSeconds: 30,
			StateTimeoutSeconds:      15 * 60,
		},
	}
	clientIPRestriction = payload{
		Name:     "ClientIPRestriction",
		TypeName: "ClientIPRestriction",
		VarName:  "clientIPRestriction",
		// ClientIPRestriction is a singleton: the [client.Client] helpers resolve
		// the name server-side, so the provider never has to pass it.
		GetMethod:             "GetClientIPRestriction",
		CreateMethod:          "CreateClientIPRestriction",
		UpsertMethodArity:     2,
		UpdateMethod:          "UpsertClientIPRestriction",
		DeleteMethod:          "DeleteClientIPRestriction",
		ID:                    "clientIPRestriction.Metadata.Name",
		Kind:                  "client_ip_restriction",
		HasStaticID:           false,
		ProtoPackage:          "clientiprestrictionv1",
		ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1",
		SchemaPackage:         "schemav1",
		SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/clientiprestriction/v1",
		TerraformResourceType: "teleport_client_ip_restriction",
		// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
		// resources are plain structs
		IsPlainStruct: true,
		// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
		// We import the package containing kinds, then use ForceSetKind.
		ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
		ForceSetKind: "apitypes.KindClientIPRestriction",
		DefaultName:  "apitypes.MetaNameClientIPRestriction",
	}
	/*
		//
		// Example payload, copy this and replace every "example", "v1", and "TypeA" reference with your resource.
		//
			typeA = payload{
				Name:                  "TypeA",
				TypeName:              "TypeA",
				VarName:               "typeA",
				GetMethod:             "GetTypeA",
				CreateMethod:          "CreateTypeA",
				UpsertMethodArity:     2,
				UpdateMethod:          "UpsertTypeA",
				DeleteMethod:          "DeleteTypeA",
				ID:                    "typeA.Metadata.Name",
				Kind:                  "type_a",
				HasStaticID:           false,
				ProtoPackage:          "examplev1",
				ProtoPackagePath:      "github.com/gravitational/teleport/api/gen/proto/go/teleport/example/v1",
				SchemaPackage:         "schemav1",
				SchemaPackagePath:     "github.com/gravitational/teleport/integrations/terraform/tfschema/example/v1",
				TerraformResourceType: "teleport_type_a",
				// Since [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
				// resources are plain structs
				IsPlainStruct: true,
				// As 153-style resources don't have CheckAndSetDefaults, we must set the Kind manually.
				// We import the package containing kinds, then use ForceSetKind.
				ExtraImports: []string{"apitypes \"github.com/gravitational/teleport/api/types\""},
				ForceSetKind: "apitypes.KindTypeA",
				// Only set default name if the resource is a singleton
				// DefaultName:  "apitypes.MetaNameTypeA",
			}
	*/
)

func main() {
	genTFSchema()
}

func genTFSchema() {
	generateResource(clusterMaintenance, singularResource)
	generateDataSource(clusterMaintenance, singularDataSource)
	generateResource(githubConnector, pluralResource)
	generateDataSource(githubConnector, pluralDataSource)
	generateResource(oidcConnector, pluralResource)
	generateDataSource(oidcConnector, pluralDataSource)
	generateResource(samlIdPServiceProvider, pluralResource)
	generateDataSource(samlIdPServiceProvider, pluralDataSource)
	generateResource(provisionToken, pluralResource)
	generateDataSource(provisionToken, pluralDataSource)
	generateResource(autoUpdateVersion, singularResource)
	generateDataSource(autoUpdateVersion, singularDataSource)
	generateResource(autoUpdateConfig, singularResource)
	generateDataSource(autoUpdateConfig, singularDataSource)
	generateResource(discoveryConfig, pluralResource)
	generateDataSource(discoveryConfig, pluralDataSource)
	generateResource(vnetConfig, singularResource)
	generateDataSource(vnetConfig, singularDataSource)
	generateResource(integration, pluralResource)
	generateDataSource(integration, pluralDataSource)
	generateResource(inferenceModel, pluralResource)
	generateDataSource(inferenceModel, pluralDataSource)
	generateResource(inferenceSecret, pluralResource)
	generateDataSource(inferenceSecret, pluralDataSource)
	generateResource(inferencePolicy, pluralResource)
	generateDataSource(inferencePolicy, pluralDataSource)
	generateResource(retrievalModel, singularResource)
	generateDataSource(retrievalModel, singularDataSource)
	generateResource(workloadCluster, pluralResource)
	generateDataSource(workloadCluster, pluralDataSource)
	generateResource(clientIPRestriction, singularResource)
	generateDataSource(clientIPRestriction, singularDataSource)
	// Add resources here, use the singular resource for singletons and the plural resource for regular resources.
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
