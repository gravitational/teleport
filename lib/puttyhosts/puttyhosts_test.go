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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddHostToHostList(t *testing.T) {
	var tests = []struct {
		hostList []string
		hostname string
		expected []string
	}{
		{
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
			hostList: []string{
				"*.example.com",
			},
			hostname: "one.example.com",
			expected: []string{
				"*.example.com",
			},
		},
		{
			hostList: []string{
				"one.example.com",
			},
			hostname: "two.example.com",
			expected: []string{
				"*.example.com",
			},
		},
		{
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
		{
			hostList: []string{
				"one",
				"two",
				"three.example.com",
				"four.example.com",
				"five.test.com",
			},
			hostname: "six",
			expected: []string{
				"one",
				"two",
				"*.example.com",
				"five.test.com",
				"six",
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			output := AddHostToHostList(tt.hostList, tt.hostname)
			require.ElementsMatch(t, tt.expected, output)
		})
	}
}

func TestFormatLocalCommandString(t *testing.T) {
	var tests = []struct {
		TSHPath        string
		clusterName    string
		expectedOutput string
	}{
		{
			TSHPath:        `C:\Users\Test\tsh.exe`,
			clusterName:    `teleport.example.com`,
			expectedOutput: `C:\\Users\\Test\\tsh.exe proxy ssh --cluster=teleport.example.com --proxy=%proxyhost %user@%host:%port`,
		},
		{
			TSHPath:        `Z:\localdata\installation path with spaces\teleport\tsh-v13.1.3.exe`,
			clusterName:    `long-cluster-name-that-isnt-an-fqdn`,
			expectedOutput: `Z:\\localdata\\installation path with spaces\\teleport\\tsh-v13.1.3.exe proxy ssh --cluster=long-cluster-name-that-isnt-an-fqdn --proxy=%proxyhost %user@%host:%port`,
		},
		{
			TSHPath:        `\\SERVER01\UNC\someotherpath\gravitational-teleport-tsh-embedded.exe`,
			clusterName:    `bigcorp.co1fqdn01.ad.enterpriseydomain.local`,
			expectedOutput: `\\\\SERVER01\\UNC\\someotherpath\\gravitational-teleport-tsh-embedded.exe proxy ssh --cluster=bigcorp.co1fqdn01.ad.enterpriseydomain.local --proxy=%proxyhost %user@%host:%port`,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			output, err := FormatLocalCommandString(tt.TSHPath, tt.clusterName)
			require.Equal(t, tt.expectedOutput, output)
			require.NoError(t, err)
		})
	}
}
