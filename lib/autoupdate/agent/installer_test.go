/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalInstaller_Install(t *testing.T) {
	t.Parallel()
	const version = "new-version"

	_, testSum := testTGZ(t, version)

	tests := []struct {
		name            string
		reservedTmp     uint64
		reservedInstall uint64
		existingSum     string
		flags           InstallFlags

		errMatch string
	}{
		{
			name: "not present",
		},
		{
			name:        "present",
			existingSum: testSum,
		},
		{
			name:        "mismatched checksum",
			existingSum: hex.EncodeToString(sha256.New().Sum(nil)),
		},
		{
			name:        "unreadable checksum",
			existingSum: "bad",
		},
		{
			name:        "out of space in /tmp",
			reservedTmp: reservedFreeDisk * 1_000_000_000,
			errMatch:    "no free space left",
		},
		{
			name:            "out of space in install dir",
			reservedInstall: reservedFreeDisk * 1_000_000_000,
			errMatch:        "no free space left",
		},
		// TODO(sclevine): test flags
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			if tt.existingSum != "" {
				err := os.WriteFile(filepath.Join(dir, checksumType), []byte(tt.existingSum), os.ModePerm)
				require.NoError(t, err)
			}

			// test parameters
			var dlPath, shaPath, shasum string

			// test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tgz, sum := testTGZ(t, version)
				shasum = sum
				var out *bytes.Buffer
				if strings.HasSuffix(r.URL.Path, "."+checksumType) { // checksum request
					shaPath = r.URL.Path
					out = bytes.NewBufferString(sum)
				} else { // tgz request
					dlPath = r.URL.Path
					out = tgz
				}
				w.Header().Set("Content-Length", strconv.Itoa(out.Len()))
				_, err := io.Copy(w, out)
				if err != nil {
					t.Fatal(err)
				}
			}))
			t.Cleanup(server.Close)

			installer := &LocalInstaller{
				InstallDir:              dir,
				HTTP:                    http.DefaultClient,
				Log:                     slog.Default(),
				ReservedFreeTmpDisk:     tt.reservedTmp,
				ReservedFreeInstallDisk: tt.reservedInstall,
			}
			ctx := context.Background()
			err := installer.Install(ctx, version, server.URL+"/{{.OS}}/{{.Arch}}/{{.Version}}", tt.flags)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.NoError(t, err)

			const expectedPath = "/" + runtime.GOOS + "/" + runtime.GOARCH + "/" + version
			require.Equal(t, expectedPath, dlPath)
			require.Equal(t, expectedPath+"."+checksumType, shaPath)

			for _, p := range []string{
				filepath.Join(dir, version, "lib", "systemd", "system", "teleport.service"),
				filepath.Join(dir, version, "bin", "teleport"),
				filepath.Join(dir, version, "bin", "tsh"),
			} {
				v, err := os.ReadFile(p)
				require.NoError(t, err)
				require.Equal(t, version, string(v))
			}

			sum, err := os.ReadFile(filepath.Join(dir, version, checksumType))
			require.NoError(t, err)
			require.Equal(t, string(sum), shasum)
		})
	}
}

func testTGZ(t *testing.T, version string) (tgz *bytes.Buffer, shasum string) {
	t.Helper()

	var buf bytes.Buffer

	sha := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(&buf, sha))
	tw := tar.NewWriter(gz)

	var files = []struct {
		Name, Body string
	}{
		{"teleport/examples/systemd/teleport.service", version},
		{"teleport/teleport", version},
		{"teleport/tsh", version},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, hex.EncodeToString(sha.Sum(nil))
}

