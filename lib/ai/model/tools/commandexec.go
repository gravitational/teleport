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

package tools

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	modeloutput "github.com/gravitational/teleport/lib/ai/model/output"
)

type CommandExecutionTool struct{}

func (c *CommandExecutionTool) Name() string {
	return "Command Execution"
}

func (c *CommandExecutionTool) Description() string {
	return fmt.Sprintf(`Execute a command on a set of remote nodes based on a set of node names or/and a set of labels.
The input must be a JSON object with the following schema:

%vjson
{
	"command": string, \\ The command to execute
	"nodes": []string, \\ Execute a command on all nodes that have the given node names
	"labels": []{"key": string, "value": string} \\ Execute a command on all nodes that has at least one of the labels
}
%v
`, "```", "```")
}

func (c *CommandExecutionTool) Run(_ context.Context, _ *ToolContext, _ string) (string, error) {
	// This is stubbed because CommandExecutionTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a command suggestion to the user.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

// ParseInput is called in a special case if the planned tool is CommandExecutionTool.
// This is because CommandExecutionTool is handled differently from most other tools and forcibly terminates the thought loop.
func (*CommandExecutionTool) ParseInput(input string) (*modeloutput.CompletionCommand, error) {
	output, err := modeloutput.ParseJSONFromModel[modeloutput.CompletionCommand](input)
	if err != nil {
		return nil, err
	}

	if output.Command == "" {
		return nil, modeloutput.NewInvalidOutputError(
			"command execution: missing command",
			"command must be non-empty",
		)
	}

	if len(output.Nodes) == 0 && len(output.Labels) == 0 {
		return nil, modeloutput.NewInvalidOutputError(
			"command execution: missing nodes or labels",
			"at least one node or label must be specified",
		)
	}

	return &output, nil
}
