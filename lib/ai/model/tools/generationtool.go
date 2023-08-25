/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
