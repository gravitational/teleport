//go:build unix

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// maxServiceFileSize is the maximum size allowed for a systemd service file.
	maxServiceFileSize = 1_000_000 // 1 MB
)

// Link the specified version into pathDir and TargetServiceFile.
// The revert function restores the previous linking.
// If force is true, Link will overwrite files that are not symlinks.
// See Installer interface for additional specs.
func (li *LocalInstaller) Link(ctx context.Context, rev Revision, pathDir string, force bool) (revert func(context.Context) bool, err error) {
	revert = func(context.Context) bool { return true }
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	revert, err = li.forceLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir),
		pathDir, force, rev.Flags,
	)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	return revert, nil
}

// LinkSystem links the system (package) version into defaultPathDir and TargetServiceFile.
// This prevents namespaced installations in /opt/teleport from linking to the system package.
// The revert function restores the previous linking.
// See Installer interface for additional specs.
func (li *LocalInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	// The system package service file is always removed without flags, so pass
	// no flags here to match the behavior.
	revert, err = li.forceLinks(ctx, li.SystemBinDir, li.SystemServiceDir, defaultPathDir, false, 0)
	return revert, trace.Wrap(err)
}

// TryLink links the specified version into pathDir, but only in the case that
// no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLink(ctx context.Context, revision Revision, pathDir string) error {
	versionDir, err := li.revisionDir(revision)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.tryLinks(ctx,
		filepath.Join(versionDir, "bin"),
		filepath.Join(versionDir, serviceDir),
		pathDir, revision.Flags,
	))
}

// TryLinkSystem links the system installation to defaultPathDir, but only in the case that
// no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLinkSystem(ctx context.Context) error {
	// The system package service file is always removed without flags, so pass
	// no flags here to match the behavior.
	return trace.Wrap(li.tryLinks(ctx, li.SystemBinDir, li.SystemServiceDir, defaultPathDir, 0))
}

// Unlink unlinks a version from pathDir and TargetServiceFile.
// See Installer interface for additional specs.
func (li *LocalInstaller) Unlink(ctx context.Context, rev Revision, pathDir string) error {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.removeLinks(ctx, filepath.Join(versionDir, "bin"), pathDir))
}

// UnlinkSystem unlinks the system (package) version from defaultPathDir and TargetServiceFile.
// See Installer interface for additional specs.
func (li *LocalInstaller) UnlinkSystem(ctx context.Context) error {
	return trace.Wrap(li.removeLinks(ctx, li.SystemBinDir, defaultPathDir))
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
// If force is true, non-link files will be overwritten.
func (li *LocalInstaller) forceLinks(ctx context.Context, srcBinDir, srcSvcDir, dstBinDir string, force bool, flags autoupdate.InstallFlags) (revert func(context.Context) bool, err error) {
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
			err := atomicSymlink(l.oldname, l.newname)
			if err != nil {
				keepLinks = append(keepLinks, l)
				li.Log.ErrorContext(ctx, "Failed to revert symlink", "oldname", l.oldname, "newname", l.newname, errorKey, err)
			}
		}
		for _, f := range revertFiles {
			err := writeFileAtomicWithinDir(f.name, f.data, f.mode)
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

	// ensure source directory exists
	entries, err := os.ReadDir(srcBinDir)
	if errors.Is(err, os.ErrNotExist) {
		return revert, trace.Wrap(ErrNoBinaries)
	}
	if err != nil {
		return revert, trace.Wrap(err, "failed to read Teleport binary directory")
	}

	// ensure target directories exist before trying to create links
	err = os.MkdirAll(dstBinDir, systemDirMode)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	for _, s := range li.TargetServices {
		err = os.MkdirAll(filepath.Dir(s.Path), systemDirMode)
		if err != nil {
			return revert, trace.Wrap(err)
		}
	}

	// create binary links
	var linked int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(srcBinDir, entry.Name())
		newname := filepath.Join(dstBinDir, entry.Name())
		exec, err := li.ValidateBinary(ctx, oldname)
		if err != nil {
			return revert, trace.Wrap(err)
		}
		if !exec {
			continue
		}
		orig, err := forceLink(oldname, newname, force)
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

	// process systemd service files

	for _, s := range li.TargetServices {
		orig, err := copyService(s, srcSvcDir, dstBinDir, flags)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return revert, trace.Wrap(err, "failed to copy service %s", filepath.Base(s.Path))
		}
		if orig != nil {
			revertFiles = append(revertFiles, *orig)
		}
	}

	return revert, nil
}

// copyService copies a systemd service file from src to dst.
// The contents of both src and dst must be smaller than n.
//
// Copied data is processed by s.ExampleFunc.
// If s.ExampleFunc nil, no data is copied, but the original file contents are still returned.
//
// See prepCopy and forceCopy for more details.
func copyService(s ServiceFile, exampleDir string, dstBinDir string, flags autoupdate.InstallFlags) (orig *smallFile, err error) {
	const n = maxServiceFileSize
	if s.ExampleFunc != nil {
		srcData, err := readFileAtMost(filepath.Join(exampleDir, s.ExampleName), n)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		orig, err = forceCopy(s.Path, s.ExampleFunc(srcData, dstBinDir, flags), n)
		return orig, trace.Wrap(err)
	}
	orig, err = prepCopy(s.Path, n)
	return orig, trace.Wrap(err)
}

