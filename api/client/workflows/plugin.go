/*
Copyright 2021 Gravitational, Inc.

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

package workflows

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// Plugin is a management plugin for Teleport access request.
// It can be used to create, retrieve, or update requests.
//
// It can also be used to watch for request events from a event stream from the
// Auth server. This can be useful for integrating with external tools.
// We have already integrated with some popular external tools using this Plugin.
// See https://goteleport.com/docs/enterprise/workflow/#integrating-with-an-external-tool.
//
// Plugin must have a client with an open connection to a Teleport Auth server in
// order to function See pkg.go.dev/github.com/gravitational/teleport/api/client
// for Teleport client documentation.
type Plugin struct {
	client *client.Client
	name   string
}

// NewPlugin creates a new plugin using the given client and plugin name.
func NewPlugin(ctx context.Context, name string, client *client.Client) *Plugin {
	return &Plugin{
		client: client,
		name:   name,
	}
}

// WatchRequests registers a new watcher for pending access requests.
func (p *Plugin) WatchRequests(ctx context.Context, filter Filter) (RequestWatcher, error) {
	return newRequestWatcher(ctx, p.client, filter)
}

// Close closes the plugin's underlying client.
func (p *Plugin) Close() error {
	return p.client.Close()
}

// CreateRequest creates a new Request for the given user to access the given role(s).
func (p *Plugin) CreateRequest(ctx context.Context, user string, roles ...string) (Request, error) {
	req, err := types.NewAccessRequest(uuid.New(), user, roles...)
	if err != nil {
		return Request{}, trace.Wrap(err)
	}
	if err = p.client.CreateAccessRequest(ctx, req); err != nil {
		return Request{}, trace.Wrap(err)
	}
	return requestFromAccessRequest(req), nil
}

// GetRequests loads all requests which match the provided filter.
func (p *Plugin) GetRequests(ctx context.Context, filter Filter) ([]Request, error) {
	accessRequests, err := p.client.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var reqs []Request
	for _, ar := range accessRequests {
		reqs = append(reqs, requestFromAccessRequest(ar))
	}
	return reqs, nil
}

// GetRequest loads a request matching ID.
func (p *Plugin) GetRequest(ctx context.Context, reqID string) (Request, error) {
	reqs, err := p.GetRequests(ctx, Filter{
		ID: reqID,
	})
	if err != nil {
		return Request{}, trace.Wrap(err)
	}
	if len(reqs) < 1 {
		return Request{}, trace.NotFound("no request matching %q", reqID)
	}
	return reqs[0], nil
}

// SetRequestState updates the state of a request.
func (p *Plugin) SetRequestState(ctx context.Context, reqID string, delegator string, params RequestUpdate) error {
	ctx = utils.WithDelegator(ctx, fmt.Sprintf("%s:%s", p.name, delegator))
	err := p.client.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID:   reqID,
		State:       params.State,
		Reason:      params.Reason,
		Annotations: params.Annotations,
		Roles:       params.Roles,
	})
	return trace.Wrap(err)
}

// GetPluginData fetches plugin data of the specific request.
func (p *Plugin) GetPluginData(ctx context.Context, reqID string) (PluginDataMap, error) {
	pluginDatas, err := p.client.GetPluginData(ctx, types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: reqID,
		Plugin:   p.name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(pluginDatas) == 0 {
		return PluginDataMap{}, nil
	}

	var pluginData types.PluginData = pluginDatas[0]
	entry := pluginData.Entries()[p.name]
	if entry == nil {
		return PluginDataMap{}, nil
	}
	return entry.Data, nil
}

// UpdatePluginData updates plugin data of the specific request comparing it with a previous value.
func (p *Plugin) UpdatePluginData(ctx context.Context, reqID string, set PluginDataMap, expect PluginDataMap) error {
	err := p.client.UpdatePluginData(ctx, types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: reqID,
		Plugin:   p.name,
		Set:      set,
		Expect:   expect,
	})
	return trace.Wrap(err)
}
