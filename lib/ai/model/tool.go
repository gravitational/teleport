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

package model

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
)

type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, input string) (string, error)
}

type commandExecutionTool struct{}

type commandExecutionToolInput struct {
	Command string   `json:"command"`
	Nodes   []string `json:"nodes"`
	Labels  []Label  `json:"labels"`
}

func (c *commandExecutionTool) Name() string {
	return "Command Execution"
}

func (c *commandExecutionTool) Description() string {
	return fmt.Sprintf(`Execute a command on a set of remote hosts based on a set of hostnames or/and a set of labels.
The input must be a JSON object with the following schema:

%vjson
{
	"command": string, \\ The command to execute
	"nodes": []string, \\ Execute a command on all nodes that have the given hostnames
	"labels": []{"key": string, "value": string} \\ Execute a command on all nodes that has at least one of the labels
}
%v
`, "```", "```")
}

func (c *commandExecutionTool) Run(ctx context.Context, input string) (string, error) {
	return "", trace.NotImplemented("not implemented")
}

func (*commandExecutionTool) parseInput(input string) (*commandExecutionToolInput, *invalidOutputError) {
	output, err := parseJSONFromModel[commandExecutionToolInput](input)
	if err != nil {
		return nil, err
	}

	if output.Command == "" {
		return nil, &invalidOutputError{
			coarse: "command execution: missing command",
			detail: "command must be non-empty",
		}
	}

	if len(output.Nodes) == 0 && len(output.Labels) == 0 {
		return nil, &invalidOutputError{
			coarse: "command execution: missing nodes or labels",
			detail: "at least one node or label must be specified",
		}
	}

	return &output, nil
}
