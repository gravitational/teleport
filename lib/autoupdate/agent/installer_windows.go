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
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// On Windows, there is no symlink-based linking of binaries into a PATH directory.
// Instead, the binPath of the Windows service (SCM) for each TargetServices entry
// acts as the "link" to the active revision: linking a revision points the service
// at versions\<rev>\bin\<binary>.exe, and reverting points it back at the previous binPath.
//
// Additionally, pathDir (e.g., C:\Program Files\Teleport\bin) is made a directory
// junction pointing at versions\<rev>\bin, so that the active revision's tools are
// available on PATH. Junctions, unlike symlinks, require no special privileges.
//
// Within TargetServices, ServiceFile.Path is interpreted as the Windows service name,
// and ServiceFile.Binary as the name of the binary (with an implied .exe extension).
//
// There is no system (package) installation of Teleport on Windows, so the *System
// methods return ErrNotSupported.

// scmConfigAccess is required to read and change the binPath of a Windows service.
const scmConfigAccess = windows.SERVICE_QUERY_CONFIG | windows.SERVICE_CHANGE_CONFIG

// serviceBinPath records the binPath of a Windows service, for reverting.
type serviceBinPath struct {
	service string
	binPath string
}

// serviceBinary associates a Windows service with the binary it should run.
type serviceBinary struct {
	service string
	exe     string
}

// junction records a directory junction from link to target, for reverting.
// An empty target means the junction did not exist.
type junction struct {
	link   string
	target string
}

// Link the specified version into the Windows services in TargetServices by
// updating their binPath, and into pathDir by creating a junction to the version's bin directory.
// The revert function restores the previous binPaths and junction.
// If force is true, Link will overwrite binPaths that do not point into InstallDir,
// and will replace a file or empty directory at pathDir.
// See Installer interface for additional specs.
func (li *LocalInstaller) Link(ctx context.Context, rev Revision, pathDir string, force bool) (revert func(context.Context) bool, err error) {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	revert, err = li.createBinPaths(ctx, filepath.Join(versionDir, "bin"), pathDir, force)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return revert, nil
}

// LinkSystem is not supported on Windows, as there is no system (package) installation of Teleport.
// See Installer interface for additional specs.
func (li *LocalInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	return nil, trace.Wrap(ErrNoBinaries)
}

// TryLink links the specified version into the Windows services in TargetServices
// by updating their binPath, and into pathDir by creating a junction, but only in
// the case that no installation of Teleport is already linked or partially linked.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLink(ctx context.Context, rev Revision, pathDir string) error {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.tryBinPaths(ctx, filepath.Join(versionDir, "bin"), pathDir))
}

// TryLinkSystem is not supported on Windows, as there is no system (package) installation of Teleport.
// See Installer interface for additional specs.
func (li *LocalInstaller) TryLinkSystem(ctx context.Context) error {
	return trace.Wrap(ErrNotSupported)
}

// Unlink unlinks a version from the Windows services in TargetServices and from pathDir.
// Services pointing at the version are re-pointed at the (non-versioned) binary in pathDir,
// preserving service arguments, and the pathDir junction is removed if it points at the version,
// so that the version directory can be safely removed.
// See Installer interface for additional specs.
func (li *LocalInstaller) Unlink(ctx context.Context, rev Revision, pathDir string) error {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(li.removeBinPaths(ctx, filepath.Join(versionDir, "bin"), pathDir))
}

// UnlinkSystem is not supported on Windows, as there is no system (package) installation of Teleport.
// See Installer interface for additional specs.
func (li *LocalInstaller) UnlinkSystem(ctx context.Context) error {
	return trace.Wrap(ErrNotSupported)
}

