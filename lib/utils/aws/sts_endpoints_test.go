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

func TestGetValidSTSEndpoints(t *testing.T) {
	// The number of endpoints may grow over time when SDK is updated. So here
	// just tests if GetValidSTSEndpoints contains entries from this selective
	// list.
	wantEndoints := append(
		[]string{
			"sts.af-south-1.amazonaws.com",
			"sts.amazonaws.com",
			"sts.ap-east-1.amazonaws.com",
			"sts.cn-north-1.amazonaws.com.cn",
			"sts.cn-northwest-1.amazonaws.com.cn",
			"sts.il-central-1.amazonaws.com",
			"sts.us-gov-west-1.amazonaws.com",
			"sts.us-iso-east-1.c2s.ic.gov",
			"sts.us-west-1.amazonaws.com",
		},
		GetSTSFipsEndpoints()...,
	)
	for _, wantEndpoint := range wantEndoints {
		require.Contains(t, GetValidSTSEndpoints(), wantEndpoint)
	}
}

func TestGetSTSFipsEndpoints(t *testing.T) {
	wantEndoints := []string{
		"sts-fips.us-east-1.amazonaws.com",
		"sts-fips.us-east-2.amazonaws.com",
		"sts-fips.us-west-1.amazonaws.com",
		"sts-fips.us-west-2.amazonaws.com",
		"sts.us-gov-east-1.amazonaws.com",
		"sts.us-gov-west-1.amazonaws.com",
	}
	for _, wantEndpoint := range wantEndoints {
		require.Contains(t, GetSTSFipsEndpoints(), wantEndpoint)
	}
}

func Test_combinations(t *testing.T) {
	require.Nil(
		t,
		combinations[string](nil),
	)

	require.Equal(
		t,
		[][]string{
			{"a"},
		},
		combinations([]string{"a"}),
	)

	require.Equal(
		t,
		[][]string{
			{"a"}, {"b"}, {"a", "b"},
			{"c"}, {"a", "c"}, {"b", "c"},
			{"a", "b", "c"},
		},
		combinations([]string{"a", "b", "c"}),
	)
}
