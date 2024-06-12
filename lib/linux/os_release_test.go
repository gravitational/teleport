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

package linux_test

import (
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/linux"
)

func TestParseOSReleaseFromReader(t *testing.T) {
	tests := []struct {
		name string
		in   io.Reader
		want *linux.OSRelease
	}{
		{
			name: "Ubuntu 22.04",
			in: strings.NewReader(`PRETTY_NAME="Ubuntu 22.04.3 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy
`),
			want: &linux.OSRelease{
				PrettyName:      "Ubuntu 22.04.3 LTS",
				Name:            "Ubuntu",
				VersionID:       "22.04",
				Version:         "22.04.3 LTS (Jammy Jellyfish)",
				ID:              "ubuntu",
				VersionCodename: "jammy",
				IDLike:          "debian",
			},
		},
		{
			name: "invalid lines ignored",
			in: strings.NewReader(`
ID=fake

// not supposed to be here

not supposed to be here either

a=b=c

# a comment

VERSION=1.0
`),
			want: &linux.OSRelease{
				Version: "1.0",
				ID:      "fake",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := linux.ParseOSReleaseFromReader(test.in)
			require.NoError(t, err, "ParseOSReleaseFromReader")

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("ParseOSReleaseFromReader mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
