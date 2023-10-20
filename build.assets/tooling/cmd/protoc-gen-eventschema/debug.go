//go:build debug

/*
Copyright 2023 Gravitational, Inc.
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

// When built with this debug tag, the protoc plugin reads its input
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

func readRequest() (*plugin.CodeGeneratorRequest, error) {
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
	return req, trace.Wrap(err)
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