// IsLinked returns true if any Windows service in TargetServices points at a binary of Revision rev,
// or if pathDir is a junction pointing at the bin directory of Revision rev.
func (li *LocalInstaller) IsLinked(ctx context.Context, rev Revision, pathDir string) (bool, error) {
	versionDir, err := li.revisionDir(rev)
	if err != nil {
		return false, trace.Wrap(err)
	}
	binDir := filepath.Join(versionDir, "bin")
	target, err := readJunction(pathDir)
	if err == nil && samePath(target, binDir) {
		return true, nil
	}
	for _, s := range li.TargetServices {
		service, err := openService(s.Path, windows.SERVICE_QUERY_CONFIG)
		if isServiceNotFound(err) {
			continue
		}
		if err != nil {
			return false, trace.Wrap(err)
		}
		cfg, err := service.Config()
		service.Close()
		if err != nil {
			return false, trace.Wrap(err, "reading configuration of Windows service %s", s.Path)
		}
		exe, _ := splitBinPath(cfg.BinaryPathName)
		if samePath(exe, filepath.Join(binDir, windowsBinaryName(s.Binary))) {
			return true, nil
		}
	}
	return false, nil
}

// createBinPaths points the Windows services in TargetServices at the binaries in binDir,
// and creates a junction from dstBinDir to binDir.
// Existing binPaths pointing into InstallDir and existing junctions are replaced, but binPaths
// pointing elsewhere and non-junction files at dstBinDir will result in ErrFilePresent unless force is true.
// createBinPaths will revert any overridden binPaths and junctions if it hits an error.
// If successful, createBinPaths may also be reverted after it returns by calling revert.
// The revert function returns true if reverting succeeds.
func (li *LocalInstaller) createBinPaths(ctx context.Context, binDir, dstBinDir string, force bool) (revert func(context.Context) bool, err error) {
	// setup revert function
	var (
		revertPaths     []serviceBinPath
		revertJunctions []junction
	)
	revert = func(ctx context.Context) bool {
		// This function is safe to call repeatedly.
		// Returns true only when all changes are successfully reverted.
		var (
			keepPaths     []serviceBinPath
			keepJunctions []junction
		)
		for _, p := range revertPaths {
			if err := setServiceBinPath(p.service, p.binPath); err != nil {
				keepPaths = append(keepPaths, p)
				li.Log.ErrorContext(ctx, "Failed to revert Windows service binPath", unitKey, p.service, "binPath", p.binPath, errorKey, err)
			}
		}
		for _, j := range revertJunctions {
			if err := revertJunction(j); err != nil {
				keepJunctions = append(keepJunctions, j)
				li.Log.ErrorContext(ctx, "Failed to revert junction", "link", j.link, "target", j.target, errorKey, err)
			}
		}
		revertPaths = keepPaths
		revertJunctions = keepJunctions
		return len(revertPaths) == 0 && len(revertJunctions) == 0
	}
	// revert immediately on error, so caller can ignore revert arg
	defer func() {
		if err != nil {
			revert(ctx)
		}
	}()

	managedDir, err := filepath.Abs(li.InstallDir)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	bins, err := li.linkableServices(ctx, binDir)
	if err != nil {
		return revert, trace.Wrap(err)
	}
	for _, b := range bins {
		service, err := openService(b.service, scmConfigAccess)
		if isServiceNotFound(err) {
			li.Log.DebugContext(ctx, "Windows service not present, skipping.", unitKey, b.service)
			continue
		}
		if err != nil {
			return revert, trace.Wrap(err)
		}
		orig, err := forceBinPath(service, b.exe, managedDir, force)
		service.Close()
		if err != nil && !errors.Is(err, os.ErrExist) {
			return revert, trace.Wrap(err, "failed to set binPath for Windows service %s", b.service)
		}
		if orig != "" {
			revertPaths = append(revertPaths, serviceBinPath{
				service: b.service,
				binPath: orig,
			})
		}
	}

	// create junction to bin directory
	orig, changed, err := createJunction(dstBinDir, binDir, force)
	if err != nil {
		return revert, trace.Wrap(err, "failed to create junction at %s", dstBinDir)
	}
	if changed {
		revertJunctions = append(revertJunctions, junction{
			link:   dstBinDir,
			target: orig,
		})
	}
	return revert, nil
}

