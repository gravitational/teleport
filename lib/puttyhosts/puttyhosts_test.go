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
	t.Parallel()

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
	t.Parallel()

	var tests = []struct {
		PuttyProxyTelnetCommandArgs
		expectedOutput string
	}{
		{
			PuttyProxyTelnetCommandArgs{
				TSHPath: `C:\Users\Test\tsh.exe`,
				Cluster: `teleport.example.com`,
			},
			`C:\\Users\\Test\\tsh.exe proxy ssh --cluster=teleport.example.com --proxy=%proxyhost %user@%host:%port`,
		},
		{
			PuttyProxyTelnetCommandArgs{
				TSHPath: `Z:\localdata\installation path with spaces\teleport\tsh-v13.1.3.exe`,
				Cluster: `long-cluster-name-that-isnt-an-fqdn`,
			},
			`Z:\\localdata\\installation path with spaces\\teleport\\tsh-v13.1.3.exe proxy ssh --cluster=long-cluster-name-that-isnt-an-fqdn --proxy=%proxyhost %user@%host:%port`,
		},
		{
			PuttyProxyTelnetCommandArgs{
				TSHPath: `\\SERVER01\UNC\someotherpath\gravitational-teleport-tsh-embedded.exe`,
				Cluster: `bigcorp.co1fqdn01.ad.enterpriseydomain.local`,
			},
			`\\\\SERVER01\\UNC\\someotherpath\\gravitational-teleport-tsh-embedded.exe proxy ssh --cluster=bigcorp.co1fqdn01.ad.enterpriseydomain.local --proxy=%proxyhost %user@%host:%port`,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			output, err := FormatLocalCommandString(tt.TSHPath, tt.Cluster)
			require.Equal(t, tt.expectedOutput, output)
			require.NoError(t, err)
		})
	}
}