func TestLocalInstaller_Link(t *testing.T) {
	t.Parallel()
	const version = "new-version"
	servicePath := filepath.Join(serviceDir, serviceName)

	tests := []struct {
		name            string
		installDirs     []string
		installFiles    []string
		installFileMode os.FileMode
		existingLinks   []string
		existingFiles   []string

		resultPaths []string
		errMatch    string
	}{
		{
			name: "present with new links",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,

			resultPaths: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				"lib/systemd/system/teleport.service",
			},
		},
		{
			name: "present with non-executable files",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: 0644,

			errMatch: "executable",
		},
		{
			name: "present with existing links",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,
			existingLinks: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
			},
			existingFiles: []string{
				"lib/systemd/system/teleport.service",
			},

			resultPaths: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				"lib/systemd/system/teleport.service",
			},
		},
		{
			name: "conflicting systemd files",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,
			existingLinks: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				"lib/systemd/system/teleport.service",
			},

			errMatch: "refusing",
		},
		{
			name: "conflicting bin files",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,
			existingLinks: []string{
				"bin/teleport",
				"bin/tbot",
				"lib/systemd/system/teleport.service",
			},
			existingFiles: []string{
				"bin/tsh",
			},

			errMatch: "refusing",
		},
		{
			name:         "no links",
			installFiles: []string{"README"},
			installDirs:  []string{"bin"},

			errMatch: "no binaries",
		},
		{
			name:         "no bin directory",
			installFiles: []string{"README"},

			errMatch: "binary directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionsDir := t.TempDir()
			versionDir := filepath.Join(versionsDir, version)
			err := os.MkdirAll(versionDir, 0o755)
			require.NoError(t, err)

			// setup files in version directory
			for _, d := range tt.installDirs {
				err := os.Mkdir(filepath.Join(versionDir, d), os.ModePerm)
				require.NoError(t, err)
			}
			for _, n := range tt.installFiles {
				err := os.WriteFile(filepath.Join(versionDir, n), []byte(filepath.Base(n)), tt.installFileMode)
				require.NoError(t, err)
			}

			// setup files in system links directory
			linkDir := t.TempDir()
			for _, n := range tt.existingLinks {
				err := os.MkdirAll(filepath.Dir(filepath.Join(linkDir, n)), os.ModePerm)
				require.NoError(t, err)
				err = os.Symlink(filepath.Base(n)+".old", filepath.Join(linkDir, n))
				require.NoError(t, err)
			}
			for _, n := range tt.existingFiles {
				err := os.MkdirAll(filepath.Dir(filepath.Join(linkDir, n)), os.ModePerm)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(linkDir, n), []byte(filepath.Base(n)), os.ModePerm)
				require.NoError(t, err)
			}

			installer := &LocalInstaller{
				InstallDir:     versionsDir,
				LinkBinDir:     filepath.Join(linkDir, "bin"),
				LinkServiceDir: filepath.Join(linkDir, "lib/systemd/system"),
				Log:            slog.Default(),
			}
			ctx := context.Background()
			revert, err := installer.Link(ctx, version)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)

				// verify automatic revert
				for _, link := range tt.existingLinks {
					v, err := os.Readlink(filepath.Join(linkDir, link))
					require.NoError(t, err)
					require.Equal(t, filepath.Base(link)+".old", v)
				}
				for _, n := range tt.existingFiles {
					v, err := os.ReadFile(filepath.Join(linkDir, n))
					require.NoError(t, err)
					require.Equal(t, filepath.Base(n), string(v))
				}

				// ensure revert still succeeds
				ok := revert(ctx)
				require.True(t, ok)
				return
			}
			require.NoError(t, err)

			// verify links
			for _, link := range tt.resultPaths {
				v, err := os.ReadFile(filepath.Join(linkDir, link))
				require.NoError(t, err)
				require.Equal(t, filepath.Base(link), string(v))
			}

			// verify manual revert
			ok := revert(ctx)
			require.True(t, ok)
			for _, link := range tt.existingLinks {
				v, err := os.Readlink(filepath.Join(linkDir, link))
				require.NoError(t, err)
				require.Equal(t, filepath.Base(link)+".old", v)
			}
			for _, n := range tt.existingFiles {
				v, err := os.ReadFile(filepath.Join(linkDir, n))
				require.NoError(t, err)
				require.Equal(t, filepath.Base(n), string(v))
			}
		})
	}
}

func TestLocalInstaller_TryLink(t *testing.T) {
	t.Parallel()
	const version = "new-version"
	servicePath := filepath.Join(serviceDir, serviceName)

	tests := []struct {
		name            string
		installDirs     []string
		installFiles    []string
		installFileMode os.FileMode
		existingLinks   []string
		existingFiles   []string

		resultPaths []string
		errMatch    string
	}{
		{
			name: "present with new links",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,

			resultPaths: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				"lib/systemd/system/teleport.service",
			},
		},
		{
			name: "present with non-executable files",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: 0644,

			errMatch: "executable",
		},
		{
			name: "present with existing links",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,
			existingLinks: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
			},
			existingFiles: []string{
				"lib/systemd/system/teleport.service",
			},

			errMatch: "refusing",
		},
		{
			name: "conflicting systemd files",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,
			existingLinks: []string{
				"lib/systemd/system/teleport.service",
			},

			errMatch: "replace irregular file",
		},
		{
			name: "conflicting bin files",
			installDirs: []string{
				"bin",
				"bin/somedir",
				"lib",
				"lib/systemd",
				"lib/systemd/system",
				"somedir",
			},
			installFiles: []string{
				"bin/teleport",
				"bin/tsh",
				"bin/tbot",
				servicePath,
				"README",
			},
			installFileMode: os.ModePerm,
			existingFiles: []string{
				"bin/tsh",
			},

			errMatch: "replace file",
		},
		{
			name:         "no links",
			installFiles: []string{"README"},
			installDirs:  []string{"bin"},

			errMatch: "no binaries",
		},
		{
			name:         "no bin directory",
			installFiles: []string{"README"},

			errMatch: "binary directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionsDir := t.TempDir()
			versionDir := filepath.Join(versionsDir, version)
			err := os.MkdirAll(versionDir, 0o755)
			require.NoError(t, err)

			// setup files in version directory
			for _, d := range tt.installDirs {
				err := os.Mkdir(filepath.Join(versionDir, d), os.ModePerm)
				require.NoError(t, err)
			}
			for _, n := range tt.installFiles {
				err := os.WriteFile(filepath.Join(versionDir, n), []byte(filepath.Base(n)), tt.installFileMode)
				require.NoError(t, err)
			}

			// setup files in system links directory
			linkDir := t.TempDir()
			for _, n := range tt.existingLinks {
				err := os.MkdirAll(filepath.Dir(filepath.Join(linkDir, n)), os.ModePerm)
				require.NoError(t, err)
				err = os.Symlink(filepath.Base(n)+".old", filepath.Join(linkDir, n))
				require.NoError(t, err)
			}
			for _, n := range tt.existingFiles {
				err := os.MkdirAll(filepath.Dir(filepath.Join(linkDir, n)), os.ModePerm)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(linkDir, n), []byte(filepath.Base(n)), os.ModePerm)
				require.NoError(t, err)
			}

			installer := &LocalInstaller{
				InstallDir:     versionsDir,
				LinkBinDir:     filepath.Join(linkDir, "bin"),
				LinkServiceDir: filepath.Join(linkDir, "lib/systemd/system"),
				Log:            slog.Default(),
			}
			ctx := context.Background()
			err = installer.TryLink(ctx, version)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)

				// verify no changes
				for _, link := range tt.existingLinks {
					v, err := os.Readlink(filepath.Join(linkDir, link))
					require.NoError(t, err)
					require.Equal(t, filepath.Base(link)+".old", v)
				}
				for _, n := range tt.existingFiles {
					v, err := os.ReadFile(filepath.Join(linkDir, n))
					require.NoError(t, err)
					require.Equal(t, filepath.Base(n), string(v))
				}
				return
			}
			require.NoError(t, err)

			// verify links
			for _, link := range tt.resultPaths {
				v, err := os.ReadFile(filepath.Join(linkDir, link))
				require.NoError(t, err)
				require.Equal(t, filepath.Base(link), string(v))
			}
		})
	}
}

