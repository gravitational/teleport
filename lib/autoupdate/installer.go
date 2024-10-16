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

package autoupdate

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
func (ai *LocalInstaller) Remove(ctx context.Context, version string) error {
	versionDir := filepath.Join(ai.InstallDir, version)
	sumPath := filepath.Join(versionDir, checksumType)

	// invalidate checksum first, to protect against partially-removed
	// directory with valid checksum.
	if err := os.Remove(sumPath); err != nil {
		return trace.Wrap(err)
	}
	if err := os.RemoveAll(versionDir); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Install a Teleport version directory in InstallDir.
func (ai *LocalInstaller) Install(ctx context.Context, version, template string) error {
	versionDir := filepath.Join(ai.InstallDir, version)
	sumPath := filepath.Join(versionDir, checksumType)

	// generate download URI from template
	uri, err := makeURL(template, version)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get new and old checksums. If they match, skip download.
	// Otherwise, clear the old version directory and re-download.
	checksumURI := uri + "." + checksumType
	newSum, err := ai.getChecksum(ctx, checksumURI)
	if err != nil {
		return trace.Errorf("failed to download checksum from %s: %w", checksumURI, err)
	}
	oldSum, err := readChecksum(sumPath)
	if err == nil {
		if bytes.Equal(oldSum, newSum) {
			ai.Log.InfoContext(ctx, "Version already present.", "version", version)
			return nil
		}
		ai.Log.WarnContext(ctx, "Removing version that does not match checksum.", "version", version)
		if err := ai.Remove(ctx, version); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		ai.Log.WarnContext(ctx, "Removing version with unreadable checksum.", "version", version, "error", err)
		if err := ai.Remove(ctx, version); err != nil {
			return trace.Wrap(err)
		}
	}

	// Verify that we have enough free temp space, then download tgz
	freeTmp, err := utils.FreeDiskWithReserve(os.TempDir(), ai.ReservedFreeTmpDisk)
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
			ai.Log.WarnContext(ctx, "Failed to cleanup temporary download.", "error", err)
		}
	}()
	pathSum, err := ai.download(ctx, f, int64(freeTmp), uri)
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
	if err := ai.extract(ctx, versionDir, f, n); err != nil {
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
func makeURL(uriTmpl, version string) (string, error) {
	tmpl, err := template.New("uri").Parse(uriTmpl)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var uriBuf bytes.Buffer
	params := struct {
		OS, Version, Arch string
	}{runtime.GOOS, version, runtime.GOARCH}
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

func (ai *LocalInstaller) getChecksum(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := ai.HTTP.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	// Only attempt to read first 64 bytes
	var buf bytes.Buffer
	_, err = io.CopyN(&buf, resp.Body, 64)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sum, err := hex.DecodeString(buf.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sum, nil
}

func (ai *LocalInstaller) download(ctx context.Context, w io.Writer, max int64, url string) (sum []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := ai.HTTP.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}
	ai.Log.InfoContext(ctx, "Downloading Teleport tarball.", "url", url, "size", resp.ContentLength)

	// Ensure there's enough space in /tmp for the download.
	size := resp.ContentLength
	if size < 0 {
		ai.Log.Warn("Content length missing from response, unable to verify Teleport download size")
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

func (ai *LocalInstaller) extract(ctx context.Context, dstDir string, src io.Reader, max int64) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return trace.Wrap(err)
	}
	free, err := utils.FreeDiskWithReserve(dstDir, ai.ReservedFreeInstallDisk)
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
	ai.Log.InfoContext(ctx, "Extracting Teleport tarball.", "path", dstDir, "size", max)

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
