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
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
)

type ToolContext struct {
	assist.AssistEmbeddingServiceClient
	AccessRequestClient
	Roles []types.Role
	User  string
}

type AccessRequestClient interface {
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

// Tool is an interface that allows the agent to interact with the outside world.
// It is used to implement things such as vector document retrieval and command execution.
type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, toolCtx ToolContext, input string) (string, error)
}
type commandExecutionTool struct{}

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

func (c *commandExecutionTool) Run(_ context.Context, _ ToolContext, _ string) (string, error) {
	// This is stubbed because commandExecutionTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a command suggestion to the user.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

// parseInput is called in a special case if the planned tool is commandExecutionTool.
// This is because commandExecutionTool is handled differently from most other tools and forcibly terminates the thought loop.
func (*commandExecutionTool) parseInput(input string) (*CompletionCommand, error) {
	output, err := parseJSONFromModel[CompletionCommand](input)
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

func (a *accessRequestListRequestableRolesTool) Run(ctx context.Context, toolCtx ToolContext, input string) (string, error) {
	requestable := make(map[string]struct{}, 0)
	for _, role := range toolCtx.Roles {
		for _, requestableRole := range role.GetAccessRequestConditions(types.Allow).Roles {
			requestable[requestableRole] = struct{}{}
		}
	}
	for _, role := range toolCtx.Roles {
		for _, requestableRole := range role.GetAccessRequestConditions(types.Deny).Roles {
			delete(requestable, requestableRole)
		}
	}

	resp := strings.Builder{}
	for role := range requestable {
		resp.Write([]byte(role))
		resp.Write([]byte("\n"))
	}

	return resp.String(), nil
}

type accessRequestCreateTool struct{}

func (*accessRequestCreateTool) Name() string {
	return "Create Access Requests"
}

func (*accessRequestCreateTool) Description() string {
	return fmt.Sprintf(`Create an access request with a set of roles, a reason, and a set of suggested reviewers.
You must get this information from the conversations context or by asking the user for clarification.
The input must be a JSON object with the following schema:

%vjson
{
	"roles": []string, \\ The optional set of roles being requested
	"reason": string, \\ A reason for the request; attempt to ask the user for this if not provided
	"suggested_reviewers": []string \\ An optional list of suggested reviewers; these must be Teleport usernames
}
%v
`, "```", "```")
}

func (*accessRequestCreateTool) Run(ctx context.Context, toolCtx ToolContext, input string) (string, error) {
	// This is stubbed because accessRequestCreateTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a suggestion UI prompt.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

func (*accessRequestCreateTool) parseInput(input string) (*AccessRequest, error) {
	output, err := parseJSONFromModel[AccessRequest](input)
	if err != nil {
		return nil, err
	}

	if output.Reason == "" {
		return nil, &invalidOutputError{
			coarse: "access request create: missing reason",
			detail: "a reason must be specified for the access request",
		}
	}

	if len(output.Roles) == 0 {
		return nil, &invalidOutputError{
			coarse: "access request create: no requested roles",
			detail: "an access request must be for one or more roles",
		}
	}

	return &output, nil
}

type accessRequestsListTool struct{}

func (*accessRequestsListTool) Name() string {
	return "List Access Requests"
}

func (*accessRequestsListTool) Description() string {
	return "List all access requests that the user has access to."
}

func (*accessRequestsListTool) Run(ctx context.Context, toolCtx ToolContext, input string) (string, error) {
	requests, err := toolCtx.GetAccessRequests(ctx, types.AccessRequestFilter{
		User: toolCtx.User,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	items := make([]accessRequestLLMItem, 0, len(requests))
	for _, request := range requests {
		items = append(items, accessRequestLLMItem{
			Roles:              request.GetRoles(),
			RequestReason:      request.GetRequestReason(),
			SuggestedReviewers: request.GetSuggestedReviewers(),
			State:              request.GetState().String(),
			ResolveReason:      request.GetResolveReason(),
			Created:            request.GetCreationTime().Format(time.RFC1123),
		})
	}

	itemYaml, err := yaml.Marshal(items)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(itemYaml), nil
}

type accessRequestLLMItem struct {
	Roles              []string `yaml:"roles"`
	RequestReason      string   `yaml:"request_reason"`
	SuggestedReviewers []string `yaml:"suggested_reviewers"`
	State              string   `yaml:"state"`
	ResolveReason      string   `yaml:"resolved_reason"`
	Created            string   `yaml:"created"`
}

type accessRequestsDisplayTool struct{}

func (*accessRequestsDisplayTool) Name() string {
	return "Display Access Requests to User"
}

func (*accessRequestsDisplayTool) Description() string {
	return `Directly display a smart view of all access requests the user has access to.
Prefer this when the user is directly asking to see access requests.`
}

func (*accessRequestsDisplayTool) Run(ctx context.Context, toolCtx ToolContext, input string) (string, error) {
	// This is stubbed because accessRequestsDisplayTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a suggestion UI prompt.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

type embeddingRetrievalTool struct{}

type embeddingRetrievalToolInput struct {
	Question string `json:"question"`
}

func (e *embeddingRetrievalTool) Run(ctx context.Context, toolCtx ToolContext, input string) (string, error) {
	inputCmd, outErr := e.parseInput(input)
	if outErr == nil {
		// If we failed to parse the input, we can still send the payload for embedding retrieval.
		// In most cases, we will still get some sensible results.
		// If we parsed the input successfully, we should use the parsed input instead.
		input = inputCmd.Question
	}
	log.Tracef("embedding retrieval input: %v", input)

	resp, err := toolCtx.GetAssistantEmbeddings(ctx, &assist.GetAssistantEmbeddingsRequest{
		Username: toolCtx.User,
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
