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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestIsOktaConnected(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewFakeClock())
	ap.serviceCounts = map[types.SystemRole]uint64{
		types.RoleAdmin: 1,
		types.RoleAuth:  1,
		types.RoleOkta:  1,
	}

	require.True(t, isOktaServiceConnected(ctx, ap))

	ap.serviceCounts = map[types.SystemRole]uint64{
		types.RoleAdmin: 1,
		types.RoleAuth:  1,
	}

	require.False(t, isOktaServiceConnected(ctx, ap))
}
