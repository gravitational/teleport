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
	checksumType = "sha256"
)

type TeleportVersion struct {
	VersionsDir    string
	DownloadClient *http.Client
}

func (tv *TeleportVersion) Remove(version string) error {
	versionPath := filepath.Join(tv.VersionsDir, version)
	sumPath := filepath.Join(versionPath, checksumType)

	// invalidate checksum first, to protect against partially-removed
	// directory with valid checksum
	if err := os.Remove(sumPath); err != nil {
		return trace.Wrap(err)
	}
	if err := os.RemoveAll(versionPath); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (tv *TeleportVersion) Create(ctx context.Context, uriTmpl, version string) error {
	versionPath := filepath.Join(tv.VersionsDir, version)
	sumPath := filepath.Join(versionPath, checksumType)

	tmpl, err := template.New("uri").Parse(uriTmpl)
	if err != nil {
		return trace.Wrap(err)
	}
	var uriBuf bytes.Buffer
	params := struct {
		OS, Version, Arch string
	}{runtime.GOOS, version, runtime.GOARCH}
	err = tmpl.Execute(&uriBuf, params)
	if err != nil {
		return trace.Wrap(err)
	}
	uri := uriBuf.String()

	sum, err := tv.getChecksum(ctx, uri+"."+checksumType)
	if err != nil {
		return trace.Errorf("failed to download checksum from %s: %w", uri, err)
	}
	existSum, err := readChecksum(ctx, sumPath)
	if err == nil {
		if bytes.Equal(existSum, sum) {
			plog.InfoContext(ctx, "Version already present", "version", version)
			return nil
		}
		plog.WarnContext(ctx, "Removing version that does not match checksum", "version", version)
		if err := tv.Remove(version); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
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

	if !bytes.Equal(sum, pathSum) {
		return trace.Errorf("mismatched checksum, download possibly corrupt")
	}
	// avoid gzip bomb by validating checksum before decompression
	n, err := uncompressedSize(tgz)
	if err != nil {
		return trace.Wrap(err)
	}
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
	if d := free - n; d < 0 {
		return trace.Errorf("%q needs %d additional bytes of disk space for download", versionPath, -d)
	}
	err = untar(tgz, versionPath)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.WriteFile(sumPath, []byte(hex.EncodeToString(sum)), 0755)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func readChecksum(ctx context.Context, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(f, 65))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := buf.String()
	if n != 64 {
		plog.WarnContext(ctx, "unexpected checksum size", "size", n, "checksum", raw)
	}
	sum, err := hex.DecodeString(raw)
	if err != nil {
		plog.WarnContext(ctx, "corrupt checksum detected", "size", n, "checksum", raw)
		return nil, nil
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
	plog.InfoContext(ctx, "Downloading Teleport tarball", "path", f.Name(), "size", resp.ContentLength)

	if resp.ContentLength < 0 {
		plog.Warn("Content length missing from response, unable to verify Teleport download size")
	} else if resp.ContentLength > free {
		return nil, nil, trace.Errorf("size of download (%d bytes) exceeds available disk space (%d bytes)", resp.ContentLength, free)
	}
	shaReader := sha256.New()
	n, err := io.Copy(f, io.TeeReader(resp.Body, shaReader))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return nil, nil, trace.Errorf("mismatch in Teleport download size")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, trace.Errorf("failed seek to start of download: %w", err)
	}
	return rmCloser{f}, shaReader.Sum(nil), nil
}

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

	r := io.LimitReader(resp.Body, 64)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sum, err := hex.DecodeString(buf.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sum, nil
}
