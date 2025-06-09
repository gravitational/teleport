package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var plugin = resource{
	getHandler:    getPlugin,
	createHandler: createPlugin,
	updateHandler: updatePlugin,
}

// pluginResourceWrapper provides custom JSON unmarshaling for Plugin resource
// types. The Plugin resource uses structures generated from a protobuf `oneof`
// directive, which the stdlib JSON unmarshaller can't handle, so we use this
// custom wrapper to help.
type pluginResourceWrapper struct {
	types.PluginV1
}

func (p *pluginResourceWrapper) UnmarshalJSON(data []byte) error {
	// If your plugin contains a `oneof` message, implement custom UnmarshalJSON/MarshalJSON
	// using gogo/jsonpb for the type.
	const (
		credOauth2AccessToken             = "oauth2_access_token"
		credBearerToken                   = "bearer_token"
		credIdSecret                      = "id_secret"
		credStaticCredentialsRef          = "static_credentials_ref"
		settingsSlackAccessPlugin         = "slack_access_plugin"
		settingsOpsgenie                  = "opsgenie"
		settingsOpenAI                    = "openai"
		settingsOkta                      = "okta"
		settingsJamf                      = "jamf"
		settingsPagerDuty                 = "pager_duty"
		settingsMattermost                = "mattermost"
		settingsJira                      = "jira"
		settingsDiscord                   = "discord"
		settingsServiceNow                = "serviceNow"
		settingsGitlab                    = "gitlab"
		settingsEntraID                   = "entra_id"
		settingsDatadogIncidentManagement = "datadog_incident_management"
		settingsEmailAccessPlugin         = "email_access_plugin"
		settingsAWSIdentityCenter         = "aws_ic"
		settingsNetIQ                     = "net_iq"
	)
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
			switch k {
			case credOauth2AccessToken:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_Oauth2AccessToken{}
			case credBearerToken:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_BearerToken{}
			case credIdSecret:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_IdSecret{}
			case credStaticCredentialsRef:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_StaticCredentialsRef{}
			default:
				return trace.BadParameter("unsupported plugin credential type: %v", k)
			}
		}
	}

	for k := range unknownPlugin.Spec.Settings {
		switch k {
		case settingsSlackAccessPlugin:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_SlackAccessPlugin{}
		case settingsOpsgenie:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Opsgenie{}
		case settingsOpenAI:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Openai{}
		case settingsOkta:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Okta{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_Okta{}
		case settingsJamf:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Jamf{}
		case settingsPagerDuty:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_PagerDuty{}
		case settingsMattermost:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Mattermost{}
		case settingsJira:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Jira{}
		case settingsDiscord:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Discord{}
		case settingsServiceNow:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_ServiceNow{}
		case settingsGitlab:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Gitlab{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_Gitlab{}
		case settingsEntraID:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_EntraId{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_EntraId{}
		case settingsDatadogIncidentManagement:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Datadog{}
		case settingsEmailAccessPlugin:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Email{}
		case settingsAWSIdentityCenter:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_AwsIc{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_AwsIc{}
		case settingsNetIQ:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_NetIq{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_NetIq{}

		default:
			return trace.BadParameter("unsupported plugin type: %v", k)
		}
	}

	if err := json.Unmarshal(data, &p.PluginV1); err != nil {
		return err
	}
	return nil
}
func updatePlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	item := pluginResourceWrapper{PluginV1: types.PluginV1{}}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{Plugin: &item.PluginV1}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func createPlugin(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	item := pluginResourceWrapper{
		PluginV1: types.PluginV1{},
	}
	if err := utils.FastUnmarshal(raw.Raw, &item); err != nil {
		return trace.Wrap(err)
	}
	if !opts.force {
		// Plugin needs to be installed before it can be updated.
		return trace.BadParameter("Only plugin update operation is supported. Please use 'tctl plugins install' instead\n")
	}
	if _, err := client.PluginsClient().UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{Plugin: &item.PluginV1}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("plugin %q has been updated\n", item.GetName())
	return nil
}

func getPlugin(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		plugin, err := client.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{Name: ref.Name})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewPluginCollection([]types.Plugin{plugin}), nil
	}
	var plugins []types.Plugin
	startKey := ""
	for {
		resp, err := client.PluginsClient().ListPlugins(ctx, &pluginsv1.ListPluginsRequest{
			PageSize:    100,
			StartKey:    startKey,
			WithSecrets: opts.withSecrets,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, v := range resp.Plugins {
			plugins = append(plugins, v)
		}
		if resp.NextKey == "" {
			break
		}
		startKey = resp.NextKey
	}
	return collections.NewPluginCollection(plugins), nil
}