// createJunction creates or replaces the junction at link so that it points to target.
// If a non-junction file or empty directory exists at link and force is true, it is replaced.
// If a non-junction exists at link and force is false, createJunction errors with ErrFilePresent.
// If the junction is already present with the desired target, createJunction returns changed=false.
// Otherwise, the original target is returned as orig ("" if no junction existed).
func createJunction(link, target string, force bool) (orig string, changed bool, err error) {
	orig, err = readJunction(link)
	switch {
	case errors.Is(err, os.ErrNotExist):
		orig = ""
	case errors.Is(err, ErrFilePresent):
		if !force {
			// important: do not attempt to replace a non-linked install of Teleport without force
			return "", false, trace.Wrap(ErrFilePresent, "refusing to replace file at %s", link)
		}
		// Only files and empty directories can be removed; refuse to destroy directory contents.
		if err := os.Remove(link); err != nil {
			return "", false, trace.Wrap(trace.ConvertSystemError(err), "failed to replace non-empty directory at %s", link)
		}
		orig = ""
	case err != nil:
		return "", false, trace.Wrap(err)
	case samePath(orig, target):
		return "", false, nil
	}
	if err := os.MkdirAll(filepath.Dir(link), systemDirMode); err != nil {
		return "", false, trace.Wrap(err)
	}
	if err := setJunction(link, target); err != nil {
		return "", false, trace.Wrap(err)
	}
	return orig, true, nil
}

// revertJunction restores the junction recorded in j.
// If j.target is empty, the junction is removed.
func revertJunction(j junction) error {
	if j.target == "" {
		err := os.Remove(j.link)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return trace.ConvertSystemError(err)
		}
		return nil
	}
	return trace.Wrap(setJunction(j.link, j.target))
}

