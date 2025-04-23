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

package awsconfigfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// strPtr returns a pointer to the given string.
func strPtr(s string) *string {
	return &s
}

func TestAWSConfigFilePath(t *testing.T) {
	t.Run("AWS_CONFIG_FILE is set", func(t *testing.T) {
		t.Setenv("AWS_CONFIG_FILE", "/path/to/config")

		path, err := AWSConfigFilePath()
		require.NoError(t, err)
		require.Equal(t, "/path/to/config", path)
	})

	t.Run("AWS_CONFIG_FILE is not set", func(t *testing.T) {
		t.Setenv("AWS_CONFIG_FILE", "")
		os.Unsetenv("AWS_CONFIG_FILE")

		path, err := AWSConfigFilePath()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(os.Getenv("HOME"), ".aws", "config"), path)
	})
}

func TestAddCredentialProcessToSection(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sectionName      string
		existingContents *string
		errCheck         require.ErrorAssertionFunc
		expected         string
	}{
		{
			name:             "adds section",
			sectionName:      "profile my-aws-iam-ra-profile",
			existingContents: nil, // no config file
			errCheck:         require.NoError,
			expected: `; This section is managed by Teleport. Do not edit.
[profile my-aws-iam-ra-profile]
credential_process = credential_process
`,
		},
		{
			name:             "no config file",
			sectionName:      "default",
			existingContents: nil, // no config file
			errCheck:         require.NoError,
			expected: `; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`,
		},
		{
			name:             "empty config file",
			sectionName:      "default",
			existingContents: strPtr(""),
			errCheck:         require.NoError,
			expected: `; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`,
		},
		{
			name:             "no default profile",
			sectionName:      "default",
			existingContents: strPtr("[profile foo]"),
			errCheck:         require.NoError,
			expected: `[profile foo]

; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`,
		},
		{
			name:        "replaces default credential process",
			sectionName: "default",
			existingContents: strPtr(`[default]
credential_process = another process`),
			errCheck: require.NoError,
			expected: `; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`,
		},
		{
			name:        "comments are kept",
			sectionName: "default",
			existingContents: strPtr(`[default]
; this is a comment
# yet another comment
credential_process = another process`),
			errCheck: require.NoError,
			expected: `; This section is managed by Teleport. Do not edit.
[default]
; this is a comment
# yet another comment
credential_process = credential_process
`,
		},
		{
			name:        "error when default profile exists and has other fields",
			sectionName: "default",
			existingContents: strPtr(`[default]
credential_process = another process
another_field = another_value`),
			errCheck: require.Error,
		},
		{
			name:             "invalid file returns an error",
			sectionName:      "default",
			existingContents: strPtr(`[invalid section`),
			errCheck:         require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configFilePath := filepath.Join(t.TempDir(), "config")
			if tc.existingContents != nil {
				err := os.WriteFile(configFilePath, []byte(*tc.existingContents), 0600)
				require.NoError(t, err)
			}

			err := addCredentialProcessToSection(configFilePath, tc.sectionName, "credential_process")
			tc.errCheck(t, err)

			if tc.expected != "" {
				bs, err := os.ReadFile(configFilePath)
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(bs))
			}
		})
	}

	t.Run("creates directory if it does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFilePath := filepath.Join(tmpDir, "dir", "config")
		err := SetDefaultProfileCredentialProcess(configFilePath, "credential_process")
		require.NoError(t, err)

		require.DirExists(t, filepath.Join(tmpDir, "dir"))
		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`, string(bs))
	})
}

func TestRemoveProfilesUsingCredentialProcess(t *testing.T) {
	for _, tc := range []struct {
		name             string
		existingContents *string
		errCheck         require.ErrorAssertionFunc
		expected         string
	}{
		{
			name:             "no config file",
			existingContents: nil, // no config file
			errCheck:         require.NoError,
			expected:         "",
		},
		{
			name:             "empty config file",
			existingContents: strPtr(""),
			errCheck:         require.NoError,
			expected:         "",
		},
		{
			name:             "no profiles using credential_process",
			existingContents: strPtr("[profile foo]"),
			errCheck:         require.NoError,
			expected:         "[profile foo]",
		},
		{
			name:             "removes profiles using credential_process",
			existingContents: strPtr("[profile foo]\ncredential_process = process\n[profile bar]\ncredential_process = process\n[profile baz]\ncredential_process = not the process\n"),
			errCheck:         require.NoError,
			expected:         "[profile baz]\ncredential_process = not the process\n",
		},
		{
			name:             "only one profile using credential_process",
			existingContents: strPtr("[profile foo]\ncredential_process = process\nanother = field"),
			errCheck:         require.NoError,
			expected:         "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configFilePath := filepath.Join(t.TempDir(), "config")
			if tc.existingContents != nil {
				err := os.WriteFile(configFilePath, []byte(*tc.existingContents), 0600)
				require.NoError(t, err)
			}

			err := RemoveProfilesUsingCredentialProcess(configFilePath, "process")
			tc.errCheck(t, err)

			if tc.expected != "" {
				bs, err := os.ReadFile(configFilePath)
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(bs))
			}
		})
	}
}

func TestUpdateRemoveCycle(t *testing.T) {
	initialContents := []byte("[profile baz]\nregion = us-east-1\n\n[default]\nregion = us-west-2\n")
	configFilePath := filepath.Join(t.TempDir(), "config")
	err := os.WriteFile(configFilePath, initialContents, 0600)
	require.NoError(t, err)

	err = UpsertProfileCredentialProcess(configFilePath, "my-profile", "my-process")
	require.NoError(t, err)

	err = RemoveProfilesUsingCredentialProcess(configFilePath, "my-process")
	require.NoError(t, err)

	bs, err := os.ReadFile(configFilePath)
	require.NoError(t, err)
	require.Equal(t, initialContents, bs)
}
