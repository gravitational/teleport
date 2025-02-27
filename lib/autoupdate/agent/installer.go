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
	"path"
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
	// checksumType for Teleport tgzs
	checksumType = "sha256"
	// checksumHexLen is the length of the Teleport checksum.
	checksumHexLen = sha256.Size * 2 // bytes to hex
	// maxServiceFileSize is the maximum size allowed for a systemd service file.
	maxServiceFileSize = 1_000_000 // 1 MB
	// configFileMode is the mode used for new configuration files.
	configFileMode = 0644
	// systemDirMode is the mode used for new directories.
	systemDirMode = 0755
)

const (
	// serviceDir contains the relative path to the Teleport SystemD service dir.
	serviceDir = "lib/systemd/system"
	// serviceName contains the name of the Teleport SystemD service file.
	serviceName = "teleport.service"
	// updateServiceName contains the name of the Teleport Update Systemd service
	updateServiceName = "teleport-update.service"
	// updateTimerName contains the name of the Teleport Update Systemd timer
	updateTimerName = "teleport-update.timer"
)

// LocalInstaller manages the creation and removal of installations
// of Teleport.
type LocalInstaller struct {
	// InstallDir contains each installation, named by version.
	InstallDir string
	// LinkBinDir contains symlinks to the linked installation's binaries.
	LinkBinDir string
	// LinkServiceDir contains a copy of the linked installation's systemd service.
	LinkServiceDir string
	// SystemBinDir contains binaries for the system (packaged) install of Teleport.
	SystemBinDir string
	// SystemServiceDir contains the systemd service file for the system (packaged) install of Teleport.
	SystemServiceDir string
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
	if err := li.extract(ctx, versionDir, f, n, flags); err != nil {
		return trace.Errorf("failed to extract teleport: %w", err)
	}
	// Write the checksum last. This marks the version directory as valid.
	err = renameio.WriteFile(sumPath, []byte(hex.EncodeToString(newSum)), configFileMode)
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

func (li *LocalInstaller) extract(ctx context.Context, dstDir string, src io.Reader, max int64, flags InstallFlags) error {
	if err := os.MkdirAll(dstDir, systemDirMode); err != nil {
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

	err = utils.Extract(zr, dstDir, tgzExtractPaths(flags&(FlagEnterprise|FlagFIPS) != 0)...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// tgzExtractPaths describes how to extract the Teleport tgz.
// See utils.Extract for more details on how this list is parsed.
// Paths must use tarball-style / separators (not filepath).
func tgzExtractPaths(ent bool) []utils.ExtractPath {
	prefix := "teleport"
	if ent {
		prefix += "-ent"
	}
	return []utils.ExtractPath{
		{Src: path.Join(prefix, "examples/systemd/teleport.service"), Dst: filepath.Join(serviceDir, serviceName), DirMode: systemDirMode},
		{Src: path.Join(prefix, "examples"), Skip: true, DirMode: systemDirMode},
		{Src: path.Join(prefix, "install"), Skip: true, DirMode: systemDirMode},
		{Src: path.Join(prefix, "README.md"), Dst: "share/README.md", DirMode: systemDirMode},
		{Src: path.Join(prefix, "CHANGELOG.md"), Dst: "share/CHANGELOG.md", DirMode: systemDirMode},
		{Src: path.Join(prefix, "VERSION"), Dst: "share/VERSION", DirMode: systemDirMode},
		{Src: prefix, Dst: "bin", DirMode: systemDirMode},
	}
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
	revert = func(context.Context) bool { return true }
	versionDir, err := li.versionDir(version)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	revert, err = li.forceLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir),
	)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	return revert, nil
}

// LinkSystem links the system (package) version into LinkBinDir and LinkServiceDir.
// The revert function restores the previous linking.
// See Installer interface for additional specs.
func (li *LocalInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	revert, err = li.forceLinks(ctx, li.SystemBinDir, li.SystemServiceDir)
	return revert, trace.Wrap(err)
}

// symlink from oldname to newname
type symlink struct {
	oldname, newname string
}

// smallFile is a file small enough to be stored in memory.
type smallFile struct {
	name string
	data []byte
	mode os.FileMode
}

// forceLinks replaces binary links and service files using files in binDir and svcDir.
// Existing links and files are replaced, but mismatched links and files will result in error.
// forceLinks will revert any overridden links or files if it hits an error.
// If successful, forceLinks may also be reverted after it returns by calling revert.
// The revert function returns true if reverting succeeds.
func (li *LocalInstaller) forceLinks(ctx context.Context, binDir, svcDir string) (revert func(context.Context) bool, err error) {
	// setup revert function
	var (
		revertLinks []symlink
		revertFiles []smallFile
	)
	revert = func(ctx context.Context) bool {
		// This function is safe to call repeatedly.
		// Returns true only when all changes are successfully reverted.
		var (
			keepLinks []symlink
			keepFiles []smallFile
		)
		for _, l := range revertLinks {
			err := renameio.Symlink(l.oldname, l.newname)
			if err != nil {
				keepLinks = append(keepLinks, l)
				li.Log.ErrorContext(ctx, "Failed to revert symlink", "oldname", l.oldname, "newname", l.newname, errorKey, err)
			}
		}
		for _, f := range revertFiles {
			err := renameio.WriteFile(f.name, f.data, f.mode)
			if err != nil {
				keepFiles = append(keepFiles, f)
				li.Log.ErrorContext(ctx, "Failed to revert files", "name", f.name, errorKey, err)
			}
		}
		revertLinks = keepLinks
		revertFiles = keepFiles
		return len(revertLinks) == 0 && len(revertFiles) == 0
	}
	// revert immediately on error, so caller can ignore revert arg
	defer func() {
		if err != nil {
			revert(ctx)
		}
	}()

	// ensure target directories exist before trying to create links
	err = os.MkdirAll(li.LinkBinDir, systemDirMode)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	err = os.MkdirAll(li.LinkServiceDir, systemDirMode)
	if err != nil {
		return revert, trace.Wrap(err)
	}

	// create binary links

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
		orig, err := forceLink(oldname, newname)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return revert, trace.Errorf("failed to create symlink for %s: %w", filepath.Base(oldname), err)
		}
		if orig != "" {
			revertLinks = append(revertLinks, symlink{
				oldname: orig,
				newname: newname,
			})
		}
		linked++
	}
	if linked == 0 {
		return revert, trace.Errorf("no binaries available to link")
	}

	// create systemd service file

	src := filepath.Join(svcDir, serviceName)
	dst := filepath.Join(li.LinkServiceDir, serviceName)
	orig, err := forceCopy(dst, src, maxServiceFileSize)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return revert, trace.Errorf("failed to write file %s: %w", serviceName, err)
	}
	if orig != nil {
		revertFiles = append(revertFiles, *orig)
	}
	return revert, nil
}

