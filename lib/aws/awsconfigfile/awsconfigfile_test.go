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
			expected: `; Do not edit. Section managed by Teleport.
[profile my-aws-iam-ra-profile]
credential_process = credential_process
`,
		},
		{
			name:             "no config file",
			sectionName:      "default",
			existingContents: nil, // no config file
			errCheck:         require.NoError,
			expected: `; Do not edit. Section managed by Teleport.
[default]
credential_process = credential_process
`,
		},
		{
			name:             "empty config file",
			sectionName:      "default",
			existingContents: strPtr(""),
			errCheck:         require.NoError,
			expected: `; Do not edit. Section managed by Teleport.
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

; Do not edit. Section managed by Teleport.
[default]
credential_process = credential_process
`,
		},
		{
			name:        "replaces default credential process",
			sectionName: "default",
			existingContents: strPtr(`; Do not edit. Section managed by Teleport.
[default]
credential_process = another process`),
			errCheck: require.NoError,
			expected: `; Do not edit. Section managed by Teleport.
[default]
credential_process = credential_process
`,
		},
		{
			name:        "comments are kept",
			sectionName: "default",
			existingContents: strPtr(`; Do not edit. Section managed by Teleport.
[default]
; this is a comment
# yet another comment
credential_process = another process`),
			errCheck: require.NoError,
			expected: `; Do not edit. Section managed by Teleport.
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
		{
			name:        "error when profile does not have the expected comment",
			sectionName: "default",
			existingContents: strPtr(`; Another Comment
[default]
credential_process = another process
another_field = another_value`),
			errCheck: require.Error,
		},
		{
			name:        "re-apply the login, should not add another section",
			sectionName: "profile Upper-and-lower-CASE",
			existingContents: strPtr(`[sectionA]
some_setting = value

; Do not edit. Section managed by Teleport.
[profile Upper-and-lower-CASE]
credential_process=credential_process
`),
			errCheck: require.NoError,
			expected: `[sectionA]
some_setting = value

; Do not edit. Section managed by Teleport.
[profile Upper-and-lower-CASE]
credential_process = credential_process
`,
		},
		{
			name:        "refuses to change the profile when a profile with the same name already exists but has no comment",
			sectionName: "profile My-Profile",
			existingContents: strPtr(`[sectionA]
some_setting = value

[profile My-Profile]
credential_process=credential_process
`),
			errCheck: require.Error,
		},
		{
			// This is not exactly a test but serves documentation purposes on the limitation of the library we use.
			// It's not possible to keep the exact formatting of the existing file because it doesn't support it.
			// Instead, it will reformat the file before saving it.
			// The library supports turning off pretty printing but that would just reformat the entire file using no spaces, and no alignment,
			// even if the original file had it.
			name:        "document reformatting behavior",
			sectionName: "profile Upper-and-lower-CASE",
			existingContents: strPtr(`[sectionA]
with_spaces = value
without_spaces=value`),
			errCheck: require.NoError,
			expected: `[sectionA]
with_spaces    = value
without_spaces = value

; Do not edit. Section managed by Teleport.
[profile Upper-and-lower-CASE]
credential_process = credential_process
`,
		},
		{
			name:        "upserting an existing profile which used the previous version of the comment",
			sectionName: "profile Upper-and-lower-CASE",
			existingContents: strPtr(`[sectionA]
some_setting = value

; Do not edit. Section managed by Teleport. Generated for accessing Upper-and-lower-CASE
[profile Upper-and-lower-CASE]
credential_process=credential_process
`),
			errCheck: require.NoError,
			expected: `[sectionA]
some_setting = value

; Do not edit. Section managed by Teleport.
[profile Upper-and-lower-CASE]
credential_process = credential_process
`,
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

		dir := filepath.Join(tmpDir, "dir")
		require.DirExists(t, dir)
		info, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())

		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; Do not edit. Section managed by Teleport.
[default]
credential_process = credential_process
`, string(bs))
	})

	t.Run("sets a named profile", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFilePath := filepath.Join(tmpDir, "dir", "config")
		err := UpsertProfileCredentialProcess(configFilePath, "my-profile", "credential_process")
		require.NoError(t, err)

		require.DirExists(t, filepath.Join(tmpDir, "dir"))
		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; Do not edit. Section managed by Teleport.
[profile my-profile]
credential_process = credential_process
`, string(bs))
	})
}

func TestRemoveTeleportManagedProfile(t *testing.T) {
	for _, tc := range []struct {
		name             string
		existingContents *string
		profile          string
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
			name:             "no section with expected comment",
			existingContents: strPtr("; another comment\n[profile foo]\ncredential_process = process"),
			errCheck:         require.NoError,
			expected:         "; another comment\n[profile foo]\ncredential_process = process",
		},
		{
			name:             "matching comment but no profile using credential_process",
			existingContents: strPtr("; a comment\n[profile foo]\nanother_key = value"),
			errCheck:         require.NoError,
			expected:         "; a comment\n[profile foo]\nanother_key = value",
		},
		{
			name:             "removes the entire profile when the only key is the credential process",
			existingContents: strPtr("; a comment\n[profile foo]\ncredential_process = process"),
			errCheck:         require.NoError,
			expected:         "",
		},
		{
			name:    "an error is returned if comment doesn't match",
			profile: "foo",
			existingContents: strPtr(`; a comment
[profile foo]
credential_process = process
`),
			errCheck: require.Error,
			expected: `; a comment
[profile foo]
credential_process = process
`,
		},
		{
			name:    "no error even if it has more keys",
			profile: "foo",
			existingContents: strPtr(`; Do not edit. Section managed by Teleport.
[profile foo]
credential_process = process
another_key = value
`),
			errCheck: require.NoError,
			expected: ``,
		},
		{
			name:    "no error even if it is using the previous comment version",
			profile: "foo",
			existingContents: strPtr(`; Do not edit. Section managed by Teleport. Generated for accessing MyApp
[profile foo]
credential_process = process
another_key = value
`),
			errCheck: require.NoError,
			expected: ``,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configFilePath := filepath.Join(t.TempDir(), "config")
			if tc.existingContents != nil {
				err := os.WriteFile(configFilePath, []byte(*tc.existingContents), 0600)
				require.NoError(t, err)
			}
			err := RemoveTeleportManagedProfile(configFilePath, tc.profile)
			tc.errCheck(t, err)

			if tc.expected != "" {
				bs, err := os.ReadFile(configFilePath)
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(bs))
			}
		})
	}
}

func TestRemoveAllTeleportManagedProfiles(t *testing.T) {
	for _, tc := range []struct {
		name             string
		profile          string
		existingContents *string
		errCheck         require.ErrorAssertionFunc
		expected         string
	}{
		{
			name: "multiple sections are removed",
			existingContents: strPtr(`; Do not remove. Generated by ACME Tool
[profile foo1]
credential_process = process

[default]
aws_region = us-east-1

; Do not edit. Section managed by Teleport.
[profile foo2]
credential_process = process

; Do not edit. Section managed by Teleport.
[profile foo3]
credential_process = process
`),
			errCheck: require.NoError,
			expected: `; Do not remove. Generated by ACME Tool
[profile foo1]
credential_process = process

[default]
aws_region = us-east-1
`,
		},
		{
			name: "does not remove any section when comments are missing",
			existingContents: strPtr(`[profile foo1]
credential_process = process

[default]
aws_region = us-east-1

[profile foo2]
credential_process = process
`),
			errCheck: require.NoError,
			expected: `[profile foo1]
credential_process = process

[default]
aws_region = us-east-1

[profile foo2]
credential_process = process
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configFilePath := filepath.Join(t.TempDir(), "config")
			if tc.existingContents != nil {
				err := os.WriteFile(configFilePath, []byte(*tc.existingContents), 0600)
				require.NoError(t, err)
			}
			err := RemoveAllTeleportManagedProfiles(configFilePath)
			tc.errCheck(t, err)

			if tc.expected != "" {
				bs, err := os.ReadFile(configFilePath)
				require.NoError(t, err)
				require.Equal(t, tc.expected, string(bs))
			}
		})
	}
}