// forceLink attempts to create a symlink, atomically replacing an existing link if already present.
// If a non-symlink file or directory exists in newname already and force is false, forceLink errors with ErrFilePresent.
// If the link is already present with the desired oldname, forceLink returns os.ErrExist.
func forceLink(oldname, newname string, force bool) (orig string, err error) {
	orig, err = os.Readlink(newname)
	if errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, syscall.EINVAL) { // workaround missing ErrInvalid wrapper
		if force {
			return "", trace.Wrap(atomicSymlink(oldname, newname))
		}
		// important: do not attempt to replace a non-linked install of Teleport without force
		return "", trace.Wrap(ErrFilePresent, "refusing to replace file at %s", newname)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", trace.Wrap(err)
	}
	if orig == oldname {
		return "", trace.Wrap(os.ErrExist)
	}
	err = atomicSymlink(oldname, newname)
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
	orig, err = prepCopy(dst, n)
	if err != nil {
		return orig, trace.Wrap(err)
	}
	if orig != nil && bytes.Equal(srcData, orig.data) {
		return nil, trace.Wrap(os.ErrExist)
	}
	err = writeFileAtomicWithinDir(dst, srcData, configFileMode)
	if err != nil {
		return orig, trace.Wrap(err)
	}
	return orig, nil
}

// prepCopy validates and returns a preserved original copy of a file with
// length <= n at the path specified by dst.
func prepCopy(dst string, n int64) (orig *smallFile, err error) {
	fi, err := os.Lstat(dst)
	if errors.Is(err, os.ErrNotExist) {
		return orig, nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
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

func (li *LocalInstaller) removeLinks(ctx context.Context, srcBinDir, dstBinDir string) error {
	entries, err := os.ReadDir(srcBinDir)
	if err != nil {
		return trace.Wrap(err, "failed to find Teleport binary directory")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(srcBinDir, entry.Name())
		newname := filepath.Join(dstBinDir, entry.Name())
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

		for _, s := range li.TargetServices {
			if filepath.Base(newname) != s.Binary {
				continue
			}
			// binRev is either the version or "system"
			binRev := filepath.Base(filepath.Dir(filepath.Dir(oldname)))
			rev, err := NewRevisionFromDir(binRev)
			if err != nil {
				li.Log.DebugContext(ctx, "Service not present.", "path", s.Path)
				continue
			}
			revMarker := genMarker(rev)
			diskMarker, err := readFileLimit(s.Path, int64(len(revMarker)))
			if errors.Is(err, os.ErrNotExist) {
				li.Log.DebugContext(ctx, "Service not present.", "path", s.Path)
				continue
			}
			if err != nil {
				return trace.Wrap(err)
			}
			// Note that old versions of teleport-update will install services without the marker.
			// Certain version combinations (before and after this commit) may leave services behind
			// if they are not replaced by the new version of teleport-update. This should only impact
			// explicit system package unlinking, which is rarely used.
			if string(diskMarker) != revMarker {
				li.Log.WarnContext(ctx, "Removed binary link, but skipping removal of custom service that does not match the binary.",
					"service", filepath.Base(s.Path), "binary", filepath.Base(newname))
				continue
			}
			if err := os.Remove(s.Path); err != nil {
				return trace.Wrap(err, "error removing copy of %s", filepath.Base(s.Path))
			}
		}
	}
	return nil
}

// tryLinks create binary and service links for files in binDir and svcDir if links are not already present.
// Existing links that point to files outside binDir or svcDir, as well as existing non-link files, will error.
// tryLinks will not attempt to create any links if linking could result in an error.
// However, concurrent changes to links may result in an error with partially-complete linking.
func (li *LocalInstaller) tryLinks(ctx context.Context, srcBinDir, srcSvcDir, dstBinDir string, flags autoupdate.InstallFlags) error {
	// ensure source directory exists
	entries, err := os.ReadDir(srcBinDir)
	if errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(ErrNoBinaries)
	}
	if err != nil {
		return trace.Wrap(err, "failed to read Teleport binary directory")
	}

	// ensure target directories exist before trying to create links
	err = os.MkdirAll(dstBinDir, systemDirMode)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, s := range li.TargetServices {
		err = os.MkdirAll(filepath.Dir(s.Path), systemDirMode)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// validate that we can link all system binaries before attempting linking
	var links []symlink
	var linked int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		oldname := filepath.Join(srcBinDir, entry.Name())
		newname := filepath.Join(dstBinDir, entry.Name())
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

	// process systemd service files
	for _, s := range li.TargetServices {
		_, err := copyService(s, srcSvcDir, dstBinDir, flags)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return trace.Wrap(err, "failed to copy service %s", filepath.Base(s.Path))
		}
	}

	return nil
}

// needsLink returns true when a symlink from oldname to newname needs to be created, or false if it exists.
// If a non-symlink file or directory exists at newname, needsLink errors with ErrFilePresent.
// If a symlink to a different location exists, needsLink errors with ErrLinked.
func needsLink(oldname, newname string) (ok bool, err error) {
	orig, err := os.Readlink(newname)
	if errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, syscall.EINVAL) { // workaround missing ErrInvalid wrapper
		// important: do not attempt to replace a non-linked install of Teleport
		return false, trace.Wrap(ErrFilePresent, "refusing to replace file at %s", newname)
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

// IsLinked returns true if any binaries for Revision rev are linked to pathDir.
// Returns os.ErrNotExist error if the revision does not exist.
func (li *LocalInstaller) IsLinked(ctx context.Context, rev Revision, pathDir string) (bool, error) {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return false, trace.Wrap(err)
	}
	binDir := filepath.Join(versionDir, "bin")
	entries, err := os.ReadDir(binDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		v, err := os.Readlink(filepath.Join(pathDir, entry.Name()))
		if err != nil {
			continue
		}
		if filepath.Clean(v) == filepath.Join(binDir, entry.Name()) {
			return true, nil
		}
	}
	return false, nil
}
