package plugin

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Validate validates the plugin before writing it in the storage.
func Validate(plugin types.Plugin) error {
	if plugin == nil {
		return nil
	}
	switch plugin.GetType() {
	case types.PluginTypeOkta:
		pluginV1, err := toPluginV1(plugin)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(validateOkta(pluginV1))
	}
	return nil
}

func toPluginV1(plugin types.Plugin) (*types.PluginV1, error) {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin.(%T) %q is not of type PluginV1", plugin, plugin.GetName())
	}
	return pluginV1, nil
}

func validateOkta(plugin *types.PluginV1) error {
	oktaSettings := plugin.Spec.GetOkta()
	if oktaSettings == nil {
		return trace.BadParameter("plugin %q does not have Okta settings", plugin.GetName())
	}

	_, err := OktaParseTimeBetweenImports(oktaSettings.GetSyncSettings())
	return trace.Wrap(err)
}
