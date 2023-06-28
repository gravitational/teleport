/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"

	gogodesc "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/api/types"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)
	req := command.Read()
	if err := handleRequest(req); err != nil {
		log.WithError(err).Error("Failed to generate schema")
		os.Exit(-1)
	}
}

func handleRequest(req *gogoplugin.CodeGeneratorRequest) error {
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
			if err := generateSchema(file, "resources.teleport.dev", gen.Response); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	command.Write(gen.Response)

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

func generateSchema(file *File, groupName string, resp *gogoplugin.CodeGeneratorResponse) error {
	generator := NewSchemaGenerator(groupName)

	resources := []resource{
		{name: "UserV2"},
		{name: "RoleV6", opts: []resourceSchemaOption{withVersionOverride(types.V5)}},
		{name: "RoleV6"},
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
		{name: "ProvisionTokenV2"},
		{name: "OktaImportRuleV1"},
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
		crd := root.CustomResourceDefinition()
		data, err := yaml.Marshal(crd)
		if err != nil {
			return trace.Wrap(err)
		}
		name := fmt.Sprintf("%s_%s.yaml", groupName, root.pluralName)
		content := string(data)
		resp.File = append(resp.File, &gogoplugin.CodeGeneratorResponse_File{Name: &name, Content: &content})
	}

	return nil
}
