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

	"github.com/stretchr/testify/require"
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
	require.Equal(t, UnixAttestation{
		Attested: true,
		PID:      pid,
		UID:      uid,
		GID:      gid,
	}, att)
}
