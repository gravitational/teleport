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
			expected: "Ubuntu 22.04",
		},
		{
			desc: "combined NAME and VERSION_ID if /etc/os-release exists (debian)",
			readFile: func(name string) ([]byte, error) {
				if name != "/etc/os-release" {
					return nil, trace.NotFound("file does not exist")
				}
				return []byte(expectedFormatDebian), nil
			},
			expected: "Debian GNU/Linux 11",
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
			c := &metadataFetchConfig{
				readFile: tc.readFile,
			}
			require.Equal(t, tc.expected, c.fetchOSVersion())
		})
	}
}

func TestFetchGlibcVersion(t *testing.T) {
	t.Parallel()

	expectedFormat := `
ldd (Debian GLIBC 2.31-13+deb11u5) 2.31
Copyright (C) 2020 Free Software Foundation, Inc.
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
Written by Roland McGrath and Ulrich Drepper.
	`

	testCases := []struct {
		desc        string
		execCommand func(string, ...string) ([]byte, error)
		expected    string
	}{
		{
			desc: "last word of first line if ldd exists",
			execCommand: func(name string, args ...string) ([]byte, error) {
				if name != "ldd" {
					return nil, trace.NotFound("command does not exist")
				}
				if len(args) != 1 {
					return nil, trace.Errorf("invalid command argument")
				}

				switch args[0] {
				case "--version":
					return []byte(expectedFormat), nil
				default:
					return nil, trace.Errorf("invalid command argument")
				}
			},
			expected: "2.31",
		},
		{
			desc: "empty if ldd does not exist",
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &metadataFetchConfig{
				execCommand: tc.execCommand,
			}
			require.Equal(t, tc.expected, c.fetchGlibcVersion())
		})
	}
}
