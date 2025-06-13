/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

//nolint:gci // Remove when GCI is fixed upstream https://github.com/daixiang0/gci/issues/135
package resources_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

// Temporary type alias to slightly decrease the size of this commit, it can be
// removed in a follow-up.
type testSetup = testlib.TestSetup

// Temporary function "aliases" to slightly decrease the size of this commit,
// they can be removed in a follow-up.
func setupTestEnv(t *testing.T, opts ...testlib.TestOption) *testlib.TestSetup {
	return testlib.SetupTestEnv(t, opts...)
}
func validRandomResourceName(prefix string) string       { return testlib.ValidRandomResourceName(prefix) }
func fastEventually(t *testing.T, condition func() bool) { testlib.FastEventually(t, condition) }
func fastEventuallyWithT(t *testing.T, condition func(*assert.CollectT)) {
	testlib.FastEventuallyWithT(t, condition)
}

func teleportCreateDummyRole(ctx context.Context, roleName string, tClient *client.Client) error {
	// The role is created in Teleport
	tRole, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"a", "b"},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	metadata := tRole.GetMetadata()
	metadata.Labels = map[string]string{types.OriginLabel: types.OriginKubernetes}
	tRole.SetMetadata(metadata)

	_, err = tClient.UpsertRole(ctx, tRole)
	return trace.Wrap(err)
}

func teleportResourceToMap[T types.Resource](resource T) (map[string]any, error) {
	resourceJSON, _ := json.Marshal(resource)
	resourceMap := make(map[string]any)
	err := json.Unmarshal(resourceJSON, &resourceMap)
	return resourceMap, trace.Wrap(err)
}
