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

package lib

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
	log "github.com/sirupsen/logrus"
)

const pluginInputPathEnvironment = "TELEPORT_PROTOC_READ_FILE"

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)
	inputPath := os.Getenv(pluginInputPathEnvironment)
	if inputPath == "" {
		log.Error(trace.BadParameter("When built with the 'debug' tag, the input path must be set through the environment variable: %s", pluginInputPathEnvironment))
		os.Exit(-1)
	}
	log.Infof("This is a debug build, the protoc request is read from the file: '%s'", inputPath)

	req, err := readRequestFromFile(inputPath)
	if err != nil {
		log.WithError(err).Error("error reading request from file")
		os.Exit(-1)
	}
	if err := handleRequest(req); err != nil {
		log.WithError(err).Error("Failed to generate schema")
		os.Exit(-1)
	}
}

func readRequestFromFile(inputPath string) (*plugin.CodeGeneratorRequest, error) {
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
