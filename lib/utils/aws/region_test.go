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
)

func TestIsKnownRegion(t *testing.T) {
	for _, region := range []string{
		"us-east-1",
		"cn-north-1",
		"us-gov-west-1",
		"us-isob-east-1",
	} {
		require.True(t, IsKnownRegion(region))
	}

	for _, region := range []string{
		"us-east-100",
		"cn-north",
	} {
		require.False(t, IsKnownRegion(region))
	}
}

func TestIsValidRegion(t *testing.T) {
	tests := []struct {
		name         string
		inputRegions []string
		checkResult  require.BoolAssertionFunc
	}{
		{
			// If this test fails, validRegionRegex must be updated.
			name:         "known regions",
			inputRegions: GetKnownRegions(),
			checkResult:  require.True,
		},
		{
			name: "valid regions",
			inputRegions: []string{
				"us-gov-central-1",
				"xx-northwest-5",
			},
			checkResult: require.True,
		},
		{
			name: "invalid regions",
			inputRegions: []string{
				"x-east-1",
				"us-east-10",
				"us-nowhere-1",
				"us-xx-east-1",
			},
			checkResult: require.False,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, region := range test.inputRegions {
				test.checkResult(t, IsValidRegion(region), region)
			}
		})
	}

}