func TestSSORemovalGuard(t *testing.T) {
	configFilePath := filepath.Join(t.TempDir(), "config")

	// 1. Create a transient profile (managed by old comment)
	err := UpsertProfileCredentialProcess(configFilePath, "transient", "tsh apps config --format aws-credential-process app")
	require.NoError(t, err)

	// 2. Create an SSO session and profile (managed by new SSO comment)
	err = UpsertSSOSession(configFilePath, "sso-session", "https://start.url", "us-east-1")
	require.NoError(t, err)
	err = UpsertSSOProfile(configFilePath, SSOProfile{
		Name:      "sso-profile",
		Session:   "sso-session",
		AccountID: "123",
		RoleName:  "Admin",
	})
	require.NoError(t, err)

	// 3. Verify they all exist
	content, err := os.ReadFile(configFilePath)
	require.NoError(t, err)
	s := string(content)
	require.Contains(t, s, "[profile transient]")
	require.Contains(t, s, "[sso-session sso-session]")
	require.Contains(t, s, "[profile sso-profile]")

	// 4. Run RemoveAllTeleportManagedProfiles (simulating 'tsh apps logout')
	err = RemoveAllTeleportManagedProfiles(configFilePath)
	require.NoError(t, err)

	// 5. Verify transient is gone, but SSO sections remain
	content, err = os.ReadFile(configFilePath)
	require.NoError(t, err)
	s = string(content)

	require.NotContains(t, s, "[profile transient]", "Transient profile should be removed on logout")
	require.Contains(t, s, "[sso-session sso-session]", "SSO session should persist after logout")
	require.Contains(t, s, "[profile sso-profile]", "SSO profile should persist after logout")
}

