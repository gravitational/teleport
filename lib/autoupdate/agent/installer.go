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
	"syscall"
	"text/template"
	"time"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	checksumType   = "sha256"
	checksumHexLen = sha256.Size * 2 // bytes to hex
)

var (
	// tgzExtractPaths describes how to extract the Teleport tgz.
	// See utils.Extract for more details on how this list is parsed.
	// Paths must use tarball-style / separators (not filepath).
	tgzExtractPaths = []utils.ExtractPath{
		{Src: "teleport/examples/systemd/teleport.service", Dst: "etc/systemd/teleport.service", DirMode: 0755},
		{Src: "teleport/examples", Skip: true, DirMode: 0755},
		{Src: "teleport/install", Skip: true, DirMode: 0755},
		{Src: "teleport/README.md", Dst: "share/README.md", DirMode: 0755},
		{Src: "teleport/CHANGELOG.md", Dst: "share/CHANGELOG.md", DirMode: 0755},
		{Src: "teleport/VERSION", Dst: "share/VERSION", DirMode: 0755},
		{Src: "teleport", Dst: "bin", DirMode: 0755},
	}

	// servicePath contains the path to the Teleport SystemD service within the version directory.
	servicePath = filepath.Join("etc", "systemd", "teleport.service")
)

// LocalInstaller manages the creation and removal of installations
// of Teleport.
type LocalInstaller struct {
	// InstallDir contains each installation, named by version.
	InstallDir string
	// LinkBinDir contains symlinks to the linked installation's binaries.
	LinkBinDir string
	// LinkServiceDir contains a symlink to the linked installation's systemd service.
	LinkServiceDir string
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
// See Installer interface for additional specs.
func (li *LocalInstaller) Remove(ctx context.Context, version string) error {
	// os.RemoveAll is dangerous because it can remove an entire directory tree.
	// We must validate the version to ensure that we remove only a single path
	// element under the InstallDir, and not InstallDir or its parents.
	// versionDir performs these validations.
	versionDir, err := li.versionDir(version)
	if err != nil {
		return trace.Wrap(err)
	}

	linked, err := li.isLinked(versionDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return trace.Errorf("failed to determine if linked: %w", err)
	}
	if linked {
		return trace.Errorf("refusing to remove: %w", ErrLinked)
	}

	// invalidate checksum first, to protect against partially-removed
	// directory with valid checksum.
	err = os.Remove(filepath.Join(versionDir, checksumType))
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
// See Installer interface for additional specs.
func (li *LocalInstaller) Install(ctx context.Context, version, template string, flags InstallFlags) (err error) {
	versionDir, err := li.versionDir(version)
	if err != nil {
		return trace.Wrap(err)
	}
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

	// If interrupted, close the file immediately to stop extracting.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	context.AfterFunc(ctx, func() {
		_ = f.Close() // safe to close file multiple times
	})
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

	// If there's an error after we start extracting, delete the version dir.
	defer func() {
		if err != nil {
			if err := os.RemoveAll(versionDir); err != nil {
				li.Log.WarnContext(ctx, "Failed to cleanup broken version extraction.", "error", err, "dir", versionDir)
			}
		}
	}()

	// Extract tgz into version directory.
	if err := li.extract(ctx, versionDir, f, n); err != nil {
		return trace.Errorf("failed to extract teleport: %w", err)
	}
	// Write the checksum last. This marks the version directory as valid.
	err = renameio.WriteFile(sumPath, []byte(hex.EncodeToString(newSum)), 0755)
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

	err = utils.Extract(zr, dstDir, tgzExtractPaths...)
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

// List installed versions of Teleport.
func (li *LocalInstaller) List(ctx context.Context) (versions []string, err error) {
	entries, err := os.ReadDir(li.InstallDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versions = append(versions, entry.Name())
	}
	return versions, nil
}

// Link the specified version into the system LinkBinDir and LinkServiceDir.
// The revert function restores the previous linking.
// See Installer interface for additional specs.
func (li *LocalInstaller) Link(ctx context.Context, version string) (revert func(context.Context) bool, err error) {
	// setup revert function
	type symlink struct {
		old, new string
	}
	var revertLinks []symlink
	revert = func(ctx context.Context) bool {
		// This function is safe to call repeatedly.
		// Returns true only when all symlinks are successfully reverted.
		var keep []symlink
		for _, l := range revertLinks {
			err := renameio.Symlink(l.old, l.new)
			if err != nil {
				keep = append(keep, l)
				li.Log.ErrorContext(ctx, "Failed to revert symlink", "old", l.old, "new", l.new, "err", err)
			}
		}
		revertLinks = keep
		return len(revertLinks) == 0
	}
	// revert immediately on error, so caller can ignore revert arg
	defer func() {
		if err != nil {
			revert(ctx)
		}
	}()

	versionDir, err := li.versionDir(version)
	if err != nil {
		return revert, trace.Wrap(err)
	}

	// ensure target directories exist before trying to create links
	err = os.MkdirAll(li.LinkBinDir, 0755)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	err = os.MkdirAll(li.LinkServiceDir, 0755)
	if err != nil {
		return revert, trace.Wrap(err)
	}

	// create binary links

	binDir := filepath.Join(versionDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return revert, trace.Errorf("failed to find Teleport binary directory: %w", err)
	}
	var linked int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(binDir, entry.Name())
		newname := filepath.Join(li.LinkBinDir, entry.Name())
		orig, err := tryLink(oldname, newname)
		if err != nil {
			return revert, trace.Errorf("failed to create symlink for %s: %w", filepath.Base(oldname), err)
		}
		if orig != "" {
			revertLinks = append(revertLinks, symlink{
				old: orig,
				new: newname,
			})
		}
		linked++
	}
	if linked == 0 {
		return revert, trace.Errorf("no binaries available to link")
	}

	// create systemd service link

	oldname := filepath.Join(versionDir, servicePath)
	newname := filepath.Join(li.LinkServiceDir, filepath.Base(servicePath))
	orig, err := tryLink(oldname, newname)
	if err != nil {
		return revert, trace.Errorf("failed to create symlink for %s: %w", filepath.Base(oldname), err)
	}
	if orig != "" {
		revertLinks = append(revertLinks, symlink{
			old: orig,
			new: newname,
		})
	}
	return revert, nil
}

