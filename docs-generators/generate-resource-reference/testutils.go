package main

import (
	"io"
	"io/ioutil"

	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"go.starlark.net/lib/proto"
)

// readProto reads from r and unmarshals it into req, returning any errors
// encountered during the process.
func readProto(req *gogoplugin.CodeGeneratorRequest, r io.Reader) error {
	g := generator.New()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil
	}

	if err := proto.Unmarshal(data, req); err != nil {
		g.Error(err, "parsing input proto")
	}

	if len(g.Request.FileToGenerate) == 0 {
		g.Fail("no files to generate")
	}
	return nil
}
