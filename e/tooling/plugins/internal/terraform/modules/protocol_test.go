package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
)

func TestVersions(t *testing.T) {
	const validIndex = `
{"modules":[{"versions":[{"version":"1.2.3"},{"version":"19.0.0-dev.llama.1"},{"version":"19.0.0-dev.llama.2"}]}]}
`
	versions := Versions{
		Modules: []Module{{
			Versions: []Version{
				{semver.Version{Major: 1, Minor: 2, Patch: 3}},
			},
		}},
	}
	// this is to test that versions are compacted to avoid duplicates.
	versions.Append(Version{semver.Version{Major: 1, Minor: 2, Patch: 3}})
	// should be sorted below dev.llama.2
	versions.Append(Version{semver.Version{Major: 19, Minor: 0, Patch: 0, PreRelease: "dev.llama.2"}})
	versions.Append(Version{semver.Version{Major: 19, Minor: 0, Patch: 0, PreRelease: "dev.llama.1"}})
	versions.Append(Version{semver.Version{Major: 19, Minor: 0, Patch: 0, PreRelease: "dev.llama.1"}})

	outPath := filepath.Join(t.TempDir(), "versions")
	require.NoError(t, versions.Save(outPath))

	got, err := LoadModulesVersionFile(outPath)
	require.NoError(t, err)
	require.Equal(t, versions, *got)

	t.Run("Formatting", func(t *testing.T) {
		got, err := os.ReadFile(outPath)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(validIndex), strings.TrimSpace(string(got)))
	})
}

func TestVersionsFilePath(t *testing.T) {
	tests := []struct {
		name      string
		baseDir   string
		namespace string
		module    string
		system    string
		expected  string
	}{
		{
			name:      "absolute basedir",
			baseDir:   "/tmp/modules",
			namespace: "teleport",
			module:    "discovery",
			system:    "aws",
			expected:  "/tmp/modules/teleport/discovery/aws/versions",
		},
		{
			name:      "relative basedir",
			baseDir:   "data",
			namespace: "org",
			module:    "mod",
			system:    "sys",
			expected:  "data/org/mod/sys/versions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := VersionsFilePath(
				tt.baseDir,
				tt.namespace,
				tt.module,
				tt.system,
			)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestModuleFilePath(t *testing.T) {
	tests := []struct {
		desc      string
		baseDir   string
		namespace string
		name      string
		system    string
		version   string
		expected  string
	}{
		{
			desc:      "standard version",
			baseDir:   "/tmp/build/modules",
			namespace: "teleport",
			name:      "discovery",
			system:    "aws",
			version:   "19.0.0",
			expected:  "/tmp/build/modules/teleport/discovery/aws/19.0.0.tar.gz",
		},
		{
			desc:      "prerelease version",
			baseDir:   "/tmp/other",
			namespace: "org",
			name:      "mod",
			system:    "sys",
			version:   "19.0.0-dev.llama.1",
			expected:  "/tmp/other/org/mod/sys/19.0.0-dev.llama.1.tar.gz",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			v, err := semver.NewVersion(test.version)
			require.NoError(t, err)

			got := ModuleFilePath(
				test.baseDir,
				test.namespace,
				test.name,
				test.system,
				*v,
			)
			require.Equal(t, test.expected, got)
		})
	}
}
