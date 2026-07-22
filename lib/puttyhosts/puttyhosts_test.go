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
					`AAAAB3NzaC1yc2EAAAADAQABAAABAQDNbSbDa+bAjeH6wQPMfcUoyKHOTOwBRc1Lr+5Vy6aHOz+lWsovldH0r4mGFv2mLyWmqax18YVWG/YY+5um9y19SxlIHcAZI/uqnV7lAOhVkni87CGZ+Noww512dlrtczYZDc4735mSYxcSYQyRZywwXOfSqA0Euc6P2a0e03hcdROeJxx50xQcDw/wjreot5swiVHOvOGIIauekPswP58Z+F4goIFaFk5i5gDDBfX4mvtFV5AOkYQlk4hzmwJZ2JpphUQ33YbwhDrEPat2/mLf1tUk6aY8qHFqE9g5bjFjuLQxeva3Y5in49Zt+pg701TbBwS+R8wbuQqDM8b7VgEV`, // RSA
					`AAAAB3NzaC1yc2EAAAADAQABAAABAQDm0PWl5llSpFArdHkXv8xXgsO9qEAbjvIAjMaoUbr79d03pBlmCCU7Zm3X9NkiLL7om2KLSE7AA0oQI+S+VgrDX17S327uj8M3hNZkfkbKGvzY5NS17DubpEEuAoF1r8Of7GKMbAmQ9d8dF8iNkREaJ+FT8g2JmGtRwmQGf8c0v2FCdz7SbChE9nUxk4Q8f1Qjhx8Pgjga/ntqkB+JpwATVvCxkd/ld0yzh9T0l90dV1TYYwnmWVpQzes1nbotQoMK8vUO20dWBEMWVMxXXp/P4OaztYGLmGJ9YP9upxq8IoSUdef7URUuJZGPWEyCQ0Mk6GRYJHvlX5cNOSHxYDBt`, // RSA
					`AAAAC3NzaC1lZDI1NTE5AAAAICj/inr+V2oDyH39iESDof/jM4XcPzUZOVZ/Bm79CVGi`,                                                                         // Ed25519
					`AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBJp4V4vuk5BjiOXKhls02lsw61OZFhZ9Ya188inproU5FmaUhjYnjEvsGPLeMYu3o2AQ4/gsV6MW2H1bNnr5SvY=`, // ECDSA
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
					HostCAPublicKeyForRegistry{
						KeyName:   "TeleportHostCA-teleport.example.com-2",
						PublicKey: "AAAAC3NzaC1lZDI1NTE5AAAAICj/inr+V2oDyH39iESDof/jM4XcPzUZOVZ/Bm79CVGi",
						Hostname:  "test-hostname.example.com",
					},
					HostCAPublicKeyForRegistry{
						KeyName:   "TeleportHostCA-teleport.example.com-3",
						PublicKey: "AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBJp4V4vuk5BjiOXKhls02lsw61OZFhZ9Ya188inproU5FmaUhjYnjEvsGPLeMYu3o2AQ4/gsV6MW2H1bNnr5SvY=",
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
			hostname:   "test",
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
		{
			hostname:   "host:22",
			shouldPass: false,
		},
		{
			hostname:   "host with spaces",
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

func TestCheckAndSplitValidityKey(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		name          string
		input         string
		desiredOutput []string
		checkErr      require.ErrorAssertionFunc
	}{
		{
			name:          "Should pass with an empty input string",
			input:         "",
			desiredOutput: []string(nil),
			checkErr:      require.NoError,
		},
		{
			name:  "Should pass with two wildcards",
			input: "*.foo.example.com || *.bar.example.com",
			desiredOutput: []string{
				"*.foo.example.com",
				"*.bar.example.com",
			},
			checkErr: require.NoError,
		},
		{
			name:          "Should pass with a single string and no delimiter",
			input:         "test",
			desiredOutput: []string{"test"},
			checkErr:      require.NoError,
		},
		{
			name:  "Should pass with wildcard, single string and regular hostname",
			input: "*.example.com || test || teleport.test.com",
			desiredOutput: []string{
				"*.example.com",
				"test",
				"teleport.test.com",
			},
			checkErr: require.NoError,
		},
		{
			name:  "Should pass with mixed usage",
			input: "*.example.com || test || teleport.test.com || longstring || *.wow.com",
			desiredOutput: []string{
				"*.example.com",
				"test",
				"teleport.test.com",
				"longstring",
				"*.wow.com",
			},
			checkErr: require.NoError,
		},
		{
			name:  "Should pass with trailing space",
			input: "*.example.com || lol.example.com || test.teleport.com ",
			desiredOutput: []string{
				"*.example.com",
				"lol.example.com",
				"test.teleport.com",
			},
			checkErr: require.NoError,
		},
		{
			name:  "Should pass with preceding space",
			input: " *.example.com || lol.example.com",
			desiredOutput: []string{
				"*.example.com",
				"lol.example.com",
			},
			checkErr: require.NoError,
		},
		{
			name:  "Should pass with random spacing",
			input: " *.example.com  ||   lol.example.com",
			desiredOutput: []string{
				"*.example.com",
				"lol.example.com",
			},
			checkErr: require.NoError,
		},
		{
			name:  "Should pass with extra space in the middle",
			input: " *.example.com ||  lol.example.com || test.teleport.com",
			desiredOutput: []string{
				"*.example.com",
				"lol.example.com",
				"test.teleport.com",
			},
			checkErr: require.NoError,
		},
		{
			name:     "Should error if colons are used",
			input:    "*.example.com && port:22",
			checkErr: require.Error,
		},
		{
			name:     "Should fail if negation is used",
			input:    "*.example.com && ! *.extrasecure.example.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail if parentheses are used",
			input:    "(*.foo.example.com || *.bar.example.com)",
			checkErr: require.Error,
		},
		{
			name:     "Should fail if parentheses and port are used",
			input:    "(*.foo.example.com || *.bar.example.com) && port:0-1023",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with multiple parentheses and port",
			input:    "(*.foo.example.com || *.bar.example.com || *.qux.example.com) && port:0-1023",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with multiple parentheses, port and trailing hostname",
			input:    "((*.foo.example.com || *.bar.example.com || *.qux.example.com) && port:0-1023) || teleport.example.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with multiple parentheses, port and two trailing hostnames",
			input:    "((*.example.com || lol.example.com && port:22) && port:1024) || test.teleport.com || teleport.test.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail if single pipe delimiter is used",
			input:    "*.example.com || lol.example.com | test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail if single ampersand delimiter is used",
			input:    "*.example.com & lol.example.com || test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail if boolean AND is used",
			input:    "*.example.com && lol.example.com || test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with misplaced pipe delimiter",
			input:    "*.example.com || lol.example.com || | test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with empty final field",
			input:    "*.example.com || lol.example.com || | test.teleport.com || ",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with double delimiter and empty field",
			input:    "*.example.com || lol.example.com || || test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with triple delimiter and empty field",
			input:    "*.example.com || lol.example.com || || || test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with pipe character in hostname",
			input:    "*.example.com || lol.example.com || test.|teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with negation character around hostname",
			input:    "*.example.com || lol.example.com || !test.teleport.com",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with a non-hostname",
			input:    "*.example.com || lol.example.com || test.teleport.com || \"\"",
			checkErr: require.Error,
		},
		{
			name:     "Should fail with a single trailing dot",
			input:    "*.example.com || lol.example.com || .",
			checkErr: require.Error,
		},
		{
			name:     "Should error with single wildcard symbol",
			input:    "*",
			checkErr: require.Error,
		},
		{
			name:     "Should error if multiple single wildcard symbols are present",
			input:    "* || *",
			checkErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testResult, err := CheckAndSplitValidityKey(tt.input, "TeleportHostCA-testcluster.example.com")
			tt.checkErr(t, err)
			require.Equal(t, tt.desiredOutput, testResult)
		})
	}
}