// forceLink attempts to create a symlink, atomically replacing an existing link if already present.
// If a non-symlink file or directory exists in newname already, forceLink errors.
// If the link is already present with the desired oldname, forceLink returns os.ErrExist.
func forceLink(oldname, newname string) (orig string, err error) {
	exec, err := isExecutable(oldname)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if !exec {
		return "", trace.Errorf("%s is not a regular executable file", oldname)
	}
	orig, err = os.Readlink(newname)
	if errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, syscall.EINVAL) { // workaround missing ErrInvalid wrapper
		// important: do not attempt to replace a non-linked install of Teleport
		return "", trace.Errorf("refusing to replace file at %s", newname)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", trace.Wrap(err)
	}
	if orig == oldname {
		return "", trace.Wrap(os.ErrExist)
	}
	err = renameio.Symlink(oldname, newname)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return orig, nil
}

// isExecutable returns true for regular files that are executable by all users (0111).
func isExecutable(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, trace.Wrap(err)
	}
	// TODO(sclevine): verify path is valid binary
	return fi.Mode().IsRegular() &&
		fi.Mode()&0111 == 0111, nil
}

// forceCopy atomically copies a file from src to dst, replacing an existing file at dst if needed.
// Both src and dst must be smaller than n.
// forceCopy returns the original file path, mode, and contents as orig.
// If an irregular file, too large file, or directory exists in path already, forceCopy errors.
// If the file is already present with the desired contents, forceCopy returns os.ErrExist.
func forceCopy(dst, src string, n int64) (orig *smallFile, err error) {
	srcData, err := readFileN(src, n)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fi, err := os.Lstat(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		orig = &smallFile{
			name: dst,
			mode: fi.Mode(),
		}
		if !orig.mode.IsRegular() {
			return nil, trace.Errorf("refusing to replace irregular file at %s", dst)
		}
		orig.data, err = readFileN(dst, n)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if bytes.Equal(srcData, orig.data) {
			return nil, trace.Wrap(os.ErrExist)
		}
	}
	err = renameio.WriteFile(dst, srcData, configFileMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return orig, nil
}

// readFileN reads a file up to n, or errors if it is too large.
func readFileN(name string, n int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := utils.ReadAtMost(f, n)
	return data, trace.Wrap(err)
}

// TryLink links the specified version, but only in the case that
// no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLink(ctx context.Context, version string) error {
	versionDir, err := li.versionDir(version)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.tryLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir),
	))
}

