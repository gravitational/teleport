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

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

const (
	checksumType   = "sha256"
	checksumHexLen = 64
)

type TeleportVersion struct {
	VersionsDir    string
	URLTemplate    string
	DownloadClient *http.Client
}

// Remove a Teleport version directory from in /var/lib/teleport/versions/.
func (tv *TeleportVersion) Remove(version string) error {
	versionPath := filepath.Join(tv.VersionsDir, version)
	sumPath := filepath.Join(versionPath, checksumType)

	// invalidate checksum first, to protect against partially-removed
	// directory with valid checksum.
	if err := os.Remove(sumPath); err != nil {
		return trace.Wrap(err)
	}
	if err := os.RemoveAll(versionPath); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Create a Teleport version directory in /var/lib/teleport/versions/.
func (tv *TeleportVersion) Create(ctx context.Context, version string) error {
	versionPath := filepath.Join(tv.VersionsDir, version)
	sumPath := filepath.Join(versionPath, checksumType)

	// generate download URI from template
	uri, err := makeURL(tv.URLTemplate, version)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get new and old checksums. If they match, skip download.
	// Otherwise, clear the old version directory and re-download.
	checksumURI := uri + "." + checksumType
	newSum, err := tv.getChecksum(ctx, checksumURI)
	if err != nil {
		return trace.Errorf("failed to download checksum from %s: %w", checksumURI, err)
	}
	oldSum, err := readChecksum(sumPath)
	if err == nil {
		if bytes.Equal(oldSum, newSum) {
			plog.InfoContext(ctx, "Version already present", "version", version)
			return nil
		}
		plog.WarnContext(ctx, "Removing version that does not match checksum", "version", version)
		if err := tv.Remove(version); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		plog.WarnContext(ctx, "Removing version with unreadable checksum", "version", version, "error", err)
		if err := tv.Remove(version); err != nil {
			return trace.Wrap(err)
		}
	}

	tgz, pathSum, err := tv.download(ctx, uri)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := tgz.Close(); err != nil {
			plog.WarnContext(ctx, "Failed to cleanup temporary download after error", "error", err)
		}
	}()

	// Check integrity before decompression
	if !bytes.Equal(newSum, pathSum) {
		return trace.Errorf("mismatched checksum, download possibly corrupt")
	}
	// Get uncompressed size of the tgz
	n, err := uncompressedSize(tgz)
	if err != nil {
		return trace.Wrap(err)
	}
	// Seek to start of tgz after reading size
	if _, err := tgz.Seek(0, io.SeekStart); err != nil {
		return trace.Errorf("failed seek to start: %w", err)
	}
	if err := os.MkdirAll(versionPath, 0755); err != nil {
		return trace.Wrap(err)
	}
	free, err := freeDisk(versionPath)
	if err != nil {
		return trace.Errorf("failed to calculate free disk in %q: %w", versionPath, err)
	}
	// Bail if there's not enough free disk space at the target
	if d := free - n; d < 0 {
		return trace.Errorf("%q needs %d additional bytes of disk space for decompression", versionPath, -d)
	}
	err = untar(tgz, versionPath)
	if err != nil {
		return trace.Wrap(err)
	}
	// Write the checksum last. This marks the version directory as valid.
	err = os.WriteFile(sumPath, []byte(hex.EncodeToString(newSum)), 0755)
	if err != nil {
		return trace.Wrap(err)
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

func (tv *TeleportVersion) download(ctx context.Context, url string) (r io.ReadSeekCloser, sum []byte, err error) {
	f, err := os.CreateTemp("", "teleport-update-")
	if err != nil {
		return nil, nil, trace.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			f.Close()
			if err := os.Remove(f.Name()); err != nil {
				plog.WarnContext(ctx, "Failed to cleanup temporary download after error", "error", err)
			}
		}
	}()
	free, err := freeDisk(os.TempDir())
	if err != nil {
		return nil, nil, trace.Errorf("failed to calculate free disk: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	resp, err := tv.DownloadClient.Do(req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, trace.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}
	plog.InfoContext(ctx, "Downloading Teleport tarball", "path", f.Name(), "size", resp.ContentLength)

	// Ensure there's enough space in /tmp for the download.
	size := resp.ContentLength
	if size < 0 {
		plog.Warn("Content length missing from response, unable to verify Teleport download size")
	} else if size > free {
		return nil, nil, trace.Errorf("size of download (%d bytes) exceeds available disk space (%d bytes)", resp.ContentLength, free)
	}

	// Calculate checksum concurrently with download.
	shaReader := sha256.New()
	n, err := io.Copy(f, io.TeeReader(io.LimitReader(resp.Body, size), shaReader))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return nil, nil, trace.Errorf("mismatch in Teleport download size")
	}

	// Seek to the start of the temp file after writing
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, trace.Errorf("failed seek to start of download: %w", err)
	}
	return rmCloser{f}, shaReader.Sum(nil), nil
}

// rmCloser removes a file from disk after it is closed.
type rmCloser struct {
	*os.File
}

func (r rmCloser) Close() error {
	err := r.File.Close()
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Remove(r.File.Name())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const reservedFreeDisk = 10_000_000 // 10 MiB

func freeDisk(dir string) (int64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	avail := int64(stat.Bavail * uint64(stat.Bsize))
	if avail < 0 {
		return 0, trace.Errorf("invalid size")
	}
	return avail - reservedFreeDisk, nil
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

func (tv *TeleportVersion) getChecksum(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := tv.DownloadClient.Do(req)
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
