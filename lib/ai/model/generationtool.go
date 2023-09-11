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

package model

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
)

type commandGenerationTool struct{}

type commandGenerationToolInput struct {
	// Command is a unix command to execute.
	Command string `json:"command"`
}

func (c *commandGenerationTool) Name() string {
	return "Command Generation"
}

func (c *commandGenerationTool) Description() string {
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

func (c *commandGenerationTool) Run(_ context.Context, _ string) (string, error) {
	// This is stubbed because commandGenerationTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a command suggestion to the user.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

// parseInput is called in a special case if the planned tool is commandExecutionTool.
// This is because commandExecutionTool is handled differently from most other tools and forcibly terminates the thought loop.
func (*commandGenerationTool) parseInput(input string) (*commandGenerationToolInput, error) {
	output, err := parseJSONFromModel[commandGenerationToolInput](input)
	if err != nil {
		return nil, err
	}

	if output.Command == "" {
		return nil, &invalidOutputError{
			coarse: "command generation: missing command",
			detail: "command must be non-empty",
		}
	}

	// Ignore the acknowledgement field.
	// We do not care about the value. Having the command it enough.

	return &output, nil
}