func TestLocalInstaller_Remove(t *testing.T) {
	t.Parallel()
	const version = "existing-version"
	servicePath := filepath.Join(serviceDir, serviceName)

	tests := []struct {
		name          string
		dirs          []string
		files         []string
		createVersion string
		linkedVersion string
		removeVersion string

		errMatch string
	}{
		{
			name:          "present",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: version,
		},
		{
			name:          "present missing checksum",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{"bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: version,
		},
		{
			name:          "not present",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: "missing-version",
		},
		{
			name:          "version linked",
			dirs:          []string{"bin", "bin/somedir", "somedir", "lib", "lib/systemd", "lib/systemd/system"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README", servicePath},
			createVersion: version,
			linkedVersion: version,
			removeVersion: version,

			errMatch: ErrLinked.Error(),
		},
		{
			name:          "version empty",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: "",

			errMatch: "outside",
		},
		{
			name:          "version has path",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: "one/two",

			errMatch: "outside",
		},
		{
			name:          "version is ..",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: "..",

			errMatch: "outside",
		},
		{
			name:          "version is .",
			dirs:          []string{"bin", "bin/somedir", "somedir"},
			files:         []string{checksumType, "bin/teleport", "bin/tsh", "bin/tbot", "README"},
			createVersion: version,
			removeVersion: ".",

			errMatch: "outside",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionsDir := t.TempDir()
			versionDir := filepath.Join(versionsDir, tt.createVersion)
			err := os.MkdirAll(versionDir, 0o755)
			require.NoError(t, err)

			for _, d := range tt.dirs {
				err := os.Mkdir(filepath.Join(versionDir, d), os.ModePerm)
				require.NoError(t, err)
			}
			for _, n := range tt.files {
				err := os.WriteFile(filepath.Join(versionDir, n), []byte(filepath.Base(n)), os.ModePerm)
				require.NoError(t, err)
			}

			linkDir := t.TempDir()

			installer := &LocalInstaller{
				InstallDir:     versionsDir,
				LinkBinDir:     linkDir,
				LinkServiceDir: linkDir,
				Log:            slog.Default(),
			}
			ctx := context.Background()

			if tt.linkedVersion != "" {
				_, err = installer.Link(ctx, tt.linkedVersion)
				require.NoError(t, err)
			}
			err = installer.Remove(ctx, tt.removeVersion)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.NoError(t, err)
			_, err = os.Stat(filepath.Join(versionDir, "bin", tt.removeVersion))
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestLocalInstaller_List(t *testing.T) {
	installDir := t.TempDir()
	versions := []string{"v1", "v2"}

	for _, d := range versions {
		err := os.Mkdir(filepath.Join(installDir, d), os.ModePerm)
		require.NoError(t, err)
	}
	for _, n := range []string{"file1", "file2"} {
		err := os.WriteFile(filepath.Join(installDir, n), []byte(filepath.Base(n)), os.ModePerm)
		require.NoError(t, err)
	}
	installer := &LocalInstaller{
		InstallDir: installDir,
		Log:        slog.Default(),
	}
	ctx := context.Background()
	versions, err := installer.List(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"v1", "v2"}, versions)
}
