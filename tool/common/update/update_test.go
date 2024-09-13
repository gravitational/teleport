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

package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
)

const (
	testBinaryName = "updater"
)

var (
	// testVersions list of the pre-compiled binaries with encoded versions to check.
	testVersions = []string{
		"1.2.3",
		"3.2.1",
	}
	limitedWriter = newLimitedResponseWriter()
)

// TestLock verifies that second lock call is blocked until first is released.
func TestLock(t *testing.T) {
	var locked atomic.Bool

	dir := os.TempDir()

	// Acquire first lock should not return any error.
	unlock, err := lock(dir)
	require.NoError(t, err)
	locked.Store(true)

	signal := make(chan struct{})
	errChan := make(chan error)
	go func() {
		signal <- struct{}{}
		unlock, err := lock(dir)
		if err != nil {
			errChan <- err
		}
		if locked.Load() {
			errChan <- fmt.Errorf("first lock is still acquired, second lock must be blocking")
		}
		unlock()
		signal <- struct{}{}
	}()

	<-signal
	// We have to wait till next lock is reached to ensure we block execution of goroutine.
	// Since this is system call we can't track if the function reach blocking state already.
	time.Sleep(100 * time.Millisecond)
	locked.Store(false)
	unlock()

	select {
	case <-signal:
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Errorf("second lock is not released")
	}
}

// TestUpdateInterruptSignal verifies the interrupt signal send to the process must stop downloading.
func TestUpdateInterruptSignal(t *testing.T) {
	dir, err := toolsDir()
	require.NoError(t, err, "failed to find tools directory")

	err = os.MkdirAll(dir, 0755)
	require.NoError(t, err, "failed to create tools directory")

	// Initial fetch the updater binary un-archive and replace.
	err = update(testVersions[0])
	require.NoError(t, err)

	var output bytes.Buffer
	cmd := exec.Command(filepath.Join(dir, "tsh"), "version")
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)

	// By setting the limit request next test http serving file going blocked until unlock is sent.
	lock := make(chan struct{})
	limitedWriter.SetLimitRequest(limitRequest{
		limit: 1024,
		lock:  lock,
	})

	errChan := make(chan error)
	go func() {
		errChan <- cmd.Run()
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Errorf("failed to wait till the download is started")
	case <-lock:
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, sendInterrupt(cmd))
		lock <- struct{}{}
	}

	// Wait till process finished with exit code 0, but we still should get progress
	// bar in output content.
	select {
	case <-time.After(5 * time.Second):
		t.Errorf("failed to wait till the process interrupted")
	case err := <-errChan:
		require.NoError(t, err)
	}
	require.Contains(t, output.String(), "Update progress:")
}

func TestUpdate(t *testing.T) {
	dir, err := toolsDir()
	require.NoError(t, err, "failed to find tools directory")

	err = os.MkdirAll(dir, 0755)
	require.NoError(t, err, "failed to create tools directory")

	// Fetch compiled test binary with updater logic and install to $TELEPORT_HOME.
	err = update(testVersions[0])
	require.NoError(t, err)

	// Verify that the installed version is equal to requested one.
	cmd := exec.Command(filepath.Join(dir, "tsh"), "version")
	out, err := cmd.Output()
	require.NoError(t, err)

	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[0], matches[1])

	// Execute version command again with setting the new version which must
	// trigger re-execution of the same command after downloading requested version.
	cmd = exec.Command(filepath.Join(dir, "tsh"), "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err = cmd.Output()
	require.NoError(t, err)

	matches = pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])
}

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp(os.TempDir(), testBinaryName)
	if err != nil {
		log.Fatalf("failed to create temporary directory: %v", err)
	}

	srv, address := startTestHTTPServer(tmp)
	baseUrl = fmt.Sprintf("http://%s", address)

	for _, version := range testVersions {
		if err := buildAndArchiveApps(tmp, version, address); err != nil {
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
func buildAndArchiveApps(path string, version string, address string) error {
	versionPath := filepath.Join(path, version)
	for _, app := range []string{"tsh", "tctl"} {
		output := filepath.Join(versionPath, app)
		switch runtime.GOOS {
		case "windows":
			output = filepath.Join(versionPath, app+".exe")
		case "darwin":
			output = filepath.Join(versionPath, app+".app", "Contents", "MacOS", app)
		}
		if err := buildBinary(output, version, address); err != nil {
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

// buildBinary executes command to build updater integration logic for testing.
func buildBinary(output string, version string, address string) error {
	cmd := exec.Command(
		"go", "build", "-o", output,
		"-ldflags", strings.Join([]string{
			fmt.Sprintf("-X 'main.version=%s'", version),
			fmt.Sprintf("-X 'github.com/gravitational/teleport/tool/common/update.baseUrl=http://%s'", address),
		}, " "),
		"./integration",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return trace.Wrap(cmd.Run())
}
