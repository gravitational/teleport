/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type pluginCollection struct {
	plugins []types.Plugin
}

// pluginResourceWrapper provides custom JSON unmarshaling for Plugin resource
// types. The Plugin resource uses structures generated from a protobuf `oneof`
// directive, which the stdlib JSON unmarshaller can't handle, so we use this
// custom wrapper to help.
type pluginResourceWrapper struct {
	types.PluginV1
}

var pluginSettingsConstructors = map[string]func(s *types.PluginSpecV1){
	"slack_access_plugin": func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_SlackAccessPlugin{} },
	"opsgenie":            func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Opsgenie{} },
	"openai":              func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Openai{} },
	"okta":                func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Okta{} },
	"jamf":                func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Jamf{} },
	"intune":              func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Intune{} },
	"pager_duty":          func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_PagerDuty{} },
	"mattermost":          func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Mattermost{} },
	"jira":                func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Jira{} },
	"discord":             func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Discord{} },
	"serviceNow":          func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_ServiceNow{} },
	"gitlab":              func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Gitlab{} },
	"entra_id":            func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_EntraId{} },
	"datadog":             func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Datadog{} },
	"email":               func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Email{} },
	"aws_ic":              func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_AwsIc{} },
	"net_iq":              func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_NetIq{} },
	"msteams":             func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Msteams{} },
	"scim":                func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Scim{} },
	"github":              func(s *types.PluginSpecV1) { s.Settings = &types.PluginSpecV1_Github{} },
}

var pluginStatusDetailsConstructors = map[string]func(s *types.PluginStatusV1){
	"gitlab":   func(s *types.PluginStatusV1) { s.Details = &types.PluginStatusV1_Gitlab{} },
	"entra_id": func(s *types.PluginStatusV1) { s.Details = &types.PluginStatusV1_EntraId{} },
	"okta":     func(s *types.PluginStatusV1) { s.Details = &types.PluginStatusV1_Okta{} },
	"aws_ic":   func(s *types.PluginStatusV1) { s.Details = &types.PluginStatusV1_AwsIc{} },
	"net_iq":   func(s *types.PluginStatusV1) { s.Details = &types.PluginStatusV1_NetIq{} },
}

var pluginCredentialConstructors = map[string]func(c *types.PluginCredentialsV1){
	"oauth2_access_token":    func(c *types.PluginCredentialsV1) { c.Credentials = &types.PluginCredentialsV1_Oauth2AccessToken{} },
	"bearer_token":           func(c *types.PluginCredentialsV1) { c.Credentials = &types.PluginCredentialsV1_BearerToken{} },
	"id_secret":              func(c *types.PluginCredentialsV1) { c.Credentials = &types.PluginCredentialsV1_IdSecret{} },
	"static_credentials_ref": func(c *types.PluginCredentialsV1) { c.Credentials = &types.PluginCredentialsV1_StaticCredentialsRef{} },
}

func (p *pluginResourceWrapper) UnmarshalJSON(data []byte) error {
	// If your plugin contains a `oneof` message, implement custom UnmarshalJSON/MarshalJSON
	// using gogo/jsonpb for the type.
	type unknownPluginType struct {
		Spec struct {
			Settings map[string]json.RawMessage `json:"Settings"`
		} `json:"spec"`
		Status struct {
			Details map[string]json.RawMessage `json:"Details"`
		} `json:"status"`
		Credentials struct {
			Credentials map[string]json.RawMessage `json:"Credentials"`
		} `json:"credentials"`
	}

	var unknownPlugin unknownPluginType
	if err := json.Unmarshal(data, &unknownPlugin); err != nil {
		return err
	}

	if unknownPlugin.Spec.Settings == nil {
		return trace.BadParameter("plugin settings are missing")
	}
	if len(unknownPlugin.Spec.Settings) != 1 {
		return trace.BadParameter("unknown plugin settings count")
	}

	if len(unknownPlugin.Credentials.Credentials) == 1 {
		p.PluginV1.Credentials = &types.PluginCredentialsV1{}
		for k := range unknownPlugin.Credentials.Credentials {
			construct, ok := pluginCredentialConstructors[k]
			if !ok {
				return trace.BadParameter("unsupported plugin credential type: %v", k)
			}
			construct(p.PluginV1.Credentials)
		}
	}

	for k := range unknownPlugin.Spec.Settings {
		construct, ok := pluginSettingsConstructors[k]
		if !ok {
			return trace.BadParameter("unsupported plugin type: %v", k)
		}
		construct(&p.PluginV1.Spec)
		if constructStatus, ok := pluginStatusDetailsConstructors[k]; ok {
			constructStatus(&p.PluginV1.Status)
		}
	}

	if err := json.Unmarshal(data, &p.PluginV1); err != nil {
		return err
	}
	return nil
}

func (c *pluginCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.plugins))
	for i, resource := range c.plugins {
		r[i] = resource
	}
	return r
}

func (c *pluginCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Status"})
	for _, plugin := range c.plugins {
		t.AddRow([]string{
			plugin.GetName(),
			plugin.GetStatus().GetCode().String(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func pluginHandler() Handler {
	return Handler{
		getHandler:    getPlugin,
		createHandler: createPlugin,
		updateHandler: updatePlugin,
		mfaRequired:   true,
		description:   "Represents a hosted plugin (e.g. Slack, PagerDuty, Jira) run by the Auth Service.",
	}
}

func getPlugin(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		plugin, err := client.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{Name: ref.Name}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &pluginCollection{plugins: []types.Plugin{plugin}}, nil
	}
	var plugins []types.Plugin
	startKey := ""
	for {
		resp, err := client.PluginsClient().ListPlugins(ctx, pluginsv1.ListPluginsRequest_builder{
			PageSize:    100,
			StartKey:    startKey,
			WithSecrets: opts.WithSecrets,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, v := range resp.GetPlugins() {
			plugins = append(plugins, v)
		}
		if resp.GetNextKey() == "" {
			break
		}
		startKey = resp.GetNextKey()
	}
	return &pluginCollection{plugins: plugins}, nil
}

func createPlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	item := pluginResourceWrapper{
		PluginV1: types.PluginV1{},
	}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if !opts.Force {
		// Plugin needs to be installed before it can be updated.
		return trace.BadParameter("Only plugin update operation is supported. Please use 'tctl plugins install' instead\n")
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, pluginsv1.UpdatePluginRequest_builder{Plugin: &item.PluginV1}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("plugin %q has been updated\n", item.GetName())
	return nil
}

func updatePlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	item := pluginResourceWrapper{PluginV1: types.PluginV1{}}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, pluginsv1.UpdatePluginRequest_builder{Plugin: &item.PluginV1}.Build()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
