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

	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/schemagen"
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

	gen, err := schemagen.NewGenerator(req)
	if err != nil {
		return trace.Wrap(err)
	}

	rootFileName := req.FileToGenerate[0]
	gen.SetFile(rootFileName)
	for _, fileDesc := range gen.AllFiles().File {
		file := gen.AddFile(fileDesc)
		if fileDesc.GetName() == rootFileName {
			if err := generateSchema(file, "resources.teleport.dev", gen.Response); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	command.Write(gen.Response)

	return nil
}

func generateSchema(file *schemagen.File, groupName string, resp *gogoplugin.CodeGeneratorResponse) error {
	generator := schemagen.NewSchemaGenerator(groupName)

	if err := generator.ParseResource(file, "UserV2"); err != nil {
		return trace.Wrap(err)
	}

	// Use RoleV6 spec but override the version to V5.
	// This will generate crd based on RoleV6 but with resource version for v5.
	if err := generator.ParseResource(file, "RoleV6", types.V5); err != nil {
		return trace.Wrap(err)
	}

	if err := generator.ParseResource(file, "RoleV6"); err != nil {
		return trace.Wrap(err)
	}

	if err := generator.ParseResource(file, "SAMLConnectorV2"); err != nil {
		return trace.Wrap(err)
	}
	if err := generator.ParseResource(file, "OIDCConnectorV3"); err != nil {
		return trace.Wrap(err)
	}
	if err := generator.ParseResource(file, "GithubConnectorV3"); err != nil {
		return trace.Wrap(err)
	}

	for _, root := range generator.Roots {
		crd := CustomResourceDefinition(root)
		data, err := yaml.Marshal(crd)
		if err != nil {
			return trace.Wrap(err)
		}
		name := fmt.Sprintf("%s_%s.yaml", groupName, root.PluralName)
		content := string(data)
		resp.File = append(resp.File, &gogoplugin.CodeGeneratorResponse_File{Name: &name, Content: &content})
	}

	return nil
}
