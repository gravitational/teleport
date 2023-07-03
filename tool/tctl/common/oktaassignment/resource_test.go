// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	assignmentMap := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal(assignmentYAML, &assignmentMap))

	resourceMap := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal(resourceYAML, &resourceMap))

	// Test that the enum fields have been properly converted to strings, then
	// assign the regular enum values to them so that we can do an equivalence
	// check against the assignment map later.
	resourceSpec, ok := resourceMap["spec"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, constants.OktaAssignmentStatusPending, resourceSpec["status"])
	resourceSpec["status"] = int(types.OktaAssignmentSpecV1_PENDING)

	resourceTargets, ok := resourceSpec["targets"].([]interface{})
	require.True(t, ok)

	resourceTarget1, ok := resourceTargets[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, constants.OktaAssignmentTargetApplication, resourceTarget1["type"])
	resourceTarget1["type"] = int(types.OktaAssignmentTargetV1_APPLICATION)

	resourceTarget2, ok := resourceTargets[1].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, constants.OktaAssignmentTargetGroup, resourceTarget2["type"])
	resourceTarget2["type"] = int(types.OktaAssignmentTargetV1_GROUP)

	require.Equal(t, assignmentMap, resourceMap)
}