func TestFormatHostCAPublicKeysForRegistry(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		inputMap       map[string][]string
		hostname       string
		expectedOutput map[string][]HostCAPublicKeyForRegistry
	}{
		{
			inputMap: map[string][]string{
				"teleport.example.com": {
					`AAAAB3NzaC1yc2EAAAADAQABAAABAQDNbSbDa+bAjeH6wQPMfcUoyKHOTOwBRc1Lr+5Vy6aHOz+lWsovldH0r4mGFv2mLyWmqax18YVWG/YY+5um9y19SxlIHcAZI/uqnV7lAOhVkni87CGZ+Noww512dlrtczYZDc4735mSYxcSYQyRZywwXOfSqA0Euc6P2a0e03hcdROeJxx50xQcDw/wjreot5swiVHOvOGIIauekPswP58Z+F4goIFaFk5i5gDDBfX4mvtFV5AOkYQlk4hzmwJZ2JpphUQ33YbwhDrEPat2/mLf1tUk6aY8qHFqE9g5bjFjuLQxeva3Y5in49Zt+pg701TbBwS+R8wbuQqDM8b7VgEV`,
					`AAAAB3NzaC1yc2EAAAADAQABAAABAQDm0PWl5llSpFArdHkXv8xXgsO9qEAbjvIAjMaoUbr79d03pBlmCCU7Zm3X9NkiLL7om2KLSE7AA0oQI+S+VgrDX17S327uj8M3hNZkfkbKGvzY5NS17DubpEEuAoF1r8Of7GKMbAmQ9d8dF8iNkREaJ+FT8g2JmGtRwmQGf8c0v2FCdz7SbChE9nUxk4Q8f1Qjhx8Pgjga/ntqkB+JpwATVvCxkd/ld0yzh9T0l90dV1TYYwnmWVpQzes1nbotQoMK8vUO20dWBEMWVMxXXp/P4OaztYGLmGJ9YP9upxq8IoSUdef7URUuJZGPWEyCQ0Mk6GRYJHvlX5cNOSHxYDBt`,
				},
			},
			hostname: "test-hostname.example.com",
			expectedOutput: map[string][]HostCAPublicKeyForRegistry{
				"teleport.example.com": {
					HostCAPublicKeyForRegistry{
						KeyName:   "TeleportHostCA-teleport.example.com-0",
						PublicKey: "AAAAB3NzaC1yc2EAAAADAQABAAABAQDNbSbDa+bAjeH6wQPMfcUoyKHOTOwBRc1Lr+5Vy6aHOz+lWsovldH0r4mGFv2mLyWmqax18YVWG/YY+5um9y19SxlIHcAZI/uqnV7lAOhVkni87CGZ+Noww512dlrtczYZDc4735mSYxcSYQyRZywwXOfSqA0Euc6P2a0e03hcdROeJxx50xQcDw/wjreot5swiVHOvOGIIauekPswP58Z+F4goIFaFk5i5gDDBfX4mvtFV5AOkYQlk4hzmwJZ2JpphUQ33YbwhDrEPat2/mLf1tUk6aY8qHFqE9g5bjFjuLQxeva3Y5in49Zt+pg701TbBwS+R8wbuQqDM8b7VgEV",
						Hostname:  "test-hostname.example.com",
					},
					HostCAPublicKeyForRegistry{
						KeyName:   "TeleportHostCA-teleport.example.com-1",
						PublicKey: "AAAAB3NzaC1yc2EAAAADAQABAAABAQDm0PWl5llSpFArdHkXv8xXgsO9qEAbjvIAjMaoUbr79d03pBlmCCU7Zm3X9NkiLL7om2KLSE7AA0oQI+S+VgrDX17S327uj8M3hNZkfkbKGvzY5NS17DubpEEuAoF1r8Of7GKMbAmQ9d8dF8iNkREaJ+FT8g2JmGtRwmQGf8c0v2FCdz7SbChE9nUxk4Q8f1Qjhx8Pgjga/ntqkB+JpwATVvCxkd/ld0yzh9T0l90dV1TYYwnmWVpQzes1nbotQoMK8vUO20dWBEMWVMxXXp/P4OaztYGLmGJ9YP9upxq8IoSUdef7URUuJZGPWEyCQ0Mk6GRYJHvlX5cNOSHxYDBt",
						Hostname:  "test-hostname.example.com",
					},
				},
			},
		},
		{
			inputMap: map[string][]string{
				"testClusterTwo": {
					`AAAAB3NzaC1yc2EAAAADAQABAAABAQC09sJMb0CHzA8S/bYzHIsP1SgkwMD5QYOLqWhx8skWpheUZK7rTjW4y254CgLIcgGtsYyRdROs1F7IChAqfn9afCSz2a4o9tZiGXdUDw9mCB54aYF/l3WST8y+TOApSaq2Aduxagm4VlWTtohdEIKVphm7l6Dp3kTz2llQ+0qmV338d8InaEFXXhVhfOZ0/erLuFllMkeMQ66R7yjNyubY/bZy3PMF2Miv7VfX8SXAgkkS40v1esHxS26NnyD3l3MwXh99peoYQcDevq6EwYMmKSvdHgcUT+Sm9LJx48+n6ejHTUOZw2E64I26LD6PiIoFavyWVSPN/06W6n1gvmbb`,
				},
			},
			hostname: "some-other-test-host",
			expectedOutput: map[string][]HostCAPublicKeyForRegistry{
				"testClusterTwo": {
					HostCAPublicKeyForRegistry{
						KeyName:   "TeleportHostCA-testClusterTwo",
						PublicKey: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC09sJMb0CHzA8S/bYzHIsP1SgkwMD5QYOLqWhx8skWpheUZK7rTjW4y254CgLIcgGtsYyRdROs1F7IChAqfn9afCSz2a4o9tZiGXdUDw9mCB54aYF/l3WST8y+TOApSaq2Aduxagm4VlWTtohdEIKVphm7l6Dp3kTz2llQ+0qmV338d8InaEFXXhVhfOZ0/erLuFllMkeMQ66R7yjNyubY/bZy3PMF2Miv7VfX8SXAgkkS40v1esHxS26NnyD3l3MwXh99peoYQcDevq6EwYMmKSvdHgcUT+Sm9LJx48+n6ejHTUOZw2E64I26LD6PiIoFavyWVSPN/06W6n1gvmbb",
						Hostname:  "some-other-test-host",
					},
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			testOutput := FormatHostCAPublicKeysForRegistry(tt.inputMap, tt.hostname)
			require.Equal(t, tt.expectedOutput, testOutput)
		})
	}
}

func TestNaivelyValidateHostname(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		hostname   string
		shouldPass bool
	}{
		{
			hostname:   "teleport.example.com",
			shouldPass: true,
		},
		{
			hostname:   "hostname",
			shouldPass: true,
		},
		{
			hostname:   "testhost-withdashes.example.com",
			shouldPass: true,
		},
		{
			hostname:   "itendswithnumbers0123",
			shouldPass: true,
		},
		{
			hostname:   "0123itstartswithnumbers",
			shouldPass: true,
		},
		{
			hostname:   "hostname-",
			shouldPass: false,
		},
		{
			hostname:   "general.",
			shouldPass: false,
		},
		{
			hostname:   "-startswithadash",
			shouldPass: false,
		},
		{
			hostname:   "endswithadash-",
			shouldPass: false,
		},
		{
			hostname:   "consecutive..dots",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			testResult := NaivelyValidateHostname(tt.hostname)
			require.Equal(t, tt.shouldPass, testResult)
		})
	}
}
