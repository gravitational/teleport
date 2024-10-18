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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	checksumType   = "sha256"
	checksumHexLen = sha256.Size * 2 // bytes to hex
)

// LocalInstaller manages the creation and removal of installations
// of Teleport.
type LocalInstaller struct {
	// InstallDir contains each installation, named by version.
	InstallDir string
	// HTTP is an HTTP client for downloading Teleport.
	HTTP *http.Client
	// Log contains a logger.
	Log *slog.Logger
	// ReservedFreeTmpDisk is the amount of disk that must remain free in /tmp
	ReservedFreeTmpDisk uint64
	// ReservedFreeInstallDisk is the amount of disk that must remain free in the install directory.
	ReservedFreeInstallDisk uint64
}

// Remove a Teleport version directory from InstallDir.
// This function is idempotent.
func (li *LocalInstaller) Remove(ctx context.Context, version string) error {
	versionDir := filepath.Join(li.InstallDir, version)
	sumPath := filepath.Join(versionDir, checksumType)

	// invalidate checksum first, to protect against partially-removed
	// directory with valid checksum.
	err := os.Remove(sumPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}
	if err := os.RemoveAll(versionDir); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Install a Teleport version directory in InstallDir.
// This function is idempotent.
func (li *LocalInstaller) Install(ctx context.Context, version, template string, flags InstallFlags) error {
	versionDir := filepath.Join(li.InstallDir, version)
	sumPath := filepath.Join(versionDir, checksumType)

	// generate download URI from template
	uri, err := makeURL(template, version, flags)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get new and old checksums. If they match, skip download.
	// Otherwise, clear the old version directory and re-download.
	checksumURI := uri + "." + checksumType
	newSum, err := li.getChecksum(ctx, checksumURI)
	if err != nil {
		return trace.Errorf("failed to download checksum from %s: %w", checksumURI, err)
	}
	oldSum, err := readChecksum(sumPath)
	if err == nil {
		if bytes.Equal(oldSum, newSum) {
			li.Log.InfoContext(ctx, "Version already present.", "version", version)
			return nil
		}
		li.Log.WarnContext(ctx, "Removing version that does not match checksum.", "version", version)
		if err := li.Remove(ctx, version); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		li.Log.WarnContext(ctx, "Removing version with unreadable checksum.", "version", version, "error", err)
		if err := li.Remove(ctx, version); err != nil {
			return trace.Wrap(err)
		}
	}

	// Verify that we have enough free temp space, then download tgz
	freeTmp, err := utils.FreeDiskWithReserve(os.TempDir(), li.ReservedFreeTmpDisk)
	if err != nil {
		return trace.Errorf("failed to calculate free disk: %w", err)
	}
	f, err := os.CreateTemp("", "teleport-update-")
	if err != nil {
		return trace.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		_ = f.Close() // data never read after close
		if err := os.Remove(f.Name()); err != nil {
			li.Log.WarnContext(ctx, "Failed to cleanup temporary download.", "error", err)
		}
	}()
	pathSum, err := li.download(ctx, f, int64(freeTmp), uri)
	if err != nil {
		return trace.Errorf("failed to download teleport: %w", err)
	}

	// Seek to the start of the tgz file after writing
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return trace.Errorf("failed seek to start of download: %w", err)
	}
	// Check integrity before decompression
	if !bytes.Equal(newSum, pathSum) {
		return trace.Errorf("mismatched checksum, download possibly corrupt")
	}
	// Get uncompressed size of the tgz
	n, err := uncompressedSize(f)
	if err != nil {
		return trace.Errorf("failed to determine uncompressed size: %w", err)
	}
	// Seek to start of tgz after reading size
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return trace.Errorf("failed seek to start: %w", err)
	}
	if err := li.extract(ctx, versionDir, f, n); err != nil {
		return trace.Errorf("failed to extract teleport: %w", err)
	}
	// Write the checksum last. This marks the version directory as valid.
	err = os.WriteFile(sumPath, []byte(hex.EncodeToString(newSum)), 0755)
	if err != nil {
		return trace.Errorf("failed to write checksum: %w", err)
	}
	return nil
}

// makeURL to download the Teleport tgz.
func makeURL(uriTmpl, version string, flags InstallFlags) (string, error) {
	tmpl, err := template.New("uri").Parse(uriTmpl)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var uriBuf bytes.Buffer
	params := struct {
		OS, Version, Arch string
		FIPS, Enterprise  bool
	}{
		OS:         runtime.GOOS,
		Version:    version,
		Arch:       runtime.GOARCH,
		FIPS:       flags&FlagFIPS != 0,
		Enterprise: flags&(FlagEnterprise|FlagFIPS) != 0,
	}
	err = tmpl.Execute(&uriBuf, params)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return uriBuf.String(), nil
}

// readChecksum from the version directory.
func readChecksum(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()
	var buf bytes.Buffer
	_, err = io.CopyN(&buf, f, checksumHexLen)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := buf.String()
	sum, err := hex.DecodeString(raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sum, nil
}

func (li *LocalInstaller) getChecksum(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := li.HTTP.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, trace.Errorf("checksum not found: %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, resp.Body, checksumHexLen)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sum, err := hex.DecodeString(buf.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sum, nil
}

func (li *LocalInstaller) download(ctx context.Context, w io.Writer, max int64, url string) (sum []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := li.HTTP.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, trace.Errorf("Teleport download not found: %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}
	li.Log.InfoContext(ctx, "Downloading Teleport tarball.", "url", url, "size", resp.ContentLength)

	// Ensure there's enough space in /tmp for the download.
	size := resp.ContentLength
	if size < 0 {
		li.Log.WarnContext(ctx, "Content length missing from response, unable to verify Teleport download size.")
		size = max
	} else if size > max {
		return nil, trace.Errorf("size of download (%d bytes) exceeds available disk space (%d bytes)", resp.ContentLength, max)
	}
	// Calculate checksum concurrently with download.
	shaReader := sha256.New()
	n, err := io.CopyN(w, io.TeeReader(resp.Body, shaReader), size)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return nil, trace.Errorf("mismatch in Teleport download size")
	}
	return shaReader.Sum(nil), nil
}

func (li *LocalInstaller) extract(ctx context.Context, dstDir string, src io.Reader, max int64) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return trace.Wrap(err)
	}
	free, err := utils.FreeDiskWithReserve(dstDir, li.ReservedFreeInstallDisk)
	if err != nil {
		return trace.Errorf("failed to calculate free disk in %q: %w", dstDir, err)
	}
	// Bail if there's not enough free disk space at the target
	if d := int64(free) - max; d < 0 {
		return trace.Errorf("%q needs %d additional bytes of disk space for decompression", dstDir, -d)
	}
	zr, err := gzip.NewReader(src)
	if err != nil {
		return trace.Errorf("requires gzip-compressed body: %v", err)
	}
	li.Log.InfoContext(ctx, "Extracting Teleport tarball.", "path", dstDir, "size", max)

	// TODO(sclevine): add variadic arg to Extract to extract teleport/ subdir into bin/.
	err = utils.Extract(zr, dstDir)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func uncompressedSize(f io.Reader) (int64, error) {
	// NOTE: The gzip length trailer is very unreliable,
	//   but we could optimize this in the future if
	//   we are willing to verify that all published
	//   Teleport tarballs have valid trailers.
	r, err := gzip.NewReader(f)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return n, nil
}
