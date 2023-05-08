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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

type dummyInventoryGetter struct {
	summary proto.InventoryStatusSummary
}

// GetInventoryStatus returns the current inventory status.
func (d *dummyInventoryGetter) GetInventoryStatus(_ context.Context, _ proto.InventoryStatusRequest) proto.InventoryStatusSummary {
	return d.summary
}

func TestIsOktaConnected(t *testing.T) {
	ctx := context.Background()
	getter := &dummyInventoryGetter{
		summary: proto.InventoryStatusSummary{
			Connected: []proto.UpstreamInventoryHello{
				{
					Services: []types.SystemRole{types.RoleAdmin, types.RoleAuth},
				},
				{
					Services: []types.SystemRole{types.RoleOkta},
				},
			},
		},
	}

	require.True(t, isOktaServiceConnected(ctx, getter))

	getter.summary = proto.InventoryStatusSummary{
		Connected: []proto.UpstreamInventoryHello{
			{
				Services: []types.SystemRole{types.RoleAdmin, types.RoleAuth},
			},
			{
				Services: []types.SystemRole{types.RoleProxy},
			},
		},
	}

	require.False(t, isOktaServiceConnected(ctx, getter))
}
