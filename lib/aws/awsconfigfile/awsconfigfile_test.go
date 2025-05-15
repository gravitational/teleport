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
		sectionComment   string
		existingContents *string
		errCheck         require.ErrorAssertionFunc
		expected         string
	}{
		{
			name:             "adds section",
			sectionName:      "profile my-aws-iam-ra-profile",
			sectionComment:   "This section is managed by Teleport. Do not edit.",
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
			sectionComment:   "This section is managed by Teleport. Do not edit.",
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
			sectionComment:   "This section is managed by Teleport. Do not edit.",
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
			sectionComment:   "This section is managed by Teleport. Do not edit.",
			existingContents: strPtr("[profile foo]"),
			errCheck:         require.NoError,
			expected: `[profile foo]

; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`,
		},
		{
			name:           "replaces default credential process",
			sectionName:    "default",
			sectionComment: "This section is managed by Teleport. Do not edit.",
			existingContents: strPtr(`[default]
credential_process = another process`),
			errCheck: require.NoError,
			expected: `; This section is managed by Teleport. Do not edit.
[default]
credential_process = credential_process
`,
		},
		{
			name:           "comments are kept",
			sectionName:    "default",
			sectionComment: "This section is managed by Teleport. Do not edit.",
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
			name:           "error when default profile exists and has other fields",
			sectionName:    "default",
			sectionComment: "This section is managed by Teleport. Do not edit.",
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
		{
			name:           "error when profile does not have the expected comment",
			sectionName:    "default",
			sectionComment: "This section is managed by Teleport. Do not edit.",
			existingContents: strPtr(`; Another Comment
[default]
credential_process = another process
another_field = another_value`),
			errCheck: require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configFilePath := filepath.Join(t.TempDir(), "config")
			if tc.existingContents != nil {
				err := os.WriteFile(configFilePath, []byte(*tc.existingContents), 0600)
				require.NoError(t, err)
			}

			err := addCredentialProcessToSection(configFilePath, tc.sectionName, tc.sectionComment, "credential_process")
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
		sectionComment := "This section is managed by Teleport. Do not edit. Profile for app: my-app"
		err := SetDefaultProfileCredentialProcess(configFilePath, sectionComment, "credential_process")
		require.NoError(t, err)

		require.DirExists(t, filepath.Join(tmpDir, "dir"))
		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; This section is managed by Teleport. Do not edit. Profile for app: my-app
[default]
credential_process = credential_process
`, string(bs))
	})

	t.Run("sets a named profile", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFilePath := filepath.Join(tmpDir, "dir", "config")
		sectionComment := "This section is managed by Teleport. Do not edit. Profile for app: my-app"
		err := UpsertProfileCredentialProcess(configFilePath, "my-profile", sectionComment, "credential_process")
		require.NoError(t, err)

		require.DirExists(t, filepath.Join(tmpDir, "dir"))
		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; This section is managed by Teleport. Do not edit. Profile for app: my-app
[profile my-profile]
credential_process = credential_process
`, string(bs))
	})
}

func TestRemoveCredentialProcessByComment(t *testing.T) {
	for _, tc := range []struct {
		name             string
		commentSection   string
		existingContents *string
		errCheck         require.ErrorAssertionFunc
		expected         string
	}{
		{
			name:             "no config file",
			commentSection:   "a comment",
			existingContents: nil, // no config file
			errCheck:         require.NoError,
			expected:         "",
		},
		{
			name:             "empty config file",
			commentSection:   "a comment",
			existingContents: strPtr(""),
			errCheck:         require.NoError,
			expected:         "",
		},
		{
			name:             "no section with expected comment",
			commentSection:   "a comment",
			existingContents: strPtr("; another comment\n[profile foo]\ncredential_process = process"),
			errCheck:         require.NoError,
			expected:         "; another comment\n[profile foo]\ncredential_process = process",
		},
		{
			name:             "matching comment but no profile using credential_process",
			commentSection:   "; a comment",
			existingContents: strPtr("; a comment\n[profile foo]\nanother_key = value"),
			errCheck:         require.NoError,
			expected:         "; a comment\n[profile foo]\nanother_key = value",
		},
		{
			name:             "removes the entire profile when the only key is the credential process",
			commentSection:   "a comment",
			existingContents: strPtr("; a comment\n[profile foo]\ncredential_process = process"),
			errCheck:         require.NoError,
			expected:         "",
		},
		{
			name:             "an error is returned if more keys exist",
			commentSection:   "a comment",
			existingContents: strPtr("; a comment\n[profile foo]\ncredential_process = process\nanother_key = value"),
			errCheck:         require.Error,
			expected:         "; a comment\n[profile foo]\ncredential_process = process\nanother_key = value",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configFilePath := filepath.Join(t.TempDir(), "config")
			if tc.existingContents != nil {
				err := os.WriteFile(configFilePath, []byte(*tc.existingContents), 0600)
				require.NoError(t, err)
			}
			err := RemoveCredentialProcessByComment(configFilePath, tc.commentSection)
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
	initialContents := "[profile baz]\nregion = us-east-1\n\n[default]\nregion = us-west-2\n"
	configFilePath := filepath.Join(t.TempDir(), "config")
	err := os.WriteFile(configFilePath, []byte(initialContents), 0600)
	require.NoError(t, err)

	sectionComment := "This section is managed by Teleport. Do not edit."

	err = UpsertProfileCredentialProcess(configFilePath, "my-profile", sectionComment, "my-process")
	require.NoError(t, err)

	err = RemoveCredentialProcessByComment(configFilePath, sectionComment)
	require.NoError(t, err)

	bs, err := os.ReadFile(configFilePath)
	require.NoError(t, err)
	require.Equal(t, initialContents, string(bs))
}
