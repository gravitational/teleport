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
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/olekukonko/tablewriter"

	"github.com/gravitational/teleport/integrations/terraform/provider"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
	accesslistSchema "github.com/gravitational/teleport/integrations/terraform/tfschema/accesslist/v1"
	devicetrustSchema "github.com/gravitational/teleport/integrations/terraform/tfschema/devicetrust/v1"
	loginruleSchema "github.com/gravitational/teleport/integrations/terraform/tfschema/loginrule/v1"
	tokenSchema "github.com/gravitational/teleport/integrations/terraform/tfschema/token"
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
	// CreateMethod represents API update method name
	UpdateMethod string
	// DeleteMethod represents API reset method used in singular resources
	DeleteMethod string
	// UpsertMethodArity represents Create/Update method arity, if it's 2, then the call signature would be "_, err :="
	UpsertMethodArity int
	// WithSecrets value for a withSecrets param of Get method (empty means no param used)
	WithSecrets string
	// ID id value on create and import
	ID string
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

	referenceDocsTemplate = "referencedocs.go.tpl"
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
		CreateMethod:           "SetAuthPreference",
		UpdateMethod:           "SetAuthPreference",
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
		CreateMethod:           "SetClusterNetworkingConfig",
		UpdateMethod:           "SetClusterNetworkingConfig",
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
		ID:                     "strconv.FormatInt(provisionToken.Metadata.ID, 10)", // must be a string
		RandomMetadataName:     true,
		Kind:                   "token",
		HasStaticID:            false,
		ExtraImports:           []string{"strconv"},
		SchemaPackage:          "token",
		SchemaPackagePath:      "github.com/gravitational/teleport/integrations/terraform/tfschema/token",
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
		CreateMethod:           "SetSessionRecordingConfig",
		UpdateMethod:           "SetSessionRecordingConfig",
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
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "docs" {
		genReferenceDocs()
	} else {
		genTFSchema()
	}
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

// Create Docs Markdown
var (
	mapResourceSchema = map[string]func(context.Context) (tfsdk.Schema, diag.Diagnostics){
		"access_list":                accesslistSchema.GenSchemaAccessList,
		"app":                        tfschema.GenSchemaAppV3,
		"auth_preference":            tfschema.GenSchemaAuthPreferenceV2,
		"bot":                        provider.GenSchemaBot,
		"cluster_maintenance_config": tfschema.GenSchemaClusterMaintenanceConfigV1,
		"cluster_networking_config":  tfschema.GenSchemaClusterNetworkingConfigV2,
		"database":                   tfschema.GenSchemaDatabaseV3,
		"trusted_device":             devicetrustSchema.GenSchemaDeviceV1,
		"github_connector":           tfschema.GenSchemaGithubConnectorV3,
		"login_rule":                 loginruleSchema.GenSchemaLoginRule,
		"okta_import_rule":           tfschema.GenSchemaOktaImportRuleV1,
		"oidc_connector":             tfschema.GenSchemaOIDCConnectorV3,
		"provision_token":            tokenSchema.GenSchemaProvisionTokenV2,
		"role":                       tfschema.GenSchemaRoleV6,
		"saml_connector":             tfschema.GenSchemaSAMLConnectorV2,
		"session_recording_config":   tfschema.GenSchemaSessionRecordingConfigV2,
		"trusted_cluster":            tfschema.GenSchemaTrustedClusterV2,
		"user":                       tfschema.GenSchemaUserV2,
		"server":                     tfschema.GenSchemaServerV2,
	}

	// hiddenFields are fields that are not outputted to the reference doc.
	// It supports non-top level fields by adding its prefix. Eg: metadata.namespace
	hiddenFields = []string{
		"id",   // read only field
		"kind", // each resource already defines its kind so this is redundant
	}

	// fieldComments is used to define specific descriptions for the given fields.
	// Typical usage is for enums which we don't have comments yet.
	fieldComments = map[string]string{
		"teleport_auth_preference.spec.require_session_mfa": "RequireMFAType is the type of MFA requirement enforced for this cluster: 0:Off, 1:Session, 2:SessionAndHardwareKey, 3:HardwareKeyTouch",
		"teleport_role.spec.options.require_session_mfa":    "RequireMFAType is the type of MFA requirement enforced for this role: 0:Off, 1:Session, 2:SessionAndHardwareKey, 3:HardwareKeyTouch",
	}
)

