package okta

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// connectedGetter is an interface used to retrieve the current inventory or the current plugins..
type connectedGetter interface {
	// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
	GetInventoryConnectedServiceCount(service types.SystemRole) uint64
}

// isOktaServiceConnected will return true if an Okta service is seen in the inventory or in the plugins list.
func isOktaServiceConnected(ctx context.Context, log logrus.FieldLogger, getter connectedGetter, plugins services.Plugins) bool {
	// Check to see if the Okta service is in the inventory.
	if getter.GetInventoryConnectedServiceCount(types.RoleOkta) > 0 {
		return true
	}

	if plugins != nil {
		// If it's not in the inventory, check to see if there's an Okta plugin.
		hasPlugin, err := plugins.HasPluginType(ctx, types.PluginTypeOkta)
		if err != nil {
			log.WithError(err).Error("Error trying to get plugins to test for Okta service connectivity")
			return false
		}

		return hasPlugin
	}

	return false
}
