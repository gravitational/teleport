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

func TestTeleportInstaller_Install(t *testing.T) {
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
		tt := tt
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

			teleportVersion, err := os.ReadFile(filepath.Join(dir, version, "teleport"))
			require.NoError(t, err)
			require.Equal(t, version, string(teleportVersion))

			tshVersion, err := os.ReadFile(filepath.Join(dir, version, "tsh"))
			require.NoError(t, err)
			require.Equal(t, version, string(tshVersion))

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
		{"teleport", version},
		{"tsh", version},
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
