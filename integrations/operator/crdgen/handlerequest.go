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

package crdgen

import (
	"os"

	gogodesc "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/gravitational/teleport/api/types"
)

func HandleCRDRequest(req *gogoplugin.CodeGeneratorRequest) error {
	return handleRequest(req, formatAsCRD)
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
		// Role V7 and V8 is using the RoleV6 message
		{name: "RoleV6", opts: []resourceSchemaOption{withVersionOverride(types.V7), withVersionInKindOverride()}},
		{name: "RoleV6", opts: []resourceSchemaOption{withVersionOverride(types.V8), withVersionInKindOverride()}},
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
		{name: "TrustedClusterV2", opts: []resourceSchemaOption{withVersionInKindOverride()}},
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
		data, filename, err := format(crd, groupName, root.pluralName)
		if err != nil {
			return trace.Wrap(err)
		}
		content := string(data)
		resp.File = append(resp.File, &gogoplugin.CodeGeneratorResponse_File{Name: &filename, Content: &content})
	}

	return nil
}
