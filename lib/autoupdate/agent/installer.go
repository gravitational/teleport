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

	"github.com/gravitational/teleport"
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
	// serviceName contains the upstream name of the Teleport SystemD service file.
	serviceName = "teleport.service"
)

// LocalInstaller manages the creation and removal of installations
// of Teleport.
type LocalInstaller struct {
	// InstallDir contains each installation, named by version.
	InstallDir string
	// LinkBinDir contains symlinks to the linked installation's binaries.
	LinkBinDir string
	// CopyServiceFile contains a copy of the linked installation's systemd service.
	CopyServiceFile string
	// SystemBinDir contains binaries for the system (packaged) install of Teleport.
	SystemBinDir string
	// SystemServiceFile contains the systemd service file for the system (packaged) install of Teleport.
	SystemServiceFile string
	// HTTP is an HTTP client for downloading Teleport.
	HTTP *http.Client
	// Log contains a logger.
	Log *slog.Logger
	// ReservedFreeTmpDisk is the amount of disk that must remain free in /tmp
	ReservedFreeTmpDisk uint64
	// ReservedFreeInstallDisk is the amount of disk that must remain free in the install directory.
	ReservedFreeInstallDisk uint64
	// TransformService transforms the systemd service during copying.
	TransformService func([]byte) []byte
	// ValidateBinary returns true if a file is a linkable binary, or
	// false if a file should not be linked.
	ValidateBinary func(ctx context.Context, path string) (bool, error)
}

// Remove a Teleport version directory from InstallDir.
// This function is idempotent.
// See Installer interface for additional specs.
func (li *LocalInstaller) Remove(ctx context.Context, rev Revision) error {
	// os.RemoveAll is dangerous because it can remove an entire directory tree.
	// We must validate the version to ensure that we remove only a single path
	// element under the InstallDir, and not InstallDir or its parents.
	// revisionDir performs these validations.
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return trace.Wrap(err)
	}

	linked, err := li.isLinked(filepath.Join(versionDir, "bin"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err, "failed to determine if linked")
	}
	if linked {
		return trace.Wrap(ErrLinked, "refusing to remove")
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
func (li *LocalInstaller) Install(ctx context.Context, rev Revision, template string) (err error) {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return trace.Wrap(err)
	}
	sumPath := filepath.Join(versionDir, checksumType)

	// generate download URI from template
	uri, err := makeURL(template, rev)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get new and old checksums. If they match, skip download.
	// Otherwise, clear the old version directory and re-download.
	checksumURI := uri + "." + checksumType
	newSum, err := li.getChecksum(ctx, checksumURI)
	if err != nil {
		return trace.Wrap(err, "failed to download checksum from %s", checksumURI)
	}
	oldSum, err := readChecksum(sumPath)
	if err == nil {
		if bytes.Equal(oldSum, newSum) {
			li.Log.InfoContext(ctx, "Version already present.", "version", rev)
			return nil
		}
		li.Log.WarnContext(ctx, "Removing version that does not match checksum.", "version", rev)
		if err := li.Remove(ctx, rev); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		li.Log.WarnContext(ctx, "Removing version with unreadable checksum.", "version", rev, "error", err)
		if err := li.Remove(ctx, rev); err != nil {
			return trace.Wrap(err)
		}
	}

	// Verify that we have enough free temp space, then download tgz
	freeTmp, err := utils.FreeDiskWithReserve(os.TempDir(), li.ReservedFreeTmpDisk)
	if err != nil {
		return trace.Wrap(err, "failed to calculate free disk")
	}
	f, err := os.CreateTemp("", "teleport-update-")
	if err != nil {
		return trace.Wrap(err, "failed to create temporary file")
	}
	defer func() {
		_ = f.Close() // data never read after close
		if err := os.Remove(f.Name()); err != nil {
			li.Log.WarnContext(ctx, "Failed to cleanup temporary download.", "error", err)
		}
	}()
	pathSum, err := li.download(ctx, f, int64(freeTmp), uri)
	if err != nil {
		return trace.Wrap(err, "failed to download teleport")
	}
	// Seek to the start of the tgz file after writing
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return trace.Wrap(err, "failed seek to start of download")
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
		return trace.Wrap(err, "failed to determine uncompressed size")
	}
	// Seek to start of tgz after reading size
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return trace.Wrap(err, "failed seek to start")
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
	if err := li.extract(ctx, versionDir, f, n, rev.Flags); err != nil {
		return trace.Wrap(err, "failed to extract teleport")
	}
	// Write the checksum last. This marks the version directory as valid.
	err = renameio.WriteFile(sumPath, []byte(hex.EncodeToString(newSum)), configFileMode)
	if err != nil {
		return trace.Wrap(err, "failed to write checksum")
	}
	return nil
}

