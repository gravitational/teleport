package generic

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// New creates a new resource handler for the given resource type.
func New(config common.Config, plugin *types.PluginV1, resourceType string) (common.ResourceHandler, error) {
	switch resourceType {
	case "Schemas":
		return &schemaHandler{
			Config: config,
			Plugin: plugin,
		}, nil
	case "ResourceTypes":
		return &resourceTypesHandler{
			Config: config,
			Plugin: plugin,
		}, nil
	case "ServiceProviderConfig":
		return &serviceProviderConfigHandler{}, nil
	default:
		return nil, trace.BadParameter("unsupported resource type: %v", resourceType)
	}
}

type Generic struct {
	Plugin *types.PluginV1
	common.Config
}
