/*
Copyright 2022 Gravitational, Inc.

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

//nolint:gci // Remove when GCI is fixed upstream https://github.com/daixiang0/gci/issues/135
package resources_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/operator/controllers/resources/testlib"
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

	return trace.Wrap(tClient.UpsertRole(ctx, tRole))
}

func teleportResourceToMap[T types.Resource](resource T) (map[string]interface{}, error) {
	resourceJSON, _ := json.Marshal(resource)
	resourceMap := make(map[string]interface{})
	err := json.Unmarshal(resourceJSON, &resourceMap)
	return resourceMap, trace.Wrap(err)
}
