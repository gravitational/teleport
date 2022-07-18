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

package utils

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOSRelease(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		in       string
		expected map[string]string
		err      error
	}{
		{
			name: "amazon example, all quoted",
			in: `
NAME="Amazon Linux"
VERSION="2"
ID="amzn"
ID_LIKE="centos rhel fedora"
VERSION_ID="2"
PRETTY_NAME="Amazon Linux 2"
ANSI_COLOR="0;33"
CPE_NAME="cpe:2.3:o:amazon:amazon_linux:2"
HOME_URL="https://amazonlinux.com/"
`,
			expected: map[string]string{
				"NAME":        "Amazon Linux",
				"VERSION":     "2",
				"ID":          "amzn",
				"ID_LIKE":     "centos rhel fedora",
				"VERSION_ID":  "2",
				"PRETTY_NAME": "Amazon Linux 2",
				"ANSI_COLOR":  "0;33",
				"CPE_NAME":    "cpe:2.3:o:amazon:amazon_linux:2",
				"HOME_URL":    "https://amazonlinux.com/",
			},
			err: nil,
		},
		{
			name: "quoted and unquoted",
			in: `
PRETTY_NAME="Debian GNU/Linux bookworm/sid"
ID=debian
`,
			expected: map[string]string{
				"PRETTY_NAME": "Debian GNU/Linux bookworm/sid",
				"ID":          "debian",
			},
			err: nil,
		},
		{
			name: "improperly quoted",
			in:   `PRETTY_NAME="Debian GNU/Linux bookworm/sid`,
			err:  strconv.ErrSyntax,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(tc.in))
			got, err := OSRelease(reader)
			if err != nil {
				if tc.err != nil {
					require.ErrorIs(t, err, tc.err)
				} else {
					require.Failf(t, "unexpected error", err.Error())
				}
			}
			require.EqualValues(t, tc.expected, got)
		})
	}
}
