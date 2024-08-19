//go:build debug

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

// This is an alternative main package that gets included when the `debug` tag
// is set. When built with this debug tag, the protoc plugin reads its input
// from a file instead of stdin. This allows to easily attach a debugger and
// inspect what is happening inside the plugin.

import (
	"io"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	plugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gravitational/trace"
)

// PluginInputPathEnvironment is the environment variable telling debug builds where the protoc request file is located.
const PluginInputPathEnvironment = "TELEPORT_PROTOC_READ_FILE"

// ReadRequestFromFile reads the protoc request from a file instead of stdin.
// This is used for debugging purposes (this allows to invoke the protoc plugin directly
// with a debugger attached).
func ReadRequestFromFile(inputPath string) (*plugin.CodeGeneratorRequest, error) {
	g := generator.New()
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := io.ReadAll(inputFile)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to read input")
	}

	if err := proto.Unmarshal(data, g.Request); err != nil {
		return nil, trace.WrapWithMessage(err, "failed to parse input proto")
	}

	if len(g.Request.FileToGenerate) == 0 {
		return nil, trace.BadParameter("no files to generate")
	}
	return g.Request, nil
}
