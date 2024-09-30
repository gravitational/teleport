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

package update_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gravitational/trace"
)

const (
	testBinaryName       = "updater"
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
)

var (
	// testVersions list of the pre-compiled binaries with encoded versions to check.
	testVersions = []string{
		"1.2.3",
		"3.2.1",
	}
	limitedWriter = newLimitedResponseWriter()
	pattern       = regexp.MustCompile(`(?m)Teleport v(.*) git`)

	toolsDir string
	baseURL  string
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp(os.TempDir(), testBinaryName)
	if err != nil {
		log.Fatalf("failed to create temporary directory: %v", err)
	}

	toolsDir, err = os.MkdirTemp(os.TempDir(), "toolsDir")
	if err != nil {
		log.Fatalf("failed to create temporary directory: %v", err)
	}

	var srv *http.Server
	srv, baseURL = startTestHTTPServer(tmp)
	for _, version := range testVersions {
		if err := buildAndArchiveApps(tmp, toolsDir, version, baseURL); err != nil {
			log.Fatalf("failed to build testing app binary archive: %v", err)
		}
	}

	// Run tests after binary is built.
	code := m.Run()

	if err := srv.Close(); err != nil {
		log.Fatalf("failed to shutdown server: %v", err)
	}
	if err := os.RemoveAll(tmp); err != nil {
		log.Fatalf("failed to remove temporary directory: %v", err)
	}
	if err := os.RemoveAll(toolsDir); err != nil {
		log.Fatalf("failed to remove tools directory: %v", err)
	}

	os.Exit(code)
}

// serve256File calculates sha256 checksum for requested file.
func serve256File(w http.ResponseWriter, _ *http.Request, filePath string) {
	log.Printf("Calculating and serving file checksum: %s\n", filePath)

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(filePath)+".sha256\"")
	w.Header().Set("Content-Type", "plain/text")

	hash := sha256.New()
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if _, err := io.Copy(hash, file); err != nil {
		http.Error(w, "failed to write to hash", http.StatusInternalServerError)
		return
	}
	if _, err := hex.NewEncoder(w).Write(hash.Sum(nil)); err != nil {
		http.Error(w, "failed to write checksum", http.StatusInternalServerError)
	}
}

// generateZipFile compresses the file into a `.zip` format. This format intended to be
// used only for windows platform and mocking paths for windows archive.
func generateZipFile(filePath, destPath string) error {
	archive, err := os.Create(destPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	return filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		zipFileWriter, err := zipWriter.Create(filepath.Base(path))
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = io.Copy(zipFileWriter, file)
		return trace.Wrap(err)
	})
}

// generateTarGzFile compresses files into a `.tar.gz` format specifically in file
// structure related to linux packaging.
func generateTarGzFile(filePath, destPath string) error {
	archive, err := os.Create(destPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer archive.Close()

	gzipWriter := gzip.NewWriter(archive)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = filepath.Join("teleport", filepath.Base(info.Name()))
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		_, err = io.Copy(tarWriter, file)
		return trace.Wrap(err)
	})
}

// generatePkgFile runs the macOS `pkgbuild` command to generate a .pkg file from the source.
func generatePkgFile(filePath, destPath string) error {
	cmd := exec.Command("pkgbuild",
		"--root", filePath,
		"--identifier", "com.example.pkgtest",
		"--version", "1.0",
		destPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to generate .pkg: %s\n", output)
		return err
	}

	return nil
}

// startTestHTTPServer starts the file-serving HTTP server for testing.
func startTestHTTPServer(baseDir string) (*http.Server, string) {
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(baseDir, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			serve256File(w, r, strings.TrimSuffix(filePath, ".sha256"))
		default:
			http.ServeFile(limitedWriter.Wrap(w), r, filePath)
		}
	})}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}

	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("failed to start server: %s", err)
		}
	}()

	return srv, listener.Addr().String()
}

// buildAndArchiveApps compiles the updater integration and pack it depends on platform is used.
func buildAndArchiveApps(path string, toolsDir string, version string, address string) error {
	versionPath := filepath.Join(path, version)
	for _, app := range []string{"tsh", "tctl"} {
		output := filepath.Join(versionPath, app)
		switch runtime.GOOS {
		case "windows":
			output = filepath.Join(versionPath, app+".exe")
		case "darwin":
			output = filepath.Join(versionPath, app+".app", "Contents", "MacOS", app)
		}
		if err := buildBinary(output, toolsDir, version, address); err != nil {
			return trace.Wrap(err)
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return trace.Wrap(generatePkgFile(versionPath, path+"/tsh-"+version+".pkg"))
	case "windows":
		return trace.Wrap(generateZipFile(versionPath, path+"/teleport-v"+version+"-windows-amd64-bin.zip"))
	case "linux":
		return trace.Wrap(generateTarGzFile(versionPath, path+"/teleport-v"+version+"-linux-"+runtime.GOARCH+"-bin.tar.gz"))
	default:
		return trace.BadParameter("unsupported platform")
	}
}

// buildBinary executes command to build binary with updater logic only for testing.
func buildBinary(output string, toolsDir string, version string, address string) error {
	cmd := exec.Command(
		"go", "build", "-o", output,
		"-ldflags", strings.Join([]string{
			fmt.Sprintf("-X 'main.toolsDir=%s'", toolsDir),
			fmt.Sprintf("-X 'main.version=%s'", version),
			fmt.Sprintf("-X 'main.baseUrl=http://%s'", address),
		}, " "),
		"./updater",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return trace.Wrap(cmd.Run())
}