// needsJunction returns true when a junction from link to target needs to be created, or false if it exists.
// If a non-junction file or directory exists at link, needsJunction errors with ErrFilePresent.
// If a junction to a different location exists, needsJunction errors with ErrLinked.
func needsJunction(link, target string) (bool, error) {
	orig, err := readJunction(link)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if errors.Is(err, ErrFilePresent) {
		// important: do not attempt to replace a non-linked install of Teleport
		return false, trace.Wrap(ErrFilePresent, "refusing to replace file at %s", link)
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !samePath(orig, target) {
		return false, trace.Wrap(ErrLinked, "refusing to replace junction at %s", link)
	}
	return false, nil
}

// forceBinPath points service at exe, preserving any service arguments.
// If the service already points outside of managedDir and force is false, forceBinPath errors with ErrFilePresent.
// If the service already points at exe, forceBinPath returns os.ErrExist.
// Otherwise, the original binPath is returned.
func forceBinPath(service *mgr.Service, exe, managedDir string, force bool) (orig string, err error) {
	cfg, err := service.Config()
	if err != nil {
		return "", trace.Wrap(err, "reading service configuration")
	}
	curExe, args := splitBinPath(cfg.BinaryPathName)
	if samePath(curExe, exe) {
		return "", trace.Wrap(os.ErrExist)
	}
	// important: do not attempt to replace a non-linked install of Teleport without force
	if curExe != "" && !isWithinDir(managedDir, curExe) && !force {
		return "", trace.Wrap(ErrFilePresent, "refusing to replace binPath %s", curExe)
	}
	if err := changeServiceBinPath(service, joinBinPath(exe, args)); err != nil {
		return "", trace.Wrap(err)
	}
	return cfg.BinaryPathName, nil
}

// tryBinPaths points the Windows services in TargetServices at the binaries in binDir
// if they are not already pointing at them, and creates a junction from dstBinDir to binDir
// if it is not already present.
// Services and junctions pointing into InstallDir will error with ErrLinked, and services
// and files pointing elsewhere will error with ErrFilePresent.
// tryBinPaths will not attempt to change any binPath if doing so could result in an error.
// However, concurrent changes to services may result in an error with partially-complete linking.
func (li *LocalInstaller) tryBinPaths(ctx context.Context, binDir, dstBinDir string) error {
	managedDir, err := filepath.Abs(li.InstallDir)
	if err != nil {
		return trace.Wrap(err)
	}
	bins, err := li.linkableServices(ctx, binDir)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate that we can create the junction before attempting linking
	needJunction, err := needsJunction(dstBinDir, binDir)
	if err != nil {
		return trace.Wrap(err, "error evaluating junction at %s", dstBinDir)
	}

	// validate that we can link all services before attempting linking
	var changes []serviceBinPath
	for _, b := range bins {
		service, err := openService(b.service, windows.SERVICE_QUERY_CONFIG)
		if isServiceNotFound(err) {
			li.Log.DebugContext(ctx, "Windows service not present, skipping.", unitKey, b.service)
			continue
		}
		if err != nil {
			return trace.Wrap(err)
		}
		cfg, err := service.Config()
		service.Close()
		if err != nil {
			return trace.Wrap(err, "reading configuration of Windows service %s", b.service)
		}
		curExe, args := splitBinPath(cfg.BinaryPathName)
		switch {
		case samePath(curExe, b.exe):
			continue
		case curExe == "":
		case isWithinDir(managedDir, curExe):
			return trace.Wrap(ErrLinked, "refusing to replace binPath of Windows service %s", b.service)
		default:
			// important: do not attempt to replace a non-linked install of Teleport
			return trace.Wrap(ErrFilePresent, "refusing to replace binPath %s of Windows service %s", curExe, b.service)
		}
		changes = append(changes, serviceBinPath{
			service: b.service,
			binPath: joinBinPath(b.exe, args),
		})
	}

	// link services that are missing links
	for _, c := range changes {
		if err := setServiceBinPath(c.service, c.binPath); err != nil {
			return trace.Wrap(err, "failed to set binPath for Windows service %s", c.service)
		}
	}

	// create junction if missing
	if needJunction {
		if err := os.MkdirAll(filepath.Dir(dstBinDir), systemDirMode); err != nil {
			return trace.Wrap(err)
		}
		if err := setJunction(dstBinDir, binDir); err != nil {
			return trace.Wrap(err, "failed to create junction at %s", dstBinDir)
		}
	}
	return nil
}

// removeBinPaths re-points any Windows service in TargetServices that points at a binary
// in srcBinDir to the same binary name in dstBinDir, preserving service arguments,
// and removes the junction at dstBinDir if it points at srcBinDir.
// Services and junctions pointing elsewhere are left unchanged.
func (li *LocalInstaller) removeBinPaths(ctx context.Context, srcBinDir, dstBinDir string) error {
	target, err := readJunction(dstBinDir)
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, ErrFilePresent):
		li.Log.DebugContext(ctx, "Junction not present.", "link", dstBinDir)
	case err != nil:
		return trace.Wrap(err, "error reading junction at %s", dstBinDir)
	case !samePath(target, srcBinDir):
		li.Log.DebugContext(ctx, "Skipping junction to different directory.", "link", dstBinDir, "target", target)
	default:
		if err := os.Remove(dstBinDir); err != nil {
			li.Log.ErrorContext(ctx, "Unable to remove junction.", "link", dstBinDir, "target", target, errorKey, err)
		}
	}
	for _, s := range li.TargetServices {
		name := windowsBinaryName(s.Binary)
		oldname := filepath.Join(srcBinDir, name)
		newname := filepath.Join(dstBinDir, name)
		service, err := openService(s.Path, scmConfigAccess)
		if isServiceNotFound(err) {
			li.Log.DebugContext(ctx, "Windows service not present.", unitKey, s.Path)
			continue
		}
		if err != nil {
			return trace.Wrap(err)
		}
		err = func() error {
			defer service.Close()
			cfg, err := service.Config()
			if err != nil {
				return trace.Wrap(err, "reading configuration of Windows service %s", s.Path)
			}
			curExe, args := splitBinPath(cfg.BinaryPathName)
			if !samePath(curExe, oldname) {
				li.Log.DebugContext(ctx, "Skipping Windows service pointing at different binary.", unitKey, s.Path, "binPath", curExe)
				return nil
			}
			if err := changeServiceBinPath(service, joinBinPath(newname, args)); err != nil {
				return trace.Wrap(err, "failed to unlink Windows service %s", s.Path)
			}
			return nil
		}()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// linkableServices returns the services in TargetServices whose binary is present
// in binDir and validates as a linkable binary.
// Services whose binary is not shipped in binDir are skipped.
func (li *LocalInstaller) linkableServices(ctx context.Context, binDir string) ([]serviceBinary, error) {
	// ensure source directory exists
	if _, err := os.Stat(binDir); errors.Is(err, os.ErrNotExist) {
		return nil, trace.Wrap(ErrNoBinaries)
	} else if err != nil {
		return nil, trace.Wrap(err, "failed to read Teleport binary directory")
	}
	var bins []serviceBinary
	for _, s := range li.TargetServices {
		exe := filepath.Join(binDir, windowsBinaryName(s.Binary))
		if _, err := os.Stat(exe); errors.Is(err, os.ErrNotExist) {
			li.Log.DebugContext(ctx, "Binary not present for Windows service.", unitKey, s.Path, "binary", exe)
			continue
		} else if err != nil {
			return nil, trace.Wrap(err)
		}
		ok, err := li.ValidateBinary(ctx, exe)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !ok {
			continue
		}
		bins = append(bins, serviceBinary{service: s.Path, exe: exe})
	}
	if len(bins) == 0 {
		return nil, trace.Wrap(ErrNoBinaries)
	}
	return bins, nil
}

// setServiceBinPath opens the named Windows service and sets its binPath.
func setServiceBinPath(name, binPath string) error {
	service, err := openService(name, scmConfigAccess)
	if err != nil {
		return trace.Wrap(err)
	}
	defer service.Close()
	return trace.Wrap(changeServiceBinPath(service, binPath))
}

// changeServiceBinPath sets the binPath of an open Windows service, leaving all other configuration unchanged.
func changeServiceBinPath(service *mgr.Service, binPath string) error {
	p, err := windows.UTF16PtrFromString(binPath)
	if err != nil {
		return trace.Wrap(err, "converting binPath to UTF16")
	}
	err = windows.ChangeServiceConfig(service.Handle,
		windows.SERVICE_NO_CHANGE, // serviceType
		windows.SERVICE_NO_CHANGE, // startType
		windows.SERVICE_NO_CHANGE, // errorControl
		p,                         // binaryPathName
		nil, nil, nil, nil, nil, nil)
	return trace.Wrap(err, "changing binPath of Windows service %s", service.Name)
}

// windowsBinaryName returns the file name of a binary, adding the .exe extension if missing.
func windowsBinaryName(name string) string {
	if filepath.Ext(name) == "" {
		return name + ".exe"
	}
	return name
}

// splitBinPath splits a Windows service binPath into the executable path and its arguments.
// The executable may be double-quoted; an unquoted executable ends at the first space.
func splitBinPath(binPath string) (exe, args string) {
	binPath = strings.TrimLeft(binPath, " ")
	if rest, ok := strings.CutPrefix(binPath, `"`); ok {
		exe, args, _ = strings.Cut(rest, `"`)
		return exe, strings.TrimLeft(args, " ")
	}
	exe, args, _ = strings.Cut(binPath, " ")
	return exe, strings.TrimLeft(args, " ")
}

// joinBinPath builds a Windows service binPath from a quoted executable path and its arguments.
func joinBinPath(exe, args string) string {
	if args == "" {
		return `"` + exe + `"`
	}
	return `"` + exe + `" ` + args
}

// samePath returns true if both paths refer to the same file, ignoring case.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// isWithinDir returns true if path is located within dir, ignoring case.
func isWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(strings.ToLower(filepath.Clean(dir)), strings.ToLower(filepath.Clean(path)))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
