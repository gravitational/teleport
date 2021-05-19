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
//
// Plugin must have a client with an open connection to a Teleport Auth server in
// order to function. See pkg.go.dev/github.com/gravitational/teleport/api/client
// for Teleport client documentation.
//
// Plugin is stateless, meaning it does not store any of its own data. This
// allows a Plugin to be restarted, stopped, etc. at any time. Plugin data can
// however be stored directly on requests using the PluginData methods.
//
// Plugin can watch for request events from an event stream from the Auth server.
// This can be used to automatically resolve access requests based on custom logic or external tools.
// We have already integrated with some popular external tools using this Plugin.
// See https://goteleport.com/docs/enterprise/workflow/#integrating-with-an-external-tool.
type Plugin struct {
	client *client.Client
	name   string
}

// NewPlugin creates a new plugin using the given client and plugin name.
// The plugin's name is used for auditing and to store plugin data on requests.
func NewPlugin(ctx context.Context, name string, client *client.Client) *Plugin {
	return &Plugin{
		client: client,
		name:   name,
	}
}

// WatchRequests registers a new watcher for access requests.
func (p *Plugin) WatchRequests(ctx context.Context, filter types.AccessRequestFilter) (*RequestWatcher, error) {
	return newRequestWatcher(ctx, p.client, filter)
}

// CreateRequest creates a new Request for the given user to access the given role(s).
func (p *Plugin) CreateRequest(ctx context.Context, user string, roles ...string) (types.AccessRequest, error) {
	req, err := types.NewAccessRequest(uuid.New(), user, roles...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = p.client.CreateAccessRequest(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// GetRequests loads all requests which match the provided filter.
func (p *Plugin) GetRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	reqs, err := p.client.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reqs, nil
}

// GetRequest loads a request matching ID.
func (p *Plugin) GetRequest(ctx context.Context, reqID string) (types.AccessRequest, error) {
	reqs, err := p.GetRequests(ctx, types.AccessRequestFilter{
		ID: reqID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(reqs) < 1 {
		return nil, trace.NotFound("no request matching %q", reqID)
	}
	return reqs[0], nil
}

// SetRequestState updates the state of a request. The request's delegator will be set as
// "[plugin.name]:[delegator]" and can be used to audit request state change events.
func (p *Plugin) SetRequestState(ctx context.Context, reqID string, delegator string, params types.AccessRequestUpdate) error {
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

// PluginDataMap is custom user data associated with an access request.
// It can be used to store arbitrary plugin data directly on requests.
type PluginDataMap map[string]string

// GetPluginData fetches plugin data of the specific request. This can be used
// to retrieve plugin data that was previously stored on the request.
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
// If expect is provided and it doesn't match the plugin data presently stored on the backend, the request will fail.
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