// tryLink attempts to create a symlink, atomically replacing an existing link if already present.
// If a non-symlink file or directory exists in newname already, tryLink errors.
func tryLink(oldname, newname string) (orig string, err error) {
	orig, err = os.Readlink(newname)
	if errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, syscall.EINVAL) { // workaround missing ErrInvalid wrapper
		// important: do not attempt to replace a non-linked install of Teleport
		return orig, trace.Errorf("refusing to replace file at %s", newname)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return orig, trace.Wrap(err)
	}
	if orig == oldname {
		return "", nil
	}
	err = renameio.Symlink(oldname, newname)
	if err != nil {
		return orig, trace.Wrap(err)
	}
	return orig, nil
}

// versionDir returns the storage directory for a Teleport version.
// versionDir will fail if the version cannot be used to construct the directory name.
// For example, it ensures that ".." cannot be provided to return a system directory.
func (li *LocalInstaller) versionDir(version string) (string, error) {
	installDir, err := filepath.Abs(li.InstallDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	versionDir := filepath.Join(installDir, version)
	if filepath.Dir(versionDir) != filepath.Clean(installDir) {
		return "", trace.Errorf("refusing to directory outside of version directory")
	}
	return versionDir, nil
}

// isLinked returns true if any binaries or services in versionDir are linked.
// Returns os.ErrNotExist error if the versionDir does not exist.
func (li *LocalInstaller) isLinked(versionDir string) (bool, error) {
	binDir := filepath.Join(versionDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		v, err := os.Readlink(filepath.Join(li.LinkBinDir, entry.Name()))
		if err != nil {
			continue
		}
		if filepath.Clean(v) == filepath.Join(binDir, entry.Name()) {
			return true, nil
		}
	}
	v, err := os.Readlink(filepath.Join(li.LinkServiceDir, filepath.Base(servicePath)))
	if err != nil {
		return false, nil
	}
	return filepath.Clean(v) ==
		filepath.Join(versionDir, servicePath), nil
}
