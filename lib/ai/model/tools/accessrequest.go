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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	modeloutput "github.com/gravitational/teleport/lib/ai/model/output"
)

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
	foundResources := make([]promptResource, 0)
	foundResourcesMu := &sync.Mutex{}
	g := new(errgroup.Group)

	searchAndAppend := func(resourceType string, convert func(types.Resource) (promptResource, error)) error {
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

			foundResourcesMu.Lock()
			foundResources = append(foundResources, resource)
			foundResourcesMu.Unlock()
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

	g.Go(func() error {
		return searchAndAppend(types.KindNode, func(resource types.Resource) (promptResource, error) {
			return promptResource{
				Name:         resource.GetName(),
				Kind:         resource.GetKind(),
				SubKind:      resource.GetSubKind(),
				Labels:       resource.GetMetadata().Labels,
				FriendlyName: resource.(types.Server).GetHostname(),
			}, nil
		})
	})
	g.Go(func() error { return searchAndAppendPlain(types.KindApp) })
	g.Go(func() error { return searchAndAppendPlain(types.KindKubernetesCluster) })
	g.Go(func() error { return searchAndAppendPlain(types.KindDatabase) })
	g.Go(func() error { return searchAndAppendPlain(types.KindWindowsDesktop) })

	if err := g.Wait(); err != nil {
		return "", trace.Wrap(err)
	}
	sb := strings.Builder{}
	total := 0
	for _, resource := range foundResources {
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

func (*AccessRequestCreateTool) ParseInput(input string) (*modeloutput.AccessRequest, error) {
	output, err := modeloutput.ParseJSONFromModel[modeloutput.AccessRequest](input)
	if err != nil {
		return nil, err
	}

	if output.Reason == "" {
		return nil, modeloutput.NewInvalidOutputError(
			"access request create: missing reason",
			"a reason must be specified for the access request",
		)
	}

	if len(output.Roles) == 0 && len(output.Resources) == 0 {
		return nil, modeloutput.NewInvalidOutputError(
			"access request create: no requested roles or resources",
			"an access request must be for one or more roles OR one or more resources",
		)
	}

	for i, resource := range output.Resources {
		if resource.Type == "" {
			return nil, modeloutput.NewInvalidOutputError(
				"access request create: missing type at index "+strconv.Itoa(i),
				"a type must be provided for each resource",
			)
		}

		if resource.Name == "" {
			return nil, modeloutput.NewInvalidOutputError(
				"access request create: missing name at index "+strconv.Itoa(i),
				"a name must be provided for each resource",
			)
		}

		if resource.Type == types.KindNode {
			if _, err := uuid.Parse(resource.Name); err != nil {
				return nil, modeloutput.NewInvalidOutputError(
					"access request create: invalid name at index "+strconv.Itoa(i),
					"a name must be a valid UUID",
				)
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
