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
	"os"

	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/schemagen"
)

// resourceConfigs maps the name of a resource type/version to its corresponding
// parsing options.
var resourceConfigs = map[string]schemagen.ParseResourceOptions{
	"UserV2":          {},
	"RoleV6":          {},
	"SAMLConnectorV2": {},
	"OIDCConnectorV3": {},
	"GithubConnectorV3": {
		IgnoredFields: []string{
			"TeamsToLogins", // Deprecated field, removed since v11
		},
	},
}

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
			if err := generateSchema(file, gen.Response); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	command.Write(gen.Response)

	return nil
}

func generateSchema(file *schemagen.File, resp *gogoplugin.CodeGeneratorResponse) error {
	generator := schemagen.NewSchemaGenerator()

	for r, c := range resourceConfigs {
		if err := generator.ParseResource(file, r, c); err != nil {
			return trace.Wrap(err)
		}
	}

	// TODO: Range through generator.roots and do something with the data

	name := "resource-reference.mdx"
	content := ""

	resp.File = []*gogoplugin.CodeGeneratorResponse_File{
		&gogoplugin.CodeGeneratorResponse_File{Name: &name, Content: &content},
	}

	return nil
}
