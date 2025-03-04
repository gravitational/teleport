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

package workloadattest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// binaryHashMaxBytes is the maximum number of bytes we'll read from a process'
// binary to calculate its SHA256 checksum.
const binaryHashMaxBytes = 1024 * 1024 * 100

// UnixAttestor attests a process id to a Unix process.
type UnixAttestor struct {
	log *slog.Logger
	os  UnixOS
}

// UnixOS is a handle on the operating system-specific features used by the Unix
// workload attestor.
type UnixOS interface {
	// ExePath returns the filesystem path of the given process' executable.
	ExePath(ctx context.Context, proc *process.Process) (string, error)

	// OpenExe opens the given process' executable for reading.
	//
	// Use this rather than `os.Open(ExePath(proc))` because operating systems
	// like Linux provide ways to read the original executable when the file on
	// disk is replaced or modified.
	OpenExe(ctx context.Context, proc *process.Process) (io.ReadCloser, error)
}

// NewUnixAttestor returns a new UnixAttestor.
func NewUnixAttestor(log *slog.Logger) *UnixAttestor {
	return &UnixAttestor{
		log: log,
		os:  unixOS,
	}
}

// Attest attests a process id to a Unix process.
func (a *UnixAttestor) Attest(ctx context.Context, pid int) (*workloadidentityv1pb.WorkloadAttrsUnix, error) {
	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return nil, trace.Wrap(err, "getting process")
	}

	att := &workloadidentityv1pb.WorkloadAttrsUnix{
		Attested: true,
		Pid:      int32(pid),
	}
	// On Linux:
	// Real, effective, saved, and file system GIDs
	// On Darwin:
	// Effective, effective, saved GIDs
	gids, err := p.Gids()
	if err != nil {
		return nil, trace.Wrap(err, "getting gids")
	}
	// We generally want to select the effective GID.
	switch len(gids) {
	case 0:
		// error as none returned
		return nil, trace.BadParameter("no gids returned")
	case 1:
		// Only one GID - this is unusual but let's take it.
		att.Gid = gids[0]
	default:
		// Take the index 1 entry as this is effective
		att.Gid = gids[1]
	}

	// On Linux:
	// Real, effective, saved set, and file system UIDs
	// On Darwin:
	// Effective
	uids, err := p.Uids()
	if err != nil {
		return nil, trace.Wrap(err, "getting uids")
	}
	// We generally want to select the effective GID.
	switch len(uids) {
	case 0:
		// error as none returned
		return nil, trace.BadParameter("no uids returned")
	case 1:
		// Only one UID, we expect this on Darwin to be the Effective UID
		att.Uid = uids[0]
	default:
		// Take the index 1 entry as this is Effective UID on Linux
		att.Uid = uids[1]
	}

	path, err := a.os.ExePath(ctx, p)
	switch {
	case trace.IsNotFound(err):
		// We could not find the executable because we're in a different mount namespace.
	case err != nil:
		a.log.ErrorContext(ctx, "Failed to find workload executable", "error", err)
	default:
		att.BinaryPath = &path
	}

	exe, err := a.os.OpenExe(ctx, p)
	if err != nil {
		a.log.ErrorContext(ctx, "Failed to open workload executable for hashing", "error", err)
		return att, nil
	}
	defer func() { _ = exe.Close() }()

	hash := sha256.New()
	if _, err := copyAtMost(hash, exe, binaryHashMaxBytes); err != nil {
		a.log.ErrorContext(ctx, "Failed to hash workload executable", "error", err)
		return att, nil
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	att.BinaryHash = &sum

	return att, nil
}

// copyAtMost copies at most n bytes from src to dst. If src contains more than
// n bytes, a LimitExceeded error will be returned.
func copyAtMost(dst io.Writer, src io.Reader, n int64) (int64, error) {
	copied, err := io.CopyN(dst, src, n)
	switch {
	case err == io.EOF:
		return copied, nil
	case err != nil:
		return 0, err
	}

	// Try to read one more byte to see if we reached the end of src.
	_, err = src.Read([]byte{0})
	switch {
	case err == io.EOF:
		return copied, nil
	case err != nil:
		return 0, err
	default:
		return 0, trace.LimitExceeded("input is larger than limit (%d)", n)
	}
}
