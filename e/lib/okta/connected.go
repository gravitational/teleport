/*
Copyright 2023 Gravitational, Inc.

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

package okta

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
)

// connectedGetter is an interface used to retrieve the current inventory or the current plugins..
type connectedGetter interface {
	// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
	GetInventoryConnectedServiceCount(service types.SystemRole) uint64

	// GetPlugins will get all plugins from the backend.
	GetPlugins(ctx context.Context, withSecrets bool) ([]types.Plugin, error)
}

// isOktaServiceConnected will return true if an Okta service is seen in the inventory or in the plugins list.
func isOktaServiceConnected(ctx context.Context, log logrus.FieldLogger, getter connectedGetter) bool {
	// Check to see if the Okta service is in the inventory.
	if getter.GetInventoryConnectedServiceCount(types.RoleOkta) > 0 {
		return true
	}

	// If it's not in the inventory, check to see if there's an Okta plugin.
	plugins, err := getter.GetPlugins(ctx, false)
	if err != nil {
		log.Errorf("error trying to get plugins to test for Okta service connectivity: %v", err)
		return false
	}

	for _, plugin := range plugins {
		if plugin.GetType() == types.PluginTypeOkta {
			return true
		}
	}

	return false
}
