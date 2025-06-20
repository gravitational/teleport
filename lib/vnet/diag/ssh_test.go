// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package diag

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keypaths"
	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

// TestSSHDiag tests the SSH configuration diagnostic, specifically its ability
// to check whether an OpenSSH config file includes the VNet SSH config file.
func TestSSHDiag(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc        string
		profilePath string
		userHome    string
		isWindows   bool
		input       string
		expect      bool
	}{
		{
			desc:        "empty",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
		},
		{
			desc:        "macos tsh",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
			input:       `Include /Users/user/.tsh/vnet_ssh_config`,
			expect:      true,
		},
		{
			desc:        "macos tsh ~",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
			input:       `Include ~/.tsh/vnet_ssh_config`,
			expect:      true,
		},
		{
			desc:        "macos connect",
			profilePath: `/Users/user/Application Support/Teleport Connect/tsh`,
			userHome:    `/Users/user`,
			input:       `Include "/Users/user/Application Support/Teleport Connect/tsh/vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "macos connect ~",
			profilePath: `/Users/user/Application Support/Teleport Connect/tsh`,
			userHome:    `/Users/user`,
			input:       `Include "~/Application Support/Teleport Connect/tsh/vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "macos tsh not match connect",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
			input:       `Include "/Users/user/Application Support/Teleport Connect/tsh/vnet_ssh_config"`,
		},
		{
			desc:        "macos connect not match tsh",
			profilePath: `/Users/user/Application Support/Teleport Connect/tsh`,
			userHome:    `/Users/user`,
			input:       `Include /Users/user/.tsh/vnet_ssh_config`,
		},
		{
			desc:        "windows tsh",
			profilePath: `C:\Users\User\.tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:\\Users\\User\\.tsh\\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows tsh unescaped",
			profilePath: `C:\Users\User\.tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:\Users\User\.tsh\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows tsh unix path",
			profilePath: `C:\Users\User\.tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:/Users/User/.tsh/vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows tsh ~",
			profilePath: `C:\Users\User\.tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "~\\.tsh\\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows connect",
			profilePath: `C:\Users\User\AppData\Roaming\Teleport Connect\tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:\\Users\\User\\AppData\\Roaming\\Teleport\ Connect\\tsh\\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows connect unescaped",
			profilePath: `C:\Users\User\AppData\Roaming\Teleport Connect\tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:\Users\User\AppData\Roaming\Teleport Connect\tsh\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows connect unix path",
			profilePath: `C:\Users\User\AppData\Roaming\Teleport Connect\tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:/Users/User/AppData/Roaming/Teleport\ Connect/tsh/vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows connect ~",
			profilePath: `C:\Users\User\AppData\Roaming\Teleport Connect\tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "~\\AppData\\Roaming\\Teleport\ Connect\\tsh\\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows tsh not match connect",
			profilePath: `C:\Users\User\.tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:\\Users\\User\\AppData\\Roaming\\Teleport\ Connect\\tsh\\vnet_ssh_config"`,
		},
		{
			desc:        "windows connect not match tsh",
			profilePath: `C:\Users\User\AppData\Roaming\Teleport Connect\tsh`,
			userHome:    `C:\Users\User`,
			isWindows:   true,
			input:       `Include "C:\\Users\\User\\.tsh\\vnet_ssh_config"`,
		},
		{
			desc:        "some other file",
			profilePath: `/Users/user/.tsh`,
			input:       `Include /Users/user/.tsh/ssh_config`,
		},
		{
			desc:        "multiple includes",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
			input: `
Include ~/.ssh/include/*
Include /Users/user/ssh_config
Include /Users/user/.tsh/vnet_ssh_config
`,
			expect: true,
		},
		{
			desc:        "commented",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
			input:       `Include #/Users/user/.tsh/vnet_ssh_config`,
		},
		{
			desc:        "single quotes",
			profilePath: `/Users/user/.tsh`,
			userHome:    `/Users/user`,
			input:       `Include '/Users/user/.tsh/vnet_ssh_config'`,
			expect:      true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			diag, err := NewSSHDiag(&SSHConfig{
				ProfilePath: tc.profilePath,
			})
			require.NoError(t, err)
			userOpenSSHConfigPath := filepath.Join(t.TempDir(), "test_ssh_config")

			// Override isWindows and paths for the purpose of the test.
			diag.isWindows = tc.isWindows
			diag.userHome = tc.userHome
			diag.userOpenSSHConfigPath = userOpenSSHConfigPath

			if len(tc.input) > 0 {
				require.NoError(t, os.WriteFile(userOpenSSHConfigPath, []byte(tc.input), 0o600))
			}

			expectReport := &diagv1.CheckReport{
				Status: diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK,
				Report: &diagv1.CheckReport_SshConfigurationReport{
					SshConfigurationReport: &diagv1.SSHConfigurationReport{
						UserOpensshConfigPath:                  userOpenSSHConfigPath,
						VnetSshConfigPath:                      keypaths.VNetSSHConfigPath(tc.profilePath),
						UserOpensshConfigIncludesVnetSshConfig: tc.expect,
						UserOpensshConfigExists:                len(tc.input) > 0,
						UserOpensshConfigContents:              tc.input,
					},
				},
			}

			report, err := diag.Run(t.Context())
			require.NoError(t, err)
			require.Equal(t, expectReport, report)
		})
	}
}
