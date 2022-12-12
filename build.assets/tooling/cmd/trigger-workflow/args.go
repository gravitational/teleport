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
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// inputMap represents the input values to a workflow. The odd `interface{}`
// value type (rather than the more obvious `string`) is so that we match the
// type that the GitHub API expects, and we can plug an `inputMap` straight
// in without any further conversion.
type inputMap map[string]interface{}

func (m inputMap) String() string {
	text := strings.Builder{}
	for k, v := range m {
		fmt.Fprintf(&text, "%s=%v, ", k, v)
	}
	return text.String()
}

func (m inputMap) Set(s string) error {
	parts := strings.SplitN(s, "=", 2)

	key := parts[0]
	if key == "" {
		return trace.BadParameter("missing input name")
	}

	value := ""
	if len(parts) > 1 {
		value = parts[1]
	}

	m[key] = value
	return nil
}

// args holds the parsed command-line arguments for the command.
type args struct {
	token       string
	owner       string
	repo        string
	workflow    string
	workflowRef string
	timeout     time.Duration
	inputs      inputMap
}

func parseCommandLine() args {

	args := args{
		workflowRef: "main",
		inputs:      make(inputMap),
	}

	flag.StringVar(&args.token, "token", "", "GitHub PAT")
	flag.StringVar(&args.owner, "owner", "", "Owner of the repo to target")
	flag.StringVar(&args.repo, "repo", "", "Repo to target")
	flag.StringVar(&args.workflow, "workflow", "", "Path to workflow")
	flag.StringVar(&args.workflowRef, "workflow-ref", args.workflowRef, "Revision reference")
	flag.DurationVar(&args.timeout, "timeout", 30*time.Minute, "Timeout")
	flag.Var(args.inputs, "input", "Input to target workflow")

	flag.Parse()

	return args
}
