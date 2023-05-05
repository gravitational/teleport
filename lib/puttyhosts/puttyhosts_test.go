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

package puttyhosts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddHostToHostList(t *testing.T) {
	var tests = []struct {
		id       string
		hostList []string
		hostname string
		expected []string
	}{
		{
			id: "test case 1",
			hostList: []string{
				"one.example.com",
				"two.example.com",
			},
			hostname: "three.example.com",
			expected: []string{
				"*.example.com",
			},
		},
		{
			id: "test case 2",
			hostList: []string{
				"one",
				"two",
			},
			hostname: "three",
			expected: []string{
				"one",
				"two",
				"three",
			},
		},
		{
			id: "test case 3",
			hostList: []string{
				"*.example.com",
			},
			hostname: "one.example.com",
			expected: []string{
				"*.example.com",
			},
		},
		{
			id: "test case 4",
			hostList: []string{
				"one.example.com",
			},
			hostname: "two.example.com",
			expected: []string{
				"*.example.com",
			},
		},
		{
			id: "test case 5",
			hostList: []string{
				"one.alpha.example.com",
				"two.beta.example.com",
				"three.beta.example.com",
			},
			hostname: "four.charlie.example.com",
			expected: []string{
				"one.alpha.example.com",
				"*.beta.example.com",
				"four.charlie.example.com",
			},
		},
		{
			id: "test case 6",
			hostList: []string{
				"one.alpha.example.com",
				"two.beta.example.com",
				"three.beta.example.com",
			},
			hostname: "four",
			expected: []string{
				"one.alpha.example.com",
				"*.beta.example.com",
				"four",
			},
		},
		{
			id: "test case 7",
			hostList: []string{
				"eggs.breakfast",
				"bacon.breakfast",
				"mimosa.breakfast",
				"salad.lunch",
			},
			hostname: "soup.lunch",
			expected: []string{
				"*.breakfast",
				"*.lunch",
			},
		},
		{
			id: "test case 8",
			hostList: []string{
				"*.breakfast",
				"*.lunch",
				"fish.dinner",
				"chips.dinner",
			},
			hostname: "apple.dessert",
			expected: []string{
				"*.breakfast",
				"*.lunch",
				"*.dinner",
				"apple.dessert",
			},
		},
	}

	for _, tt := range tests {
		output := AddHostToHostList(tt.hostList, tt.hostname)
		require.ElementsMatch(t, tt.expected, output, tt.id)
	}
}
