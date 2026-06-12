//go:build linux
// +build linux

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
