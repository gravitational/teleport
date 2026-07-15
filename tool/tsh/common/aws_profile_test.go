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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/aws/awsconfigfile"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
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
			desc:     "URL with subpath but no anchor",
			input:    "https://d-92670253d5.awsapps.com/start/console",
			expected: "https://d-92670253d5.awsapps.com/start",
		},
		{
			desc:     "GovCloud URL",
			input:    "https://start.us-gov-home.awsapps.com/directory/d-92671f2def#/console?account_id=987654321098",
			expected: "https://start.us-gov-home.awsapps.com/directory/d-92671f2def",
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
			desc:     "GovCloud URL",
			input:    "https://start.us-gov-home.awsapps.com/directory/d-92671f2def",
			expected: "teleport-d-92671f2def",
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
			desc:        "Account with space",
			accountName: "Production Account",
			roleName:    "ReadOnly",
			expected:    "teleport-awsic-production-account-readonly",
		},
		{
			desc:        "Role with space",
			accountName: "QA",
			roleName:    "System Administrator",
			expected:    "teleport-awsic-qa-system-administrator",
		},
		{
			desc:        "Account ID fallback",
			accountName: "123456789012",
			roleName:    "ReadOnly",
			expected:    "teleport-awsic-123456789012-readonly",
		},
		{
			desc:        "Malicious INI injection characters",
			accountName: "malicious \n[profile default]\n",
			roleName:    "admin [!]",
			expected:    "teleport-awsic-malicious-profile-default-admin-",
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
	profiles := []awsconfigfile.SSOProfile{
		{
			Name:      "teleport-awsic-dev-admin",
			AccountID: "123456789012",
			RoleName:  "Admin",
			Session:   "teleport-d-12345",
			Account:   "dev",
		},
		{
			Name:      "teleport-awsic-prod-reader",
			AccountID: "098765432109",
			RoleName:  "Reader",
			Session:   "teleport-d-12345",
			Account:   "prod",
		},
	}

	t.Run("with profiles", func(t *testing.T) {
		buf := &bytes.Buffer{}
		writeAWSProfileSummary(buf, configPath, profiles)
		output := buf.Bytes()

		if golden.ShouldSet() {
			golden.Set(t, output)
		}
		require.Equal(t, string(golden.Get(t)), string(output))
	})

	t.Run("empty", func(t *testing.T) {
		buf := &bytes.Buffer{}
		writeAWSProfileSummary(buf, configPath, nil)
		output := buf.Bytes()

		if golden.ShouldSet() {
			golden.Set(t, output)
		}
		require.Equal(t, string(golden.Get(t)), string(output))
	})
}

func TestFilterAWSIdentityCenterApps(t *testing.T) {
	appWithIC, err := types.NewAppV3(types.Metadata{Name: "aws-ic"}, types.AppSpecV3{
		URI: "https://d-123.awsapps.com/start",
		IdentityCenter: &types.AppIdentityCenter{
			AccountID:      "123456789012",
			PermissionSets: []*types.IdentityCenterPermissionSet{{Name: "Admin"}},
		},
	})
	require.NoError(t, err)

	appWithoutIC, err := types.NewAppV3(types.Metadata{Name: "no-ic"}, types.AppSpecV3{
		URI: "https://example.com",
	})
	require.NoError(t, err)

	resources := types.EnrichedResources{
		{
			ResourceWithLabels: &types.AppServerV3{Spec: types.AppServerSpecV3{App: appWithIC}},
		},
		{
			ResourceWithLabels: &types.AppServerV3{Spec: types.AppServerSpecV3{App: appWithoutIC}},
		},
	}

	filtered := filterAWSIdentityCenterApps(resources)
	require.Len(t, filtered, 1)
	require.Equal(t, "aws-ic", filtered[0].GetName())
}

func TestWriteAWSConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	app1, err := types.NewAppV3(types.Metadata{
		Name: "app1",
		Labels: map[string]string{
			types.AWSAccountNameLabel: "dev",
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

	apps := []types.Application{app1, app2}

	// Pre-write some non-Teleport managed content.
	nonTeleportContent := `[default]
region = us-west-2
output = json

[profile external]
role_arn = arn:aws:iam::123456789012:role/external-role
source_profile = default
`
	err = os.WriteFile(configPath, []byte(nonTeleportContent), 0600)
	require.NoError(t, err)

	written, err := writeAWSConfig(configPath, "us-east-1", apps, false, nil)
	require.NoError(t, err)
	require.Len(t, written, 3)

	// Verify the returned slice items
	require.Equal(t, "teleport-awsic-dev-admin", written[0].Name)
	require.Equal(t, "111111111111", written[0].AccountID)
	require.Equal(t, "Admin", written[0].RoleName)
	require.Equal(t, "teleport-d-123", written[0].Session)

	require.Equal(t, "teleport-awsic-dev-reader", written[1].Name)
	require.Equal(t, "111111111111", written[1].AccountID)
	require.Equal(t, "Reader", written[1].RoleName)
	require.Equal(t, "teleport-d-123", written[1].Session)

	require.Equal(t, "teleport-awsic-222222222222-admin", written[2].Name)
	require.Equal(t, "222222222222", written[2].AccountID)
	require.Equal(t, "Admin", written[2].RoleName)
	require.Equal(t, "teleport-d-123", written[2].Session)

	// Verify file content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Verify that non-Teleport content is preserved.
	require.Contains(t, string(content), "[default]")
	require.Contains(t, string(content), "[profile external]")

	if golden.ShouldSet() {
		golden.Set(t, content)
	}
	require.Equal(t, string(golden.Get(t)), string(content))
}

func TestWriteAWSConfig_RegionFromLabel(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config")

	app1, err := types.NewAppV3(types.Metadata{
		Name: "app1",
		Labels: map[string]string{
			types.AWSAccountNameLabel: "dev",
			types.AWSSSORegionLabel:   "eu-west-1",
		},
	}, types.AppSpecV3{
		URI: "https://d-123.awsapps.com/start/#/",
		IdentityCenter: &types.AppIdentityCenter{
			AccountID: "111111111111",
			PermissionSets: []*types.IdentityCenterPermissionSet{
				{Name: "Admin"},
			},
		},
	})
	require.NoError(t, err)

	// Pass empty ssoRegion to test auto-detection from label.
	written, err := writeAWSConfig(configPath, "", []types.Application{app1}, false, nil)
	require.NoError(t, err)
	require.Len(t, written, 1)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "sso_region=eu-west-1")
}

func TestWriteAWSConfig_RegionFlagOverridesLabel(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config")

	app1, err := types.NewAppV3(types.Metadata{
		Name: "app1",
		Labels: map[string]string{
			types.AWSAccountNameLabel: "dev",
			types.AWSSSORegionLabel:   "eu-west-1",
		},
	}, types.AppSpecV3{
		URI: "https://d-123.awsapps.com/start/#/",
		IdentityCenter: &types.AppIdentityCenter{
			AccountID: "111111111111",
			PermissionSets: []*types.IdentityCenterPermissionSet{
				{Name: "Admin"},
			},
		},
	})
	require.NoError(t, err)

	// Explicit flag should override label.
	written, err := writeAWSConfig(configPath, "us-west-2", []types.Application{app1}, false, nil)
	require.NoError(t, err)
	require.Len(t, written, 1)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "sso_region=us-west-2")
	require.NotContains(t, string(content), "eu-west-1")
}

func TestWriteAWSConfig_Pruning(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config")

	// Set up initial config with a stale Teleport profile.
	initialContent := `; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[sso-session teleport-stale]
sso_start_url = https://stale.awsapps.com/start
sso_region = us-east-1

; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[profile teleport-awsic-stale-admin]
sso_session = teleport-stale
sso_account_id = 111111111111
sso_role_name = Admin

[profile external]
region = us-east-1
`
	err := os.WriteFile(configPath, []byte(initialContent), 0600)
	require.NoError(t, err)

	// Now write config for a set of apps that doesn't include the stale one.
	app1, err := types.NewAppV3(types.Metadata{
		Name:   "aws-ic-new",
		Labels: map[string]string{types.AWSAccountNameLabel: "production"},
	}, types.AppSpecV3{
		URI: "https://production.awsapps.com/start/#/console",
		IdentityCenter: &types.AppIdentityCenter{
			AccountID: "222222222222",
			PermissionSets: []*types.IdentityCenterPermissionSet{
				{Name: "Admin"},
			},
		},
	})
	require.NoError(t, err)

	apps := []types.Application{app1}

	written, err := writeAWSConfig(configPath, "us-east-1", apps, false, nil)
	require.NoError(t, err)
	require.Len(t, written, 1)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	contentStr := string(content)

	// Verify new profile is there
	require.Contains(t, contentStr, "[profile teleport-awsic-production-admin]")
	require.Contains(t, contentStr, "[sso-session teleport-production]")

	// Verify external profile is preserved
	require.Contains(t, contentStr, "[profile external]")

	// Verify stale profile is gone
	require.NotContains(t, contentStr, "[profile teleport-awsic-stale-admin]")
	require.NotContains(t, contentStr, "[sso-session teleport-stale]")
}
