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

package tools_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/integration/autoupdate/tools/updater/tctl"
	"github.com/gravitational/teleport/integration/autoupdate/tools/updater/tsh"
	"github.com/gravitational/teleport/integration/helpers/archive"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testBinaryName       = "updater"
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
)

var (
	// testVersions list of the pre-compiled binaries with encoded versions to check.
	testVersions = []string{
		"1.0.0",
		"2.0.0",
		"3.0.0",
		"4.0.0",
	}
	limitedWriter = newLimitedResponseWriter()

	toolsDir string
	baseURL  string

	// tshPath is the path to initial tsh binary version 1.0.0.
	tshPath string
	// tctlPath is the path to initial tctl binary version 1.0.0.
	tctlPath string
)

func TestMain(m *testing.M) {
	executable, err := os.Executable()
	if err != nil {
		log.Fatal("failed to get executable name", err)
	}
	switch filepath.Base(executable) {
	case "tsh", "tsh.exe":
		tsh.Main()
		return
	case "tctl", "tctl.exe":
		tctl.Main()
		return
	}

	modules.SetInsecureTestMode(true)
	modules.SetModules(&modulestest.Modules{TestBuildType: modules.BuildCommunity})
	ctx := context.Background()
	tmp, err := os.MkdirTemp(os.TempDir(), testBinaryName)
	if err != nil {
		log.Fatalf("failed to create temporary directory: %v", err)
	}

	toolsDir, err = os.MkdirTemp(os.TempDir(), "tools")
	if err != nil {
		log.Fatalf("failed to create temporary directory: %v", err)
	}

	for _, version := range testVersions {
		if err := buildAndArchiveApps(ctx, tmp, version); err != nil {
			log.Fatalf("failed to build testing app binary archive: %v", err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(tmp, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			serve256File(w, r, strings.TrimSuffix(filePath, ".sha256"))
		default:
			http.ServeFile(limitedWriter.Wrap(w), r, filePath)
		}
	}))
	baseURL = server.URL
	if err := os.Setenv(autoupdate.BaseURLEnvVar, server.URL); err != nil {
		log.Fatalf("failed to set base URL environment variable: %v", err)
	}

	// Initial fetch the updater binary un-archive and replace.
	updater := tools.NewUpdater(
		toolsDir,
		testVersions[0],
		tools.WithBaseURL(baseURL),
	)
	if err := updater.Update(ctx, testVersions[0]); err != nil {
		log.Fatalf("failed to update: %v", err)
	}
	tshPath, err = updater.ToolPath(tools.DefaultClientTools()[0], testVersions[0])
	if err != nil {
		log.Fatalf("failed to get tsh path: %v", err)
	}
	tctlPath, err = updater.ToolPath(tools.DefaultClientTools()[1], testVersions[0])
	if err != nil {
		log.Fatalf("failed to get tctl path: %v", err)
	}

	// Run tests after binary is built.
	code := m.Run()

	server.Close()
	if err := os.RemoveAll(tmp); err != nil {
		log.Fatalf("failed to remove temporary directory: %v", err)
	}
	if err := os.RemoveAll(toolsDir); err != nil {
		log.Fatalf("failed to remove tools directory: %v", err)
	}
	if err := os.Unsetenv(autoupdate.BaseURLEnvVar); err != nil {
		log.Fatalf("failed to unset %q environment variable: %v", autoupdate.BaseURLEnvVar, err)
	}

	os.Exit(code)
}

// serve256File calculates sha256 checksum for requested file.
func serve256File(w http.ResponseWriter, _ *http.Request, filePath string) {
	log.Printf("Calculating and serving file checksum: %s\n", filePath)

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(filePath)+".sha256\"")
	w.Header().Set("Content-Type", "plain/text")

	file, err := os.Open(filePath)
	if errors.Is(err, os.ErrNotExist) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		http.Error(w, "failed to write to hash", http.StatusInternalServerError)
		return
	}
	if _, err := hex.NewEncoder(w).Write(hash.Sum(nil)); err != nil {
		http.Error(w, "failed to write checksum", http.StatusInternalServerError)
	}
}

// buildAndArchiveApps compiles the updater integration and pack it depends on platform is used.
func buildAndArchiveApps(ctx context.Context, path string, version string) error {
	versionPath := filepath.Join(path, version)
	for _, app := range []string{"tsh", "tctl"} {
		output := filepath.Join(versionPath, version, app)
		switch runtime.GOOS {
		case constants.WindowsOS:
			output = filepath.Join(versionPath, version, app+".exe")
		case constants.DarwinOS:
			output = filepath.Join(versionPath, app+".app", "Contents", "MacOS", version, app)
		}
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return err
		}
		testBinary, err := os.Executable()
		if err != nil {
			return err
		}
		if err := utils.CopyFile(testBinary, output, 0o755); err != nil {
			return err
		}
	}
	switch runtime.GOOS {
	case constants.DarwinOS:
		archivePath := filepath.Join(path, fmt.Sprintf("teleport-%s.pkg", version))
		return trace.Wrap(archive.CompressDirToPkgFile(ctx, versionPath, archivePath, "com.example.pkgtest"))
	case constants.WindowsOS:
		archivePath := filepath.Join(path, fmt.Sprintf("teleport-v%s-windows-amd64-bin.zip", version))
		return trace.Wrap(archive.CompressDirToZipFile(ctx, versionPath, archivePath, archive.WithNoCompress()))
	default:
		archivePath := filepath.Join(path, fmt.Sprintf("teleport-v%s-linux-%s-bin.tar.gz", version, runtime.GOARCH))
		return trace.Wrap(archive.CompressDirToTarGzFile(ctx, versionPath, archivePath, archive.WithNoCompress()))
	}
}