// TryLinkSystem links the system installation, but only in the case that
// no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLinkSystem(ctx context.Context) error {
	return trace.Wrap(li.tryLinks(ctx, li.SystemBinDir, li.SystemServiceDir))
}

// tryLinks create binary and service links for files in binDir and svcDir if links are not already present.
// Existing links that point to files outside binDir or svcDir, as well as existing non-link files, will error.
// tryLinks will not attempt to create any links if linking could result in an error.
// However, concurrent changes to links may result in an error with partially-complete linking.
func (li *LocalInstaller) tryLinks(ctx context.Context, binDir, svcDir string) error {
	// ensure target directories exist before trying to create links
	err := os.MkdirAll(li.LinkBinDir, systemDirMode)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.MkdirAll(li.LinkServiceDir, systemDirMode)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate that we can link all system binaries before attempting linking

	var links []symlink
	var linked int
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return trace.Errorf("failed to find Teleport binary directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(binDir, entry.Name())
		newname := filepath.Join(li.LinkBinDir, entry.Name())
		ok, err := needsLink(oldname, newname)
		if err != nil {
			return trace.Errorf("error evaluating link for %s: %w", filepath.Base(oldname), err)
		}
		if ok {
			links = append(links, symlink{oldname, newname})
		}
		linked++
	}
	// bail if no binaries can be linked
	if linked == 0 {
		return trace.Errorf("no binaries available to link")
	}

	// link binaries that are missing links
	for _, link := range links {
		if err := os.Symlink(link.oldname, link.newname); err != nil {
			return trace.Errorf("failed to create symlink for %s: %w", filepath.Base(link.oldname), err)
		}
	}

	// if any binaries are linked from binDir, always link the service from svcDir
	src := filepath.Join(svcDir, serviceName)
	dst := filepath.Join(li.LinkServiceDir, serviceName)
	_, err = forceCopy(dst, src, maxServiceFileSize)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return trace.Errorf("error writing %s: %w", serviceName, err)
	}

	return nil
}

// needsLink returns true when a symlink from oldname to newname needs to be created, or false if it exists.
// If a non-symlink file or directory exists at newname, needsLink errors.
// If a symlink to a different location exists, needsLink errors with ErrLinked.
func needsLink(oldname, newname string) (ok bool, err error) {
	exec, err := isExecutable(oldname)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !exec {
		return false, trace.Errorf("%s is not a regular executable file", oldname)
	}
	orig, err := os.Readlink(newname)
	if errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, syscall.EINVAL) { // workaround missing ErrInvalid wrapper
		// important: do not attempt to replace a non-linked install of Teleport
		return false, trace.Errorf("refusing to replace file at %s", newname)
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	if orig != oldname {
		return false, trace.Errorf("refusing to replace link at %s: %w", newname, ErrLinked)
	}
	return false, nil
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
		return "", trace.Errorf("refusing to link directory outside of version directory")
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
	return false, nil
}
