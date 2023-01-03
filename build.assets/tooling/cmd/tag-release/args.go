// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"path/filepath"

	"github.com/gravitational/trace"
)

// args holds the parsed command-line arguments for the command.
type args struct {
	tag        string
	annotation string
	repoDir    string
	remote     string
	push       bool
	recursive  bool
	force      bool
}

func parseCommandLine() (args, error) {
	cliArgs := args{}

	flag.StringVar(&cliArgs.repoDir, "repo", ".", "Path to repo")
	flag.StringVar(&cliArgs.remote, "remote", "origin", "The remote to push to")
	flag.StringVar(&cliArgs.tag, "tag", "", "Revision reference")
	flag.StringVar(&cliArgs.annotation, "note", "", "Annotation to add to the tag")
	flag.BoolVar(&cliArgs.push, "push", false, "Push repositories once tagging is complete")
	flag.BoolVar(&cliArgs.recursive, "recursive", false, "Recursively tag all submodules")
	flag.BoolVar(&cliArgs.force, "force", false, "Keep going, even if some tag/push operations fail")

	flag.Parse()

	var err error
	cliArgs.repoDir, err = filepath.Abs(cliArgs.repoDir)
	if err != nil {
		return args{}, trace.Wrap(err, "Failed expanding repo path")
	}

	return cliArgs, nil
}
