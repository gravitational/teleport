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

	"github.com/gravitational/teleport/api/types"
)

// inventoryGetter is an interface used to retrieve the current inventory.
type inventoryGetter interface {
	// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
	GetInventoryConnectedServiceCount(service types.SystemRole) uint64
}

// isOktaServiceConnected will return true if an Okta service is seen in the inventory.
func isOktaServiceConnected(ctx context.Context, getter inventoryGetter) bool {
	return getter.GetInventoryConnectedServiceCount(types.RoleOkta) > 0
}