func TestUpdateRemoveCycle(t *testing.T) {
	initialContents := "[profile baz]\nregion = us-east-1\n\n[default]\nregion = us-west-2\n"
	configFilePath := filepath.Join(t.TempDir(), "config")
	err := os.WriteFile(configFilePath, []byte(initialContents), 0600)
	require.NoError(t, err)

	err = UpsertProfileCredentialProcess(configFilePath, "my-profile", "my-process")
	require.NoError(t, err)

	err = UpsertProfileCredentialProcess(configFilePath, "my-profile2", "my-process")
	require.NoError(t, err)

	err = UpsertProfileCredentialProcess(configFilePath, "my-profile3", "my-process")
	require.NoError(t, err)

	err = RemoveTeleportManagedProfile(configFilePath, "my-profile")
	require.NoError(t, err)

	err = RemoveAllTeleportManagedProfiles(configFilePath)
	require.NoError(t, err)

	bs, err := os.ReadFile(configFilePath)
	require.NoError(t, err)
	require.Equal(t, initialContents, string(bs))
}
func TestSSOConfig(t *testing.T) {
	t.Run("UpsertSSOSession", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFilePath := filepath.Join(tmpDir, "dir", "config")

		err := UpsertSSOSession(configFilePath, "my-session", "https://start.url", "us-east-1")
		require.NoError(t, err)

		dir := filepath.Join(tmpDir, "dir")
		require.DirExists(t, dir)
		info, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())

		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[sso-session my-session]
sso_start_url = https://start.url
sso_region    = us-east-1
`, string(bs))
	})

	t.Run("UpsertSSOProfile", func(t *testing.T) {
		configFilePath := filepath.Join(t.TempDir(), "config")

		err := UpsertSSOProfile(configFilePath, SSOProfile{
			Name:      "my-profile",
			Session:   "my-session",
			AccountID: "123456789012",
			RoleName:  "Admin",
		})
		require.NoError(t, err)

		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[profile my-profile]
sso_session    = my-session
sso_account_id = 123456789012
sso_role_name  = Admin
`, string(bs))
	})

	t.Run("UpsertSSOProfile errors on credential_process", func(t *testing.T) {
		configFilePath := filepath.Join(t.TempDir(), "config")
		initial := `; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[profile my-profile]
credential_process = some-command
`
		err := os.WriteFile(configFilePath, []byte(initial), 0600)
		require.NoError(t, err)

		err = UpsertSSOProfile(configFilePath, SSOProfile{
			Name:      "my-profile",
			Session:   "my-session",
			AccountID: "123456789012",
			RoleName:  "Admin",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "contains 'credential_process' and cannot be converted to an SSO profile")
	})

	t.Run("UpsertSSOSession handles updates", func(t *testing.T) {
		configFilePath := filepath.Join(t.TempDir(), "config")

		err := UpsertSSOSession(configFilePath, "my-session", "https://start.url", "us-east-1")
		require.NoError(t, err)

		err = UpsertSSOSession(configFilePath, "my-session", "https://new.url", "us-west-2")
		require.NoError(t, err)

		bs, err := os.ReadFile(configFilePath)
		require.NoError(t, err)
		require.Equal(t, `; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[sso-session my-session]
sso_start_url = https://new.url
sso_region    = us-west-2
`, string(bs))
	})
}

func TestWriteSSOConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config")

	initialContent := `[profile external]
region = us-west-2

; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[sso-session teleport-stale]
sso_start_url = https://stale.url
sso_region = us-east-1

; Do not edit. Section managed by Teleport (AWS Identity Center integration).
[profile teleport-stale-profile]
sso_session = teleport-stale
`
	err := os.WriteFile(configPath, []byte(initialContent), 0600)
	require.NoError(t, err)

	profiles := []SSOProfile{
		{
			Name:      "new-profile",
			Session:   "new-session",
			AccountID: "123",
			RoleName:  "Admin",
		},
	}
	sessions := []SSOSession{
		{
			Name:     "new-session",
			StartURL: "https://new.url",
			Region:   "us-east-1",
		},
	}

	err = WriteSSOConfig(configPath, profiles, sessions)
	require.NoError(t, err)

	bs, err := os.ReadFile(configPath)
	require.NoError(t, err)
	content := string(bs)

	// External preserved
	require.Contains(t, content, "[profile external]")

	// New sections added
	require.Contains(t, content, "[sso-session new-session]")
	require.Contains(t, content, "[profile new-profile]")
	require.Contains(t, content, "sso_start_url = https://new.url")

	// Stale sections pruned
	require.NotContains(t, content, "[sso-session teleport-stale]")
	require.NotContains(t, content, "[profile teleport-stale-profile]")
}
