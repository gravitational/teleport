/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestExtractAWSStartURL(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "URL with anchor",
			input:    "https://d-92670253d5.awsapps.com/start/#/console?param=value",
			expected: "https://d-92670253d5.awsapps.com/start",
		},
		{
			desc:     "URL without anchor",
			input:    "https://test.awsapps.com/start",
			expected: "https://test.awsapps.com/start",
		},
		{
			desc:     "Random URL",
			input:    "https://aws.amazon.com",
			expected: "https://aws.amazon.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, extractAWSStartURL(tc.input))
		})
	}
}

func TestExtractAWSSessionName(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "Standard AWS Identify Center URL",
			input:    "https://d-92670253d5.awsapps.com/start",
			expected: "teleport-d-92670253d5",
		},
		{
			desc:     "URL with single subdomain",
			input:    "https://mycompany.awsapps.com/start",
			expected: "teleport-mycompany",
		},
		{
			desc:     "URL without subdomain subdomain (rare)",
			input:    "https://awsapps.com/start",
			expected: "teleport-awsapps",
		},
		{
			desc:     "Fallback to hash for non-standard URL",
			input:    "https://unknown-format",
			expected: "teleport-95924c5", // SHA256 prefix of "https://unknown-format"
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, extractAWSSessionName(tc.input))
		})
	}
}

func TestFormatAWSProfileName(t *testing.T) {
	testCases := []struct {
		desc        string
		accountName string
		roleName    string
		expected    string
	}{
		{
			desc:        "Standard lowercase",
			accountName: "teleport-dev",
			roleName:    "Admin",
			expected:    "teleport-awsic-teleport-dev-admin",
		},
		{
			desc:        "Case sensitivity check",
			accountName: "Production-Account",
			roleName:    "PowerUser",
			expected:    "teleport-awsic-production-account-poweruser",
		},
		{
			desc:        "Account ID fallback",
			accountName: "123456789012",
			roleName:    "ReadOnly",
			expected:    "teleport-awsic-123456789012-readonly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, formatAWSProfileName(tc.accountName, tc.roleName))
		})
	}
}

func TestWriteAWSProfileSummary(t *testing.T) {
	configPath := "/home/user/.aws/config"
	profiles := []awsProfileInfo{
		{
			profile:   "teleport-awsic-dev-admin",
			account:   "dev",
			accountID: "123456789012",
			role:      "Admin",
		},
		{
			profile:   "teleport-awsic-prod-reader",
			account:   "prod",
			accountID: "098765432109",
			role:      "Reader",
		},
	}

	buf := &bytes.Buffer{}
	writeAWSProfileSummary(buf, configPath, profiles)
	output := buf.String()

	require.Contains(t, output, "Wrote /home/user/.aws/config.")
	require.Contains(t, output, "teleport-awsic-dev-admin")
	require.Contains(t, output, "123456789012")
	require.Contains(t, output, "Admin")
	require.Contains(t, output, "teleport-awsic-prod-reader")
	require.Contains(t, output, "098765432109")
	require.Contains(t, output, "Reader")

	// Empty case
	buf.Reset()
	writeAWSProfileSummary(buf, configPath, nil)
	require.Contains(t, buf.String(), "No AWS Identity Center integrations found.")
}

func TestWriteAWSConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	app1, err := types.NewAppV3(types.Metadata{
		Name: "app1",
		Labels: map[string]string{
			"teleport.dev/account-name": "dev",
		},
	}, types.AppSpecV3{
		URI: "https://d-123.awsapps.com/start/#/",
		IdentityCenter: &types.AppIdentityCenter{
			AccountID: "111111111111",
			PermissionSets: []*types.IdentityCenterPermissionSet{
				{Name: "Admin"},
				{Name: "Reader"},
			},
		},
	})
	require.NoError(t, err)

	app2, err := types.NewAppV3(types.Metadata{
		Name: "app2",
	}, types.AppSpecV3{
		URI: "https://d-123.awsapps.com/start",
		IdentityCenter: &types.AppIdentityCenter{
			AccountID: "222222222222",
			PermissionSets: []*types.IdentityCenterPermissionSet{
				{Name: "Admin"},
			},
		},
	})
	require.NoError(t, err)

	resources := types.EnrichedResources{
		{
			ResourceWithLabels: &types.AppServerV3{
				Spec: types.AppServerSpecV3{
					App: app1,
				},
			},
		},
		{
			ResourceWithLabels: &types.AppServerV3{
				Spec: types.AppServerSpecV3{
					App: app2,
				},
			},
		},
	}

	written, err := writeAWSConfig(configPath, "us-east-1", resources)
	require.NoError(t, err)
	require.Len(t, written, 3)

	// Verify the returned slice items
	require.Equal(t, "teleport-awsic-dev-admin", written[0].profile)
	require.Equal(t, "111111111111", written[0].accountID)
	require.Equal(t, "Admin", written[0].role)

	require.Equal(t, "teleport-awsic-dev-reader", written[1].profile)
	require.Equal(t, "111111111111", written[1].accountID)
	require.Equal(t, "Reader", written[1].role)

	require.Equal(t, "teleport-awsic-222222222222-admin", written[2].profile)
	require.Equal(t, "222222222222", written[2].accountID)
	require.Equal(t, "Admin", written[2].role)

	// Verify file content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	s := string(content)

	// Check session
	require.Contains(t, s, "[sso-session teleport-d-123]")
	require.Contains(t, s, "sso_start_url=https://d-123.awsapps.com/start")
	require.Contains(t, s, "sso_region=us-east-1")

	// Check all profiles in file
	expectedProfiles := []struct {
		name    string
		account string
		role    string
	}{
		{"teleport-awsic-dev-admin", "111111111111", "Admin"},
		{"teleport-awsic-dev-reader", "111111111111", "Reader"},
		{"teleport-awsic-222222222222-admin", "222222222222", "Admin"},
	}

	for _, p := range expectedProfiles {
		require.Contains(t, s, fmt.Sprintf("[profile %s]", p.name))
		require.Contains(t, s, fmt.Sprintf("sso_account_id=%s", p.account))
		require.Contains(t, s, fmt.Sprintf("sso_role_name=%s", p.role))
		require.Contains(t, s, "sso_session=teleport-d-123")
	}
}
