/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package lib

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	gogodesc "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/api/types"
)

func HandleCRDRequest(req *gogoplugin.CodeGeneratorRequest) error {
	return handleRequest(req, formatAsYAML)
}

func HandleDocsRequest(req *gogoplugin.CodeGeneratorRequest) error {
	return handleRequest(req, formatAsDocsPage)
}

func handleRequest(req *gogoplugin.CodeGeneratorRequest, out crdFormatFunc) error {
	if len(req.FileToGenerate) == 0 {
		return trace.Errorf("no input file provided")
	}
	if len(req.FileToGenerate) > 1 {
		return trace.Errorf("too many input files")
	}

	gen, err := newGenerator(req)
	if err != nil {
		return trace.Wrap(err)
	}

	rootFileName := req.FileToGenerate[0]
	gen.SetFile(rootFileName)
	for _, fileDesc := range gen.AllFiles().File {
		file := gen.addFile(fileDesc)
		if fileDesc.GetName() == rootFileName {
			if err := generateSchema(
				file,
				"resources.teleport.dev",
				out,
				gen.Response,
			); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Convert the gogo response to a regular protobuf response. This allows us
	// to pack in the SupportedFeatures field, which indicates that the optional
	// field is supported.
	response := &pluginpb.CodeGeneratorResponse{}
	response.Error = gen.Response.Error
	response.File = make([]*pluginpb.CodeGeneratorResponse_File, 0, len(gen.Response.File))
	for _, file := range gen.Response.File {
		response.File = append(response.File, &pluginpb.CodeGeneratorResponse_File{
			Name:           file.Name,
			InsertionPoint: file.InsertionPoint,
			Content:        file.Content,
		})
	}
	features := uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
	response.SupportedFeatures = &features

	// Send back the results. The code below was taken from the vanity command,
	// but it now uses the regular response instead of the gogo specific one.
	data, err := proto.Marshal(response)
	if err != nil {
		return trace.Wrap(err, "failed to marshal output proto")
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		return trace.Wrap(err, "failed to write output proto")
	}

	return nil
}

func newGenerator(req *gogoplugin.CodeGeneratorRequest) (*Forest, error) {
	gen := generator.New()

	gen.Request = req
	gen.CommandLineParameters(gen.Request.GetParameter())
	gen.WrapTypes()
	gen.SetPackageNames()
	gen.BuildTypeNameMap()

	return &Forest{
		Generator:  gen,
		messageMap: make(map[*gogodesc.DescriptorProto]*Message),
	}, nil
}

type resource struct {
	name string
	opts []resourceSchemaOption
}

var userColumns = []apiextv1.CustomResourceColumnDefinition{
	{
		Name:        "Roles",
		Type:        "string",
		Description: "List of Teleport roles granted to the user.",
		Priority:    0,
		JSONPath:    ".spec.roles",
	},
}

var serverColumns = []apiextv1.CustomResourceColumnDefinition{
	{
		Name:        "Hostname",
		Type:        "string",
		Description: "Server hostname",
		Priority:    0,
		JSONPath:    ".spec.hostname",
	},
	{
		Name:        "Address",
		Type:        "string",
		Description: "Server address, with SSH port.",
		Priority:    0,
		JSONPath:    ".spec.addr",
	},
}

var tokenColumns = []apiextv1.CustomResourceColumnDefinition{
	{
		Name:        "Join Method",
		Type:        "string",
		Description: "Token join method.",
		Priority:    0,
		JSONPath:    ".spec.join_method",
	},
	{
		Name:        "System Roles",
		Type:        "string",
		Description: "System roles granted by this token.",
		Priority:    0,
		JSONPath:    ".spec.roles",
	},
}

// crdFormatFunc formats the given CRD into a document. It returns the document
// as a byte slice, plus the file extension for the document.
type crdFormatFunc func(apiextv1.CustomResourceDefinition) ([]byte, string, error)

func formatAsYAML(crd apiextv1.CustomResourceDefinition) ([]byte, string, error) {
	doc, err := yaml.Marshal(crd)
	if err != nil {
		return nil, "", err
	}
	return doc, "yaml", nil
}

var crdDocTmpl string = `---
title: {{.Title}}
description: {{.Description}}
tocDepth: 3
---

{/*Auto-generated file. Do not edit.*/}
{/*To regenerate, navigate to integrations/operator and run "make crd-docs".*/}

{{.Intro}}

{{ range .Sections}}
## {{.APIVersion}}

**apiVersion:** {{.APIVersion}}

{{- range .Subsections }}
{{- if ne .Name "" }}
### {{.Name}}
{{- end }}

|Field|Type|Description|
|---|---|---|
{{- range .Fields }}
|{{.Name}}|{{.Type}}|{{.Description}}|
{{- end }}
{{ end }}

{{- end}}
`

type ResourcePage struct {
	Title       string
	Description string
	Intro       string
	Sections    []VersionSection
}

func (r ResourcePage) Len() int {
	return len(r.Sections)
}

func (r ResourcePage) Swap(i, j int) {
	r.Sections[i], r.Sections[j] = r.Sections[j], r.Sections[i]
}

func (r ResourcePage) Less(i, j int) bool {
	return strings.Compare(r.Sections[i].APIVersion, r.Sections[j].APIVersion) == -1
}

func (v VersionSection) Len() int {
	return len(v.Subsections)
}

func (v VersionSection) Swap(i, j int) {
	v.Subsections[i], v.Subsections[j] = v.Subsections[j], v.Subsections[i]
}

func (v VersionSection) Less(i, j int) bool {
	return strings.Compare(v.Subsections[i].Name, v.Subsections[j].Name) == -1
}

type VersionSection struct {
	APIVersion  string
	Subsections []PropertyTable
}

type PropertyTable struct {
	Name   string
	Fields []PropertyTableField
}

type PropertyTableField struct {
	Name        string
	Type        string
	Description string
}

func (t PropertyTable) Len() int {
	return len(t.Fields)
}

func (t PropertyTable) Swap(i, j int) {
	t.Fields[i], t.Fields[j] = t.Fields[j], t.Fields[i]
}

func (t PropertyTable) Less(i, j int) bool {
	return strings.Compare(t.Fields[i].Name, t.Fields[j].Name) == -1
}

// Skipping the "status" field, since it is not available for users to
// configure.
const statusDescription = "Status defines the observed state of the Teleport resource"
const statusName = "status"

func propertyTable(currentFieldName string, props *apiextv1.JSONSchemaProps) ([]PropertyTable, error) {
	switch props.Type {
	case "object":
		tab := PropertyTable{
			Name: currentFieldName,
		}
		fields := []PropertyTableField{}
		tables := []PropertyTable{}
		var i int
		for k, v := range props.Properties {
			// Don't document the Status field, which is for
			// internal use.
			if k == statusName && strings.HasPrefix(v.Description, statusDescription) {
				continue
			}
			// Name the table after the hierarchy of
			// field names to avoid duplication.
			var tableName string
			if currentFieldName != "" {
				tableName = currentFieldName + "." + k
			} else {
				tableName = k
			}

			var fieldType string
			var fieldDesc string
			switch v.Type {
			case "object":
				fieldType = "object"
				if v.Properties == nil || len(v.Properties) == 0 {
					break
				}

				extra, err := propertyTable(
					tableName,
					&v,
				)
				if err != nil {
					return nil, err
				}
				fieldType = fmt.Sprintf("[object](#%v)", strings.ReplaceAll(tableName, ".", ""))
				tables = append(tables, extra...)
			case "array":
				var subtp string
				if v.Items.Schema.Type == "object" {
					extra, err := propertyTable(
						fmt.Sprintf("%v items", tableName),
						v.Items.Schema,
					)
					if err != nil {
						return nil, err
					}
					tables = append(tables, extra...)
					subtp = fmt.Sprintf("[object](#%v-items)", strings.ReplaceAll(tableName, ".", ""))
				} else {
					subtp = v.Items.Schema.Type
				}
				fieldType = fmt.Sprintf("[]%v", subtp)
			case "":
				if !v.XIntOrString {
					fieldType = v.Type
					break
				}
				fieldType = "string or integer"
				fieldDesc = strings.TrimSuffix(v.Description, ".") + ". " + "Can be either the string or the integer representation of each option."
			default:
				fieldType = v.Type
			}

			if fieldDesc == "" {
				fieldDesc = v.Description
			}

			fields = append(fields, PropertyTableField{
				Name:        k,
				Type:        fieldType,
				Description: fieldDesc,
			})
			i++
		}
		tab.Fields = fields
		sort.Sort(tab)
		tables = append([]PropertyTable{tab}, tables...)
		return tables, nil
	}
	return nil, nil
}

func formatAsDocsPage(crd apiextv1.CustomResourceDefinition) ([]byte, string, error) {
	var buf bytes.Buffer
	rp := ResourcePage{
		Title:       crd.Spec.Names.Kind,
		Description: fmt.Sprintf("Provides a comprehensive list of fields in the %v resource available through the Teleport Kubernetes operator", crd.Spec.Names.Kind),
		Intro: strings.ReplaceAll(fmt.Sprintf(
			`This guide is a comprehensive reference to the fields in the BACKTICK%vBACKTICK
resource, which you can apply after installing the Teleport Kubernetes operator.`,
			crd.Spec.Names.Kind), "BACKTICK", "`"),
	}

	vs := make([]VersionSection, len(crd.Spec.Versions))
	for i, v := range crd.Spec.Versions {
		props, err := propertyTable("", v.Schema.OpenAPIV3Schema)
		if err != nil {
			return nil, "", err
		}
		n := VersionSection{
			APIVersion:  fmt.Sprintf("%v/%v", crd.Spec.Group, v.Name),
			Subsections: props,
		}
		sort.Sort(n)
		vs[i] = n
	}
	rp.Sections = vs

	templ := template.New("docs")
	templ, err := templ.Parse(crdDocTmpl)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	err = templ.Execute(&buf, rp)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return buf.Bytes(), "mdx", nil
}

func generateSchema(file *File, groupName string, format crdFormatFunc, resp *gogoplugin.CodeGeneratorResponse) error {
	generator := NewSchemaGenerator(groupName)

	resources := []resource{
		{name: "UserV2", opts: []resourceSchemaOption{withAdditionalColumns(userColumns)}},
		// Role V5 is using the RoleV6 message
		{name: "RoleV6", opts: []resourceSchemaOption{withVersionOverride(types.V5)}},
		// For backward compatibility in v15, it actually creates v5 roles though.
		{name: "RoleV6"},
		// Role V6 and V7 have their own Kubernetes kind
		{name: "RoleV6", opts: []resourceSchemaOption{withVersionInKindOverride()}},
		// Role V7 is using the RoleV6 message
		{name: "RoleV6", opts: []resourceSchemaOption{withVersionOverride(types.V7), withVersionInKindOverride()}},
		{name: "SAMLConnectorV2"},
		{name: "OIDCConnectorV3"},
		{name: "GithubConnectorV3"},
		{
			name: "LoginRule",
			opts: []resourceSchemaOption{
				// Overriding the version because it is not in the type name.
				withVersionOverride(types.V1),
				// The LoginRule proto does not have a "spec" field, so force
				// the CRD spec to include these fields from the root.
				withCustomSpecFields([]string{"priority", "traits_expression", "traits_map"}),
			},
		},
		{name: "ProvisionTokenV2", opts: []resourceSchemaOption{withAdditionalColumns(tokenColumns)}},
		{name: "OktaImportRuleV1"},
		{
			name: "AccessList",
			opts: []resourceSchemaOption{
				withVersionOverride(types.V1),
			},
		},
		{
			name: "ServerV2",
			opts: []resourceSchemaOption{
				withVersionInKindOverride(),
				withNameOverride("OpenSSHServer"),
				withAdditionalColumns(serverColumns),
			},
		},
		{
			name: "ServerV2",
			opts: []resourceSchemaOption{
				withVersionInKindOverride(),
				withNameOverride("OpenSSHEICEServer"),
				withAdditionalColumns(serverColumns),
			},
		},
	}

	for _, resource := range resources {
		_, ok := file.messageByName[resource.name]
		if !ok {
			continue
		}
		err := generator.addResource(file, resource.name, resource.opts...)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	for _, root := range generator.roots {
		crd, err := root.CustomResourceDefinition()
		if err != nil {
			return trace.Wrap(err, "generating CRD")
		}
		data, ext, err := format(crd)
		if err != nil {
			return trace.Wrap(err)
		}
		name := fmt.Sprintf("%s_%s.%v", groupName, root.pluralName, ext)
		content := string(data)
		resp.File = append(resp.File, &gogoplugin.CodeGeneratorResponse_File{Name: &name, Content: &content})
	}

	return nil
}
