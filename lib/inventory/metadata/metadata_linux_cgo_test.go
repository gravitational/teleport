//go:build linux
// +build linux

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

package metadata

import (
	"regexp"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestFetchOSVersion(t *testing.T) {
	t.Parallel()

	expectedFormatUbuntu := `
PRETTY_NAME="Ubuntu 22.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.1 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy
`

	expectedFormatDebian := `
PRETTY_NAME="Debian GNU/Linux 11 (bullseye)"
NAME="Debian GNU/Linux"
VERSION_ID="11"
VERSION="11 (bullseye)"
VERSION_CODENAME=bullseye
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	testCases := []struct {
		desc     string
		readFile func(string) ([]byte, error)
		expected string
	}{
		{
			desc: "combined NAME and VERSION_ID if /etc/os-release exists (ubuntu)",
			readFile: func(name string) ([]byte, error) {
				if name != "/etc/os-release" {
					return nil, trace.NotFound("file does not exist")
				}
				return []byte(expectedFormatUbuntu), nil
			},
			expected: "ubuntu 22.04",
		},
		{
			desc: "combined NAME and VERSION_ID if /etc/os-release exists (debian)",
			readFile: func(name string) ([]byte, error) {
				if name != "/etc/os-release" {
					return nil, trace.NotFound("file does not exist")
				}
				return []byte(expectedFormatDebian), nil
			},
			expected: "debian 11",
		},
		{
			desc: "empty if /etc/os-release does not exist",
			readFile: func(name string) ([]byte, error) {
				return nil, trace.NotFound("file does not exist")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				readFile: tc.readFile,
			}
			require.Equal(t, tc.expected, c.fetchOSVersion())
		})
	}
}

func TestFetchGlibcVersion(t *testing.T) {
	t.Parallel()

	matchVersion := regexp.MustCompile(`^\d+\.\d+$`)
	c := &fetchConfig{}
	require.True(t, matchVersion.MatchString(c.fetchGlibcVersion()))
}
