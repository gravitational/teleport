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

type CommandGenerationTool struct{}

type CommandGenerationToolInput struct {
	// Command is a unix command to execute.
	Command string `json:"command"`
}

func (c *CommandGenerationTool) Name() string {
	return "Command Generation"
}

func (c *CommandGenerationTool) Description() string {
	// acknowledgement field is used to convince the LLM to return the JSON.
	// Base on my testing LLM ignores the JSON when the schema has only one field.
	// Adding additional "pseudo-fields" to the schema makes the LLM return the JSON.
	return fmt.Sprintf(`Generate a Bash command.
The input must be a JSON object with the following schema:
%vjson
{
	"command": string, \\ The generated command
	"acknowledgement": boolean \\ Set to true to ackowledge that you understand the formatting
}
%v
`, "```", "```")
}

func (c *CommandGenerationTool) Run(_ context.Context, toolCtx *ToolContext, _ string) (string, error) {
	// This is stubbed because CommandGenerationTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a command suggestion to the user.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

// ParseInput is called in a special case if the planned tool is CommandExecutionTool.
// This is because CommandExecutionTool is handled differently from most other tools and forcibly terminates the thought loop.
func (*CommandGenerationTool) ParseInput(input string) (*CommandGenerationToolInput, error) {
	output, err := modeloutput.ParseJSONFromModel[CommandGenerationToolInput](input)
	if err != nil {
		return nil, err
	}

	if output.Command == "" {
		return nil, modeloutput.NewInvalidOutputError(
			"command generation: missing command",
			"command must be non-empty",
		)
	}

	// Ignore the acknowledgement field.
	// We do not care about the value. Having the command it enough.

	return &output, nil
}
