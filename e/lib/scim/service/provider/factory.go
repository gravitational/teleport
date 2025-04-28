package provider

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/provider/okta"
)

// CreateHandlerForPlugin creates a resource handler for the given plugin.
func CreateHandlerForPlugin(plugin types.Plugin, config common.Config, resourceType string) (common.ResourceHandler, error) {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("expected plugin to be of type PluginV1")
	}
	var createFn pluginHandlerFunc
	switch plugin.GetType() {
	case types.PluginTypeOkta:
		createFn = okta.New
	default:
		return nil, trace.BadParameter("unsupported plugin type: %v", plugin.GetType())
	}

	h, err := createFn(config, pluginV1, resourceType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return h, nil
}

type pluginHandlerFunc func(config common.Config, pluginV1 *types.PluginV1, resourceType string) (common.ResourceHandler, error)
