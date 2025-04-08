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
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

func TestUnixAttestor_Attest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pid := os.Getpid()
	uid := os.Getuid()
	gid := os.Getgid()

	attestor := NewUnixAttestor()
	att, err := attestor.Attest(ctx, pid)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(&workloadidentityv1pb.WorkloadAttrsUnix{
		Attested: true,
		Pid:      int32(pid),
		Uid:      uint32(uid),
		Gid:      uint32(gid),
	}, att, protocmp.Transform()))
}
