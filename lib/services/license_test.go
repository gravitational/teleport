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

package services

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

func TestLicenseUnmarshal(t *testing.T) {
	t.Parallel()

	type testCase struct {
		description string
		input       string
		expected    types.License
		err         error
	}
	testCases := []testCase{
		{
			description: "simple case",
			input:       `{"kind": "license", "version": "v3", "metadata": {"name": "Teleport Commercial"}, "spec": {"account_id": "accountID", "usage": true, "k8s": true, "app": true, "db": true, "desktop": true, "feature_hiding": false, "aws_account": "123", "aws_pid": "4", "custom_theme": "cool-theme"}}`,
			expected: MustNew("Teleport Commercial", types.LicenseSpecV3{
				ReportsUsage:              types.NewBool(true),
				SupportsKubernetes:        types.NewBool(true),
				SupportsApplicationAccess: types.NewBoolP(true),
				SupportsDatabaseAccess:    types.NewBool(true),
				SupportsDesktopAccess:     types.NewBool(true),
				Cloud:                     types.NewBool(false),
				SupportsFeatureHiding:     types.NewBool(false),
				AWSAccountID:              "123",
				AWSProductID:              "4",
				AccountID:                 "accountID",
				CustomTheme:               "cool-theme",
			}),
		},
		{
			description: "simple case with string booleans",
			input:       `{"kind": "license", "version": "v3", "metadata": {"name": "license"}, "spec": {"account_id": "accountID", "usage": "yes", "k8s": "yes", "app": "yes", "db": "yes", "desktop": "yes", "feature_hiding": "no", "aws_account": "123", "aws_pid": "4", "custom_theme": "cool-theme"}}`,
			expected: MustNew("license", types.LicenseSpecV3{
				ReportsUsage:              types.NewBool(true),
				SupportsKubernetes:        types.NewBool(true),
				SupportsApplicationAccess: types.NewBoolP(true),
				SupportsDatabaseAccess:    types.NewBool(true),
				SupportsDesktopAccess:     types.NewBool(true),
				Cloud:                     types.NewBool(false),
				SupportsFeatureHiding:     types.NewBool(false),
				AWSAccountID:              "123",
				AWSProductID:              "4",
				AccountID:                 "accountID",
				CustomTheme:               "cool-theme",
			}),
		},
		{
			description: "with cloud flag",
			input:       `{"kind": "license", "version": "v3", "metadata": {"name": "license"}, "spec": {"cloud": "yes", "account_id": "accountID", "usage": "yes", "k8s": "yes", "aws_account": "123", "aws_pid": "4"}}`,
			expected: MustNew("license", types.LicenseSpecV3{
				ReportsUsage:           types.NewBool(true),
				SupportsKubernetes:     types.NewBool(true),
				SupportsDatabaseAccess: types.NewBool(false),
				SupportsDesktopAccess:  types.NewBool(false),
				Cloud:                  types.NewBool(true),
				AWSAccountID:           "123",
				AWSProductID:           "4",
				AccountID:              "accountID",
			}),
		},
		{
			description: "failed validation - unknown version",
			input:       `{"kind": "license", "version": "v2", "metadata": {"name": "license"}, "spec": {"usage": "yes", "k8s": "yes", "aws_account": "123", "aws_pid": "4"}}`,
			err:         trace.BadParameter(""),
		},
		{
			description: "failed validation, bad types",
			input:       `{"kind": "license", "version": "v3", "metadata": {"name": "license"}, "spec": {"usage": 1, "k8s": "yes", "aws_account": 14, "aws_pid": "4"}}`,
			err:         trace.BadParameter(""),
		},
	}
	for _, tc := range testCases {
		comment := fmt.Sprintf("test case %q", tc.description)
		out, err := UnmarshalLicense([]byte(tc.input))
		if tc.err == nil {
			require.NoError(t, err, comment)
			require.Empty(t, cmp.Diff(tc.expected, out))
			data, err := MarshalLicense(out)
			require.NoError(t, err, comment)
			out2, err := UnmarshalLicense(data)
			require.NoError(t, err, comment)
			require.Empty(t, cmp.Diff(tc.expected, out2))
		} else {
			require.IsType(t, err, tc.err, comment)
		}
	}
}

func TestIsDashboard(t *testing.T) {
	tt := []struct {
		name     string
		features proto.Features
		expected bool
	}{
		{
			name: "not cloud nor recovery codes is not dashboard",
			features: proto.Features{
				Cloud:         false,
				RecoveryCodes: false,
			},
			expected: false,
		},
		{
			name: "not cloud, with recovery codes is dashboard",
			features: proto.Features{
				Cloud:         false,
				RecoveryCodes: true,
			},
			expected: true,
		},
		{
			name: "cloud, with recovery codes is not dashboard",
			features: proto.Features{
				Cloud:         true,
				RecoveryCodes: true,
			},
			expected: false,
		},
		{
			name: "cloud, without recovery codes is not dashboard",
			features: proto.Features{
				Cloud:         true,
				RecoveryCodes: false,
			},
			expected: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDashboard(tc.features)
			require.Equal(t, tc.expected, result)
		})
	}
}

// MustNew is like New, but panics in case of error,
// used in tests
func MustNew(name string, spec types.LicenseSpecV3) types.License {
	out, err := types.NewLicense(name, spec)
	if err != nil {
		panic(err)
	}
	return out
}
