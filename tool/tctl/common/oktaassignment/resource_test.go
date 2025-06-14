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

package oktaassignment

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestToResource will test that the enum values of the Okta assignment are human readable
// and make a best effort to ensuring there are no missing fields from the Okta assignment
// that have not yet been added to this resource.
func TestToResource(t *testing.T) {
	assignment, err := types.NewOktaAssignment(types.Metadata{
		Name: "assignment",
	}, types.OktaAssignmentSpecV1{
		User:   "user",
		Status: types.OktaAssignmentSpecV1_PENDING,
		Targets: []*types.OktaAssignmentTargetV1{
			{
				Id:   "1",
				Type: types.OktaAssignmentTargetV1_APPLICATION,
			},
			{
				Id:   "2",
				Type: types.OktaAssignmentTargetV1_GROUP,
			},
		},
	})
	require.NoError(t, err)

	resource := ToResource(assignment)

	buf := bytes.NewBuffer(nil)
	require.NoError(t, utils.WriteYAML(buf, assignment))
	assignmentYAML := buf.Bytes()

	buf = bytes.NewBuffer(nil)
	require.NoError(t, utils.WriteYAML(buf, resource))
	resourceYAML := buf.Bytes()

	// Unmarshal these to maps for easier controlled comparison
	assignmentMap := map[string]any{}
	require.NoError(t, yaml.Unmarshal(assignmentYAML, &assignmentMap))

	resourceMap := map[string]any{}
	require.NoError(t, yaml.Unmarshal(resourceYAML, &resourceMap))

	// Test that the enum fields have been properly converted to strings, then
	// assign the regular enum values to them so that we can do an equivalence
	// check against the assignment map later.
	resourceSpec, ok := resourceMap["spec"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, constants.OktaAssignmentStatusPending, resourceSpec["status"])
	resourceSpec["status"] = int(types.OktaAssignmentSpecV1_PENDING)

	resourceTargets, ok := resourceSpec["targets"].([]any)
	require.True(t, ok)

	resourceTarget1, ok := resourceTargets[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, constants.OktaAssignmentTargetApplication, resourceTarget1["type"])
	resourceTarget1["type"] = int(types.OktaAssignmentTargetV1_APPLICATION)

	resourceTarget2, ok := resourceTargets[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, constants.OktaAssignmentTargetGroup, resourceTarget2["type"])
	resourceTarget2["type"] = int(types.OktaAssignmentTargetV1_GROUP)

	require.Equal(t, assignmentMap, resourceMap)
}