// makeURL to download the Teleport tgz.
func makeURL(uriTmpl string, rev Revision) (string, error) {
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
		Version:    rev.Version,
		Arch:       runtime.GOARCH,
		FIPS:       rev.Flags&FlagFIPS != 0,
		Enterprise: rev.Flags&(FlagEnterprise|FlagFIPS) != 0,
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
	startTime := time.Now()
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
	tee := io.TeeReader(resp.Body, shaReader)
	tee = io.TeeReader(tee, &progressLogger{
		ctx:   ctx,
		log:   li.Log,
		level: slog.LevelInfo,
		name:  path.Base(resp.Request.URL.Path),
		max:   int(resp.ContentLength),
		lines: 5,
	})
	n, err := io.CopyN(w, tee, size)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return nil, trace.Errorf("mismatch in Teleport download size")
	}
	li.Log.InfoContext(ctx, "Download complete.", "duration", time.Since(startTime), "size", n)
	return shaReader.Sum(nil), nil
}

func (li *LocalInstaller) extract(ctx context.Context, dstDir string, src io.Reader, max int64, flags InstallFlags) error {
	if err := os.MkdirAll(dstDir, systemDirMode); err != nil {
		return trace.Wrap(err)
	}
	free, err := utils.FreeDiskWithReserve(dstDir, li.ReservedFreeInstallDisk)
	if err != nil {
		return trace.Wrap(err, "failed to calculate free disk in %s", dstDir)
	}
	// Bail if there's not enough free disk space at the target
	if d := int64(free) - max; d < 0 {
		return trace.Errorf("%s needs %d additional bytes of disk space for decompression", dstDir, -d)
	}
	zr, err := gzip.NewReader(src)
	if err != nil {
		return trace.Wrap(err, "requires gzip-compressed body")
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
func (li *LocalInstaller) List(ctx context.Context) (revs []Revision, err error) {
	entries, err := os.ReadDir(li.InstallDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rev, err := NewRevisionFromDir(entry.Name())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		revs = append(revs, rev)
	}
	return revs, nil
}

// Link the specified version into the system LinkBinDir and CopyServiceFile.
// The revert function restores the previous linking.
// See Installer interface for additional specs.
func (li *LocalInstaller) Link(ctx context.Context, rev Revision) (revert func(context.Context) bool, err error) {
	revert = func(context.Context) bool { return true }
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	revert, err = li.forceLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir, serviceName),
	)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	return revert, nil
}

// LinkSystem links the system (package) version into LinkBinDir and CopyServiceFile.
// The revert function restores the previous linking.
// See Installer interface for additional specs.
func (li *LocalInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	revert, err = li.forceLinks(ctx, li.SystemBinDir, li.SystemServiceFile)
	return revert, trace.Wrap(err)
}

// TryLink links the specified version, but only in the case that
// no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLink(ctx context.Context, revision Revision) error {
	versionDir, err := li.revisionDir(revision)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.tryLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir, serviceName),
	))
}

// TryLinkSystem links the system installation, but only in the case that
// no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLinkSystem(ctx context.Context) error {
	return trace.Wrap(li.tryLinks(ctx, li.SystemBinDir, li.SystemServiceFile))
}

// Unlink unlinks a version from LinkBinDir and CopyServiceFile.
// See Installer interface for additional specs.
func (li *LocalInstaller) Unlink(ctx context.Context, rev Revision) error {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.removeLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir, serviceName),
	))
}

