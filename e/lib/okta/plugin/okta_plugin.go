package oktaplugin

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// Get fetches the Okta plugin if it exists and does proper type assertions.
//
// TODO(kopiczko) Move to OSS repo.
func Get(ctx context.Context, plugins services.Plugins, withSecrets bool) (*types.PluginV1, error) {
	plugin, err := plugins.GetPlugin(ctx, types.PluginTypeOkta, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin.(%T) is not of type PluginV1", plugin)
	}

	oktaSettings := pluginV1.Spec.GetOkta()
	if oktaSettings == nil {
		return nil, trace.BadParameter("plugin %q does not have Okta settings", plugin.GetName())
	}
	if oktaSettings.SyncSettings == nil {
		oktaSettings.SyncSettings = &types.PluginOktaSyncSettings{}
	}

	return pluginV1, nil
}
