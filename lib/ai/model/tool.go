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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	embeddinglib "github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// proxyLookupClusterMaxSize is max the number of nodes in the cluster to attempt an opportunistic node lookup
	// in the proxy cache. We always do embedding lookups if the cluster is larger than this number.
	proxyLookupClusterMaxSize = 100
	maxEmbeddingsPerLookup    = 10

	// TODO(joel): remove/change when migrating to embeddings
	maxShownRequestableItems = 50
)

// *ToolContext contains various "data" which is commonly needed by various tools.
type ToolContext struct {
	assist.AssistEmbeddingServiceClient
	AccessRequestClient
	AccessPoint
	services.AccessChecker
	NodeWatcher NodeWatcher
	User        string
	ClusterName string
}

// NodeWatcher abstracts away services.NodeWatcher for testing purposes.
type NodeWatcher interface {
	// GetNodes returns a list of nodes that match the given filter.
	GetNodes(ctx context.Context, fn func(n services.Node) bool) []types.Server

	// NodeCount returns the number of nodes in the cluster.
	NodeCount() int
}

// AccessPoint allows reading resources from proxy cache.
type AccessPoint interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// AccessRequestClient abstracts away the access request client for testing purposes.
type AccessRequestClient interface {
	CreateAccessRequest(ctx context.Context, req types.AccessRequest) error
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

// Tool is an interface that allows the agent to interact with the outside world.
// It is used to implement things such as vector document retrieval and command execution.
type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error)
}
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

// parseInput is called in a special case if the planned tool is CommandExecutionTool.
// This is because CommandExecutionTool is handled differently from most other tools and forcibly terminates the thought loop.
func (*CommandExecutionTool) parseInput(input string) (*CompletionCommand, error) {
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

type AccessRequestListRequestableRolesTool struct{}

func (*AccessRequestListRequestableRolesTool) Name() string {
	return "List Requestable Roles"
}

func (*AccessRequestListRequestableRolesTool) Description() string {
	return "List all roles that can be requested via access requests."
}

func (a *AccessRequestListRequestableRolesTool) Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error) {
	roles := toolCtx.AccessChecker.Roles()
	requestable := make(map[string]struct{}, 0)
	for _, role := range roles {
		for _, requestableRole := range role.GetAccessRequestConditions(types.Allow).Roles {
			requestable[requestableRole] = struct{}{}
		}
	}
	for _, role := range roles {
		for _, requestableRole := range role.GetAccessRequestConditions(types.Deny).Roles {
			delete(requestable, requestableRole)
		}
	}

	resp := strings.Builder{}
	for role := range requestable {
		resp.Write([]byte(role))
		resp.Write([]byte("\n"))
	}

	if resp.Len() == 0 {
		return "No requestable roles found", nil
	}

	return resp.String(), nil
}

type AccessRequestListRequestableResourcesTool struct{}

func (*AccessRequestListRequestableResourcesTool) Name() string {
	return "List Requestable Resources"
}

func (*AccessRequestListRequestableResourcesTool) Description() string {
	return `List all resources with IDs that can be requested via access requests.
This includes nodes via SSH access.`
}

