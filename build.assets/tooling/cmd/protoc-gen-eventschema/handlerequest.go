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
