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

	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// UnixAttestor attests a process id to a Unix process.
type UnixAttestor struct {
}

// NewUnixAttestor returns a new UnixAttestor.
func NewUnixAttestor() *UnixAttestor {
	return &UnixAttestor{}
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

	return att, nil
}