func genReferenceDocs() {
	sortedNames := make([]string, 0, len(mapResourceSchema))
	for k := range mapResourceSchema {
		sortedNames = append(sortedNames, k)
	}
	sort.Strings(sortedNames)

	t, err := template.ParseFiles(path.Join("gen", referenceDocsTemplate))
	if err != nil {
		log.Fatal(err)
	}

	referenceDocsResource := make([]referenceDocResource, 0, len(mapResourceSchema))
	for _, name := range sortedNames {
		resourceName := "teleport_" + name

		schemaFn := mapResourceSchema[name]
		schema, diags := schemaFn(context.Background())
		if diags.HasError() {
			log.Fatalf("%v", diags)
		}

		fieldDescBuilder := strings.Builder{}
		dumpAttributes(&fieldDescBuilder, 0, resourceName, "", schema.Attributes)

		exampleFileName := fmt.Sprintf("example/%s.tf.example", name)
		exampleBytes, err := os.ReadFile(exampleFileName)
		if err != nil {
			log.Fatalf("error loading %q file: %v", exampleFileName, err)
		}

		referenceDocsResource = append(referenceDocsResource, referenceDocResource{
			Name:       resourceName,
			FieldsDesc: fieldDescBuilder.String(),
			Example:    string(exampleBytes),
		})
	}

	var b bytes.Buffer
	err = t.ExecuteTemplate(&b, referenceDocsTemplate, map[string]any{
		"resourceList": sortedNames,
		"resourcesDoc": referenceDocsResource,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("reference.mdx", b.Bytes(), 0777)
	if err != nil {
		log.Fatal(err)
	}
}

type referenceDocResource struct {
	Name       string
	FieldsDesc string
	Example    string
}

func dumpAttributes(fp io.Writer, level int, resourceName string, prefix string, attrs map[string]tfsdk.Attribute) {
	sortedAttrKeys := make([]string, 0, len(attrs))
	for k := range attrs {
		sortedAttrKeys = append(sortedAttrKeys, k)
	}
	sort.Strings(sortedAttrKeys)

	table := tablewriter.NewWriter(fp)
	table.SetHeader([]string{"Name", "Type", "Required", "Description"})
	table.SetAutoWrapText(false)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetAutoFormatHeaders(false)

	for _, name := range sortedAttrKeys {
		fullFieldPath := resourceName + "." + prefix + name
		attr := attrs[name]

		if slices.Contains(hiddenFields, prefix+name) {
			continue
		}

		description := attr.Description
		if d, found := fieldComments[fullFieldPath]; found {
			description = d
		}
		// Using html.EscapeString also escapes `'`` (into `&#39;`)
		// This generates a lint error when running cspell because the word `doesn't` becomes `doesn&#39;t`
		// and `doesn` isn't a valid word.
		// This lint error happens when running the `Lint (docs)` CI step in the teleport repo.
		// The mdx format supports `'"&` (the other chars that html.EscapeString escapes) without escaping.
		descriptionMDXEscaped := strings.ReplaceAll(strings.ReplaceAll(description, "<", "&lt;"), ">", "&gt;")
		table.Append([]string{name, typ(attr.Type), requiredString(attr.Required), descriptionMDXEscaped})
	}
	table.Render()
	fmt.Fprintln(fp)

	for _, name := range sortedAttrKeys {
		attr := attrs[name]
		if attr.Attributes != nil {
			fmt.Fprintf(fp, "%s %s\n", strings.Repeat("#", 3+level), prefix+name)
			fmt.Fprintln(fp)
			fmt.Fprintln(fp, attr.Description)
			fmt.Fprintln(fp)

			dumpAttributes(fp, level+1, resourceName, prefix+name+".", attr.Attributes.GetAttributes())
		}
	}
}

func typ(typ attr.Type) string {
	if typ == nil {
		return "object"
	}

	switch typ.String() {
	case "types.StringType":
		return "string"
	case "TimeType(2006-01-02T15:04:05Z07:00)":
		return "RFC3339 time"
	case "DurationType":
		return "duration"
	case "types.BoolType":
		return "bool"
	case "types.MapType[types.StringType]":
		return "map of strings"
	case "types.ListType[types.StringType]":
		return "array of strings"
	case "types.MapType[types.ListType[types.StringType]]":
		return "map of string arrays"
	case "types.Int64Type":
		return "number"
	default:
		return typ.String()
	}
}

func requiredString(r bool) string {
	if r {
		return "*"
	}
	return " "
}
