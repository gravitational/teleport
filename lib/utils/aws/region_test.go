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

package aws

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/aws"
)

func TestGetKnownRegions(t *testing.T) {
	// Picked a few regions just to make sure GetKnownRegions is returning
	// something that includes these.
	t.Run("hand picked", func(t *testing.T) {
		for _, region := range []string{
			"us-east-1",
			"il-central-1",
			"cn-north-1",
			"us-gov-west-1",
			"us-isob-east-1",
		} {
			require.Contains(t, GetKnownRegions(), region)
		}
	})

	// Ideally this should be tested in api/utils/aws but api has no access
	// to AWS SDK. If this fails aws.IsValidRegion should be updated.
	t.Run("IsValidRegion", func(t *testing.T) {
		for _, region := range GetKnownRegions() {
			require.NoError(t, aws.IsValidRegion(region))
		}
	})

}
func TestIsKnownRegion(t *testing.T) {
	for _, region := range GetKnownRegions() {
		require.True(t, IsKnownRegion(region))
	}

	for _, region := range []string{
		"us-east-100",
		"cn-north",
	} {
		require.False(t, IsKnownRegion(region))
	}
}
