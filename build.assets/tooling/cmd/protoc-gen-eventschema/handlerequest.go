// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/build.assets/tooling/lib/eventschema"
	tree "github.com/gravitational/teleport/build.assets/tooling/lib/protobuf-tree"
)

const outputFileName = "zz_generated.eventschema.go"

func handleRequest(req *gogoplugin.CodeGeneratorRequest) error {
	switch inputFileCount := len(req.FileToGenerate); {
	case inputFileCount == 0:
		return trace.BadParameter("no input file provided")
	case inputFileCount > 1:
		return trace.BadParameter("too many input files")
	}

	gen, err := newGenerator(req)
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

func newGenerator(req *gogoplugin.CodeGeneratorRequest) (*tree.Forest, error) {
	gen := generator.New()

	gen.Request = req
	gen.CommandLineParameters(gen.Request.GetParameter())
	gen.WrapTypes()
	gen.SetPackageNames()
	gen.BuildTypeNameMap()

	return tree.NewForest(gen), nil
}

func generateSchema(file *tree.File, resp *gogoplugin.CodeGeneratorResponse) error {
	gen := eventschema.NewSchemaGenerator()

	err := gen.Process(file)
	if err != nil {
		return trace.Wrap(err)
	}

	name := outputFileName
	content, err := gen.Render()
	if err != nil {
		return trace.Wrap(err)
	}
	resp.File = append(resp.File, &gogoplugin.CodeGeneratorResponse_File{Name: &name, Content: &content})

	return nil
}