func (a *AccessRequestListRequestableResourcesTool) Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error) {
	foundResources := make(chan promptResource, 0)
	var workersAlive atomic.Int32
	workersAlive.Store(5)

	searchAndAppend := func(resourceType string, convert func(types.Resource) (promptResource, error)) error {
		defer func() {
			if workersAlive.Add(-1) == 0 {
				close(foundResources)
			}
		}()

		list, err := toolCtx.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType:     resourceType,
			Limit:            maxShownRequestableItems,
			UseSearchAsRoles: true,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range list.Resources {
			resource, err := convert(resource)
			if err != nil {
				return trace.Wrap(err)
			}

			select {
			case foundResources <- resource:
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		}

		return nil
	}
	searchAndAppendPlain := func(resourceType string) error {
		return searchAndAppend(resourceType, func(resource types.Resource) (promptResource, error) {
			return promptResource{
				Name:    resource.GetName(),
				Kind:    resource.GetKind(),
				SubKind: resource.GetSubKind(),
				Labels:  resource.GetMetadata().Labels,
			}, nil
		})
	}

	go searchAndAppend(types.KindNode, func(resource types.Resource) (promptResource, error) {
		return promptResource{
			Name:         resource.GetName(),
			Kind:         resource.GetKind(),
			SubKind:      resource.GetSubKind(),
			Labels:       resource.GetMetadata().Labels,
			FriendlyName: resource.(types.Server).GetHostname(),
		}, nil
	})
	go searchAndAppendPlain(types.KindApp)
	go searchAndAppendPlain(types.KindKubernetesCluster)
	go searchAndAppendPlain(types.KindDatabase)
	go searchAndAppendPlain(types.KindWindowsDesktop)

	sb := strings.Builder{}
	total := 0
	for resource := range foundResources {
		yaml, err := yaml.Marshal(resource)
		if err != nil {
			return "", trace.Wrap(err)
		}

		sb.WriteString(string(yaml))
		sb.WriteString("\n")

		total++
		if total >= maxShownRequestableItems {
			break
		}
	}

	if sb.Len() == 0 {
		return "No requestable resources found", nil
	}

	return sb.String(), nil
}

type promptResource struct {
	Name         string            `yaml:"name"`
	Kind         string            `yaml:"kind"`
	SubKind      string            `yaml:"subkind"`
	Labels       map[string]string `yaml:"labels"`
	FriendlyName string            `yaml:"friendly_name,omitempty"`
}

type AccessRequestCreateTool struct{}

func (*AccessRequestCreateTool) Name() string {
	return "Create Access Requests"
}

func (*AccessRequestCreateTool) Description() string {
	return fmt.Sprintf(`Create an access request with a set of roles to, a set of resource UUIDs, a reason, and a set of suggested reviewers.
A valid access request must be either for one or more roles or for one or more resource IDs.
If the user is not specific enough, you may try to determine the correct roles or resource UUIDs by any means you see fit.

The input must be a JSON object with the following schema:

%vjson
{
	"roles": []string, \\ The optional set of roles being requested
	"resources": []{
		"type": string, \\ The resource type
		"id": string, \\ The resource name
		"friendlyName": string \\ Optional display-friendly name for the resource
	}, \\ The optional set of UUIDs for resources being requested
	"reason": string, \\ A reason for the request. This cannot be made up or inferred, it must be explicitly said by the user
	"suggested_reviewers": []string \\ An optional list of suggested reviewers; these must be Teleport usernames
}
%v
`, "```", "```")
}

func (*AccessRequestCreateTool) Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error) {
	// This is stubbed because AccessRequestCreateTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a suggestion UI prompt.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

func (*AccessRequestCreateTool) parseInput(input string) (*AccessRequest, error) {
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

	if len(output.Roles) == 0 && len(output.Resources) == 0 {
		return nil, &invalidOutputError{
			coarse: "access request create: no requested roles or resources",
			detail: "an access request must be for one or more roles OR one or more resources",
		}
	}

	for i, resource := range output.Resources {
		if resource.Type == "" {
			return nil, &invalidOutputError{
				coarse: "access request create: missing type at index " + strconv.Itoa(i),
				detail: "a type must be provided for each resource",
			}
		}

		if resource.Name == "" {
			return nil, &invalidOutputError{
				coarse: "access request create: missing name at index " + strconv.Itoa(i),
				detail: "a name must be provided for each resource",
			}
		}

		if resource.Type == types.KindNode {
			if _, err := uuid.Parse(resource.Name); err != nil {
				return nil, &invalidOutputError{
					coarse: "access request create: invalid name at index " + strconv.Itoa(i),
					detail: "a name must be a valid UUID",
				}
			}
		}
	}

	return &output, nil
}

type AccessRequestsListTool struct{}

func (*AccessRequestsListTool) Name() string {
	return "List Access Requests"
}

func (*AccessRequestsListTool) Description() string {
	return "List all access requests that the user has access to."
}

func (*AccessRequestsListTool) Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error) {
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
			Created:            request.GetCreationTime().Format(time.RFC3339),
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

type EmbeddingRetrievalTool struct{}

type EmbeddingRetrievalToolInput struct {
	Question string `json:"question"`
}

// tryNodeLookupFromProxyCache checks how many nodes the user has access to by
// hitting the proxy cache.  If the user has access to less than
// maxEmbeddingsPerLookup, the returned boolean indicates the lookup is
// successful and the result can be used. If the boolean is false, the caller
// must not use the returned result and perform a Node lookup via other means
// (embeddings lookup).
func (e *EmbeddingRetrievalTool) tryNodeLookupFromProxyCache(ctx context.Context, toolCtx *ToolContext) (bool, string, error) {
	nodes := toolCtx.NodeWatcher.GetNodes(ctx, func(node services.Node) bool {
		err := toolCtx.CheckAccess(node, services.AccessState{MFAVerified: true})
		return err == nil
	})
	if len(nodes) == 0 || len(nodes) > maxEmbeddingsPerLookup {
		return false, "", nil
	}
	sb := strings.Builder{}
	for _, node := range nodes {
		data, err := embeddinglib.SerializeNode(node)
		if err != nil {
			return false, "", trace.Wrap(err)
		}
		sb.Write(data)
		sb.WriteString("\n")
	}
	return true, sb.String(), nil
}

func (e *EmbeddingRetrievalTool) Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error) {
	inputCmd, outErr := e.parseInput(input)
	if outErr == nil {
		// If we failed to parse the input, we can still send the payload for embedding retrieval.
		// In most cases, we will still get some sensible results.
		// If we parsed the input successfully, we should use the parsed input instead.
		input = inputCmd.Question
	}
	log.Tracef("embedding retrieval input: %v", input)

	// Threshold to avoid looping over all nodes on large clusters
	if toolCtx.NodeWatcher != nil && toolCtx.NodeWatcher.NodeCount() < proxyLookupClusterMaxSize {
		ok, result, err := e.tryNodeLookupFromProxyCache(ctx, toolCtx)
		if err != nil {
			return "", trace.Wrap(err)
		}
		if ok {
			return result, nil
		}
	}

	resp, err := toolCtx.GetAssistantEmbeddings(ctx, &assist.GetAssistantEmbeddingsRequest{
		Username: toolCtx.User,
		Kind:     types.KindNode, // currently only node embeddings are supported
		Limit:    maxEmbeddingsPerLookup,
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

func (e *EmbeddingRetrievalTool) Name() string {
	return "Nodes names and labels retrieval"
}

func (e *EmbeddingRetrievalTool) Description() string {
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

func (*EmbeddingRetrievalTool) parseInput(input string) (*EmbeddingRetrievalToolInput, error) {
	output, err := parseJSONFromModel[EmbeddingRetrievalToolInput](input)
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