// UnlinkSystem unlinks the system (package) version from LinkBinDir and CopyServiceFile.
// See Installer interface for additional specs.
func (li *LocalInstaller) UnlinkSystem(ctx context.Context) error {
	return trace.Wrap(li.removeLinks(ctx, li.SystemBinDir, li.SystemServiceFile))
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
func (li *LocalInstaller) forceLinks(ctx context.Context, binDir, svcPath string) (revert func(context.Context) bool, err error) {
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
	err = os.MkdirAll(filepath.Dir(li.CopyServiceFile), systemDirMode)
	if err != nil {
		return revert, trace.Wrap(err)
	}

	// create binary links

	entries, err := os.ReadDir(binDir)
	if err != nil {
		return revert, trace.Wrap(err, "failed to find Teleport binary directory")
	}
	var linked int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(binDir, entry.Name())
		newname := filepath.Join(li.LinkBinDir, entry.Name())
		exec, err := li.ValidateBinary(ctx, oldname)
		if err != nil {
			return revert, trace.Wrap(err)
		}
		if !exec {
			continue
		}
		orig, err := forceLink(oldname, newname)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return revert, trace.Wrap(err, "failed to create symlink for %s", entry.Name())
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
		return revert, trace.Wrap(ErrNoBinaries)
	}

	// create systemd service file

	orig, err := li.forceCopyService(li.CopyServiceFile, svcPath, maxServiceFileSize)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return revert, trace.Wrap(err, "failed to copy service")
	}
	if orig != nil {
		revertFiles = append(revertFiles, *orig)
	}
	return revert, nil
}

// forceCopyService uses forceCopy to copy a systemd service file from src to dst.
// The contents of both src and dst must be smaller than n.
// See forceCopy for more details.
func (li *LocalInstaller) forceCopyService(dst, src string, n int64) (orig *smallFile, err error) {
	srcData, err := readFileAtMost(src, n)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return forceCopy(dst, li.TransformService(srcData), n)
}

