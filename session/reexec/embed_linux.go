// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reexec

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/session/reexec/reexecconstants"
)

// loadEmbeddedReexec loads the given gzipped session helper binary into a
// sealed, executable memfd and tests it to confirm that it can be executed,
// then returns the memfd.
func loadEmbeddedReexec(name string, dataGZ string) (*os.File, error) {
	mf, err := createExecutableMemfd(name, dataGZ)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(espadolini): set the selinux file label on mf, figure out if it
	// should be the same as the current executable or a fixed
	// teleport_ssh_exec_t

	// run a self test with a noop command, just to be sure
	c := &exec.Cmd{
		Path: mf.Name(),
		Args: []string{"teleport", reexecconstants.TrueSubCommand},
	}
	if err := c.Run(); err != nil {
		_ = mf.Close()
		return nil, trace.Wrap(trace.ConvertSystemError(err), "failed to launch embedded binary")
	}

	return mf, nil
}

// createExecutableMemfd returns a sealed memfd with the given name, containing
// the uncompressed gzipped binary.
func createExecutableMemfd(name string, dataGZ string) (_ *os.File, err error) {
	// we follow the logic from runc here
	// https://github.com/opencontainers/runc/blob/edbed618bff99edaaa9d622f9f8687004fe5e50a/libcontainer/system/linux.go#L89
	mfd, err := unix.MemfdCreate(name, unix.MFD_ALLOW_SEALING|unix.MFD_CLOEXEC|unix.MFD_EXEC)
	if errors.Is(err, unix.EINVAL) {
		mfd, err = unix.MemfdCreate(name, unix.MFD_ALLOW_SEALING|unix.MFD_CLOEXEC)
	}
	if err != nil {
		if errors.Is(err, unix.EACCES) {
			return nil, trace.AccessDenied("failed to create executable memfd for embedded binary (possibly due to vm.memfd_noexec=2)")

		}
		return nil, trace.Wrap(trace.ConvertSystemError(err), "failed to create executable memfd for embedded binary")
	}

	// This must not be "/proc/self/fd/<n>" because in the post-fork, pre-exec
	// environment of the child the file descriptors inherited from the parent
	// are not guaranteed to be available - the fd will likely end up being
	// small if this runs early, so it will likely get overridden as part of the
	// exec machinery with one of the files in [exec.Cmd.ExtraFiles]. This does
	// however mean that an unprivileged child will not be able to launch the
	// helper, so this is only usable when launching it as the same user that
	// the Teleport process is running as. [setLinuxReexecPath] will ensure that
	// that's the case before attempting to use the memfd, and it will fall back
	// to a regular reexecution of "/proc/self/exe" if need be.
	mf := os.NewFile(uintptr(mfd), fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), mfd))
	defer func() {
		if err != nil {
			_ = mf.Close()
		}
	}()

	gzreader, err := gzip.NewReader(strings.NewReader(dataGZ))
	if err != nil {
		return nil, trace.Wrap(err, "failed to decompress embedded binary (this is a bug)")
	}
	if _, err := gzreader.WriteTo(mf); err != nil {
		return nil, trace.Wrap(err, "failed to write embedded binary in memfd")
	}
	if err := mf.Chmod(0o555); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "failed to chmod memfd for embedded binary")
	}

	// ignore errors here since F_SEAL_EXEC is newer than the rest of the memfd interface
	_, _ = unix.FcntlInt(uintptr(mfd), unix.F_ADD_SEALS, unix.F_SEAL_EXEC)

	if _, err := unix.FcntlInt(uintptr(mfd), unix.F_ADD_SEALS, unix.F_SEAL_SEAL|unix.F_SEAL_SHRINK|unix.F_SEAL_GROW|unix.F_SEAL_WRITE); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "failed to seal memfd for embedded binary")
	}

	return mf, nil
}
