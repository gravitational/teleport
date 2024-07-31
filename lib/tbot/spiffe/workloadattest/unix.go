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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"
)

// UnixAttestation holds the Unix process information retrieved from the
// workload attestation process.
type UnixAttestation struct {
	// Attested is true if the PID was successfully attested to a Unix
	// process. This indicates the validity of the rest of the fields.
	Attested bool
	// PID is the process ID of the attested process.
	PID int
	// UID is the primary user ID of the attested process.
	UID int
	// GID is the primary group ID of the attested process.
	GID int
}

// LogValue implements slog.LogValue to provide a nicely formatted set of
// log keys for a given attestation.
func (a UnixAttestation) LogValue() slog.Value {
	values := []slog.Attr{
		slog.Bool("attested", a.Attested),
	}
	if a.Attested {
		values = append(values,
			slog.Int("uid", a.UID),
			slog.Int("pid", a.PID),
			slog.Int("gid", a.GID),
		)
	}
	return slog.GroupValue(values...)
}

// UnixAttestor attests a process id to a Unix process.
type UnixAttestor struct {
}

// NewUnixAttestor returns a new UnixAttestor.
func NewUnixAttestor() *UnixAttestor {
	return &UnixAttestor{}
}

// Attest attests a process id to a Unix process.
func (a *UnixAttestor) Attest(ctx context.Context, pid int) (UnixAttestation, error) {
	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return UnixAttestation{}, trace.Wrap(err, "getting process")
	}

	att := UnixAttestation{
		Attested: true,
		PID:      pid,
	}
	// On Linux:
	// Real, effective, saved, and file system GIDs
	// On Darwin:
	// Effective, effective, saved GIDs
	gids, err := p.Gids()
	if err != nil {
		return UnixAttestation{}, trace.Wrap(err, "getting gids")
	}
	// We generally want to select the effective GID.
	switch len(gids) {
	case 0:
		// error as none returned
		return UnixAttestation{}, trace.BadParameter("no gids returned")
	case 1:
		// Only one GID - this is unusual but let's take it.
		att.GID = int(gids[0])
	default:
		// Take the index 1 entry as this is effective
		att.GID = int(gids[1])
	}

	// On Linux:
	// Real, effective, saved set, and file system UIDs
	// On Darwin:
	// Effective
	uids, err := p.Uids()
	if err != nil {
		return UnixAttestation{}, trace.Wrap(err, "getting uids")
	}
	// We generally want to select the effective GID.
	switch len(uids) {
	case 0:
		// error as none returned
		return UnixAttestation{}, trace.BadParameter("no uids returned")
	case 1:
		// Only one UID, we expect this on Darwin to be the Effective UID
		att.UID = int(uids[0])
	default:
		// Take the index 1 entry as this is Effective UID on Linux
		att.UID = int(uids[1])
	}

	return att, nil
}
