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

package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShortRegionToRegion(t *testing.T) {
	t.Run("valid regions", func(t *testing.T) {
		t.Parallel()

		validRegionMap := map[string]string{
			"use1":  "us-east-1",
			"use2":  "us-east-2",
			"usw1":  "us-west-1",
			"usw2":  "us-west-2",
			"cac1":  "ca-central-1",
			"euw1":  "eu-west-1",
			"euw2":  "eu-west-2",
			"euw3":  "eu-west-3",
			"euc1":  "eu-central-1",
			"eus1":  "eu-south-1",
			"eun1":  "eu-north-1",
			"apse1": "ap-southeast-1",
			"apse2": "ap-southeast-2",
			"aps1":  "ap-south-1",
			"apne1": "ap-northeast-1",
			"apne2": "ap-northeast-2",
			"ape1":  "ap-east-1",
			"sae1":  "sa-east-1",
			"afs1":  "af-south-1",
			"usgw1": "us-gov-west-1",
			"usge1": "us-gov-east-1",
			"cnn1":  "cn-north-1",
			"cnnw1": "cn-northwest-1",
		}

		for shortRegion, expectRegion := range validRegionMap {
			actualRegion, ok := ShortRegionToRegion(shortRegion)
			require.True(t, ok)
			require.Equal(t, expectRegion, actualRegion)
		}
	})

	invalidTests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid prefix",
			input: "u",
		},
		{
			name:  "not ended in number",
			input: "use1b",
		},
		{
			name:  "invalid direction",
			input: "usx1",
		},
	}
	for _, test := range invalidTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, ok := ShortRegionToRegion(test.input)
			require.False(t, ok)
		})
	}
}