// forceLink attempts to create a symlink, atomically replacing an existing link if already present.
// If a non-symlink file or directory exists in newname already, forceLink errors.
// If the link is already present with the desired oldname, forceLink returns os.ErrExist.
func forceLink(oldname, newname string) (orig string, err error) {
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

// forceCopy atomically copies a file from srcData to dst, replacing an existing file at dst if needed.
// The contents of dst must be smaller than n.
// forceCopy returns the original file path, mode, and contents as orig.
// If an irregular file, too large file, or directory exists in dst already, forceCopy errors.
// If the file is already present with the desired contents, forceCopy returns os.ErrExist.
func forceCopy(dst string, srcData []byte, n int64) (orig *smallFile, err error) {
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
		orig.data, err = readFileAtMost(dst, n)
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

// readFileAtMost reads a file up to n, or errors if it is too large.
func readFileAtMost(name string, n int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := utils.ReadAtMost(f, n)
	return data, trace.Wrap(err)
}

func (li *LocalInstaller) removeLinks(ctx context.Context, binDir, svcPath string) error {
	removeService := false
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return trace.Wrap(err, "failed to find Teleport binary directory")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(binDir, entry.Name())
		newname := filepath.Join(li.LinkBinDir, entry.Name())
		v, err := os.Readlink(newname)
		if errors.Is(err, os.ErrNotExist) ||
			errors.Is(err, os.ErrInvalid) ||
			errors.Is(err, syscall.EINVAL) {
			li.Log.DebugContext(ctx, "Link not present.", "oldname", oldname, "newname", newname)
			continue
		}
		if err != nil {
			return trace.Wrap(err, "error reading link for %s", filepath.Base(newname))
		}
		if v != oldname {
			li.Log.DebugContext(ctx, "Skipping link to different binary.", "oldname", oldname, "newname", newname)
			continue
		}
		if err := os.Remove(newname); err != nil {
			li.Log.ErrorContext(ctx, "Unable to remove link.", "oldname", oldname, "newname", newname, errorKey, err)
			continue
		}
		if filepath.Base(newname) == teleport.ComponentTeleport {
			removeService = true
		}
	}
	// only remove service if teleport was removed
	if !removeService {
		li.Log.DebugContext(ctx, "Teleport binary not unlinked. Skipping removal of teleport.service.")
		return nil
	}
	srcBytes, err := readFileAtMost(svcPath, maxServiceFileSize)
	if err != nil {
		return trace.Wrap(err)
	}
	dstBytes, err := readFileAtMost(li.CopyServiceFile, maxServiceFileSize)
	if errors.Is(err, os.ErrNotExist) {
		li.Log.DebugContext(ctx, "Service not present.", "path", li.CopyServiceFile)
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal(li.TransformService(srcBytes), dstBytes) {
		li.Log.WarnContext(ctx, "Removed teleport binary link, but skipping removal of custom teleport.service: the service file does not match the reference file for this version. The file might have been manually edited.")
		return nil
	}
	if err := os.Remove(li.CopyServiceFile); err != nil {
		return trace.Wrap(err, "error removing copy of %s", filepath.Base(li.CopyServiceFile))
	}
	return nil
}

// tryLinks create binary and service links for files in binDir and svcDir if links are not already present.
// Existing links that point to files outside binDir or svcDir, as well as existing non-link files, will error.
// tryLinks will not attempt to create any links if linking could result in an error.
// However, concurrent changes to links may result in an error with partially-complete linking.
func (li *LocalInstaller) tryLinks(ctx context.Context, binDir, svcPath string) error {
	// ensure target directories exist before trying to create links
	err := os.MkdirAll(li.LinkBinDir, systemDirMode)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.MkdirAll(filepath.Dir(li.CopyServiceFile), systemDirMode)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate that we can link all system binaries before attempting linking

	var links []symlink
	var linked int
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return trace.Wrap(err, "failed to find Teleport binary directory")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(binDir, entry.Name())
		newname := filepath.Join(li.LinkBinDir, entry.Name())
		exec, err := li.ValidateBinary(ctx, oldname)
		if err != nil {
			return trace.Wrap(err)
		}
		if !exec {
			continue
		}
		ok, err := needsLink(oldname, newname)
		if err != nil {
			return trace.Wrap(err, "error evaluating link for %s", filepath.Base(oldname))
		}
		if ok {
			links = append(links, symlink{oldname, newname})
		}
		linked++
	}
	// bail if no binaries can be linked
	if linked == 0 {
		return trace.Wrap(ErrNoBinaries)
	}

	// link binaries that are missing links
	for _, link := range links {
		if err := os.Symlink(link.oldname, link.newname); err != nil {
			return trace.Wrap(err, "failed to create symlink for %s", filepath.Base(link.oldname))
		}
	}

	// if any binaries are linked from binDir, always link the service from svcDir
	_, err = li.forceCopyService(li.CopyServiceFile, svcPath, maxServiceFileSize)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return trace.Wrap(err, "failed to copy service")
	}

	return nil
}

// needsLink returns true when a symlink from oldname to newname needs to be created, or false if it exists.
// If a non-symlink file or directory exists at newname, needsLink errors.
// If a symlink to a different location exists, needsLink errors with ErrLinked.
func needsLink(oldname, newname string) (ok bool, err error) {
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
		return false, trace.Wrap(ErrLinked, "refusing to replace link at %s", newname)
	}
	return false, nil
}

// revisionDir returns the storage directory for a Teleport revision.
// revisionDir will fail if the revision cannot be used to construct the directory name.
// For example, it ensures that ".." cannot be provided to return a system directory.
func (li *LocalInstaller) revisionDir(rev Revision) (string, error) {
	installDir, err := filepath.Abs(li.InstallDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	versionDir := filepath.Join(installDir, rev.Dir())
	if filepath.Dir(versionDir) != filepath.Clean(installDir) {
		return "", trace.Errorf("refusing to link directory outside of version directory")
	}
	return versionDir, nil
}

// isLinked returns true if any binaries in binDir are linked.
// Returns os.ErrNotExist error if the binDir does not exist.
func (li *LocalInstaller) isLinked(binDir string) (bool, error) {
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
