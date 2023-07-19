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
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
)

// Tool is an interface that allows the agent to interact with the outside world.
// It is used to implement things such as vector document retrieval and command execution.
type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, input string) (string, error)
}
type commandExecutionTool struct{}

type commandExecutionToolInput struct {
	// Command is a unix command to execute.
	Command string `json:"command"`

	// Nodes is a list of hostnames to execute the command on.
	Nodes []string `json:"nodes"`

	// Labels is a list of labels specifying node groups to execute the command on.
	Labels []Label `json:"labels"`
}

func (c *commandExecutionTool) Name() string {
	return "Command Execution"
}

func (c *commandExecutionTool) Description() string {
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

func (c *commandExecutionTool) Run(_ context.Context, _ string) (string, error) {
	// This is stubbed because commandExecutionTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a command suggestion to the user.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

// parseInput is called in a special case if the planned tool is commandExecutionTool.
// This is because commandExecutionTool is handled differently from most other tools and forcibly terminates the thought loop.
func (*commandExecutionTool) parseInput(input string) (*commandExecutionToolInput, error) {
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

// TODO: investigate integrating this into embeddingRetrievalTool
type accessRequestListRequestableRolesTool struct{}

func (*accessRequestListRequestableRolesTool) Name() string {
	return "List Requestable Roles"
}

func (*accessRequestListRequestableRolesTool) Description() string {
	return "List all roles that can be requested via access requests."
}

func (*accessRequestListRequestableRolesTool) Run(ctx context.Context, input string) (string, error) {
	return "No requestable roles available.", nil
}

type accessRequestCreateTool struct{}

func (*accessRequestCreateTool) Name() string {
	return "Create Access Requests"
}

func (*accessRequestCreateTool) Description() string {
	return fmt.Sprintf(`Create an access request with a set of roles, a reason, a set of suggested reviewers and a set of resource IDs.
The input must be a JSON object with the following schema:

%vjson
{
	"roles": []string, \\ The optional set of roles being requested
	"reason": string, \\ A reason for the request; attempt to ask the user for this if not provided
	"suggested_reviewers": []string \\ An optional list of suggested reviewers; these must be Teleport usernames
	"resource_ids": []string \\ An optional list of resource IDs that the user is requesting access to
}
%v
`, "```", "```")
}

func (*accessRequestCreateTool) Run(ctx context.Context, input string) (string, error) {
	return "Cannot create access requests at the moment.", nil
}

// TODO: investigate integrating this into embeddingRetrievalTool
type accessRequestListTool struct{}

func (*accessRequestListTool) Name() string {
	return "List Access Requests"
}

func (*accessRequestListTool) Description() string {
	return "List all access requests created by the user."
}

func (*accessRequestListTool) Run(ctx context.Context, input string) (string, error) {
	return "No access requests open.", nil
}

type accessRequestDeleteTool struct{}

func (*accessRequestDeleteTool) Name() string {
	return "Delete Access Requests"
}

func (*accessRequestDeleteTool) Description() string {
	return fmt.Sprintf(`Delete an access request with a given UUID.
The input must be a JSON object with the following schema:

%vjson
{
	"uuid": string \\ UUID of the access request to delete
}
%v
`, "```", "```")
}

func (*accessRequestDeleteTool) Run(ctx context.Context, input string) (string, error) {
	return "Cannot delete access requests at the moment.", nil
}

type embeddingRetrievalTool struct {
	assistClient assist.AssistEmbeddingServiceClient
	currentUser  string
}

type embeddingRetrievalToolInput struct {
	Question string `json:"question"`
}

func (e *embeddingRetrievalTool) Run(ctx context.Context, input string) (string, error) {
	inputCmd, outErr := e.parseInput(input)
	if outErr == nil {
		// If we failed to parse the input, we can still send the payload for embedding retrieval.
		// In most cases, we will still get some sensible results.
		// If we parsed the input successfully, we should use the parsed input instead.
		input = inputCmd.Question
	}
	log.Tracef("embedding retrieval input: %v", input)

	resp, err := e.assistClient.GetAssistantEmbeddings(ctx, &assist.GetAssistantEmbeddingsRequest{
		Username: e.currentUser,
		Kind:     types.KindNode, // currently only node embeddings are supported
		Limit:    10,
		Query:    input,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	sb := strings.Builder{}
	for _, embedding := range resp.Embeddings {
		sb.WriteString(embedding.Content)
		sb.WriteString("\n")
	}

	log.Tracef("embedding retrieval: %v", sb.String())

	if sb.Len() == 0 {
		// Either no nodes are connected, embedding process hasn't started yet, or
		// the user doesn't have access to any resources.
		return "Didn't find any nodes matching the query", nil
	}

	return sb.String(), nil
}

func (e *embeddingRetrievalTool) Name() string {
	return "Nodes names and labels retrieval"
}

func (e *embeddingRetrievalTool) Description() string {
	return fmt.Sprintf(`Ask about existing remote nodes that user has access to fetch node names or/and set of labels. 
Always use this capability before returning generating any command. Do not assume that the user has access to any nodes. Returning a command without checking for access will result in an error.
Always prefer to use labler rather than node names.
The input must be a JSON object with the following schema:
%vjson
{
	"question": string \\ Question about the available remote nodes
}
%v
`, "```", "```")
}

func (*embeddingRetrievalTool) parseInput(input string) (*embeddingRetrievalToolInput, error) {
	output, err := parseJSONFromModel[embeddingRetrievalToolInput](input)
	if err != nil {
		return nil, err
	}

	if len(output.Question) == 0 {
		return nil, &invalidOutputError{
			coarse: "embedding retrieval: missing question",
			detail: "question must be non-empty",
		}
	}

	return &output, nil
}
