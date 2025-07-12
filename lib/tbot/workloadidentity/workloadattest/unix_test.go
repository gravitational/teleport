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
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestUnixAttestor_Attest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pid := os.Getpid()
	uid := os.Getuid()
	gid := os.Getgid()

	attestor := NewUnixAttestor(
		UnixAttestorConfig{BinaryHashMaxSizeBytes: -1},
		logtest.NewLogger(),
	)
	attestor.os = testOS{
		exePath: func(context.Context, *process.Process) (string, error) {
			return "/path/to/executable", nil
		},
		openExe: func(context.Context, *process.Process) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`hello world`)), nil
		},
	}

	att, err := attestor.Attest(ctx, pid)
	require.NoError(t, err)
	require.Empty(t,
		cmp.Diff(
			&workloadidentityv1pb.WorkloadAttrsUnix{
				Attested:   true,
				Pid:        int32(pid),
				Uid:        uint32(uid),
				Gid:        uint32(gid),
				BinaryPath: proto.String("/path/to/executable"),
				BinaryHash: proto.String("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"),
			},
			att,
			protocmp.Transform(),
		),
	)
}

func TestUnixAttestor_BinaryTooLarge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	attestor := NewUnixAttestor(
		UnixAttestorConfig{BinaryHashMaxSizeBytes: 1024},
		logtest.NewLogger(),
	)
	attestor.os = testOS{
		exePath: func(context.Context, *process.Process) (string, error) {
			return "/path/to/executable", nil
		},
		openExe: func(context.Context, *process.Process) (io.ReadCloser, error) {
			var exe [2048]byte
			return io.NopCloser(bytes.NewReader(exe[:])), nil
		},
	}

	att, err := attestor.Attest(ctx, os.Getpid())
	require.NoError(t, err)
	require.Nil(t, att.BinaryHash)
}

type testOS struct {
	exePath func(context.Context, *process.Process) (string, error)
	openExe func(context.Context, *process.Process) (io.ReadCloser, error)
}

func (t testOS) ExePath(ctx context.Context, proc *process.Process) (string, error) {
	return t.exePath(ctx, proc)
}

func (t testOS) OpenExe(ctx context.Context, proc *process.Process) (io.ReadCloser, error) {
	return t.openExe(ctx, proc)
}

func Test_copyAtMost(t *testing.T) {
	t.Run("n > len(src)", func(t *testing.T) {
		var dst bytes.Buffer
		src := bytes.NewReader([]byte{1, 2, 3})

		copied, err := copyAtMost(&dst, src, 5)
		require.NoError(t, err)

		assert.Equal(t, int64(3), copied)
		assert.Equal(t, []byte{1, 2, 3}, dst.Bytes())
	})

	t.Run("n == len(src)", func(t *testing.T) {
		var dst bytes.Buffer
		src := bytes.NewReader([]byte{1, 2, 3})

		copied, err := copyAtMost(&dst, src, 3)
		require.NoError(t, err)

		assert.Equal(t, int64(3), copied)
		assert.Equal(t, []byte{1, 2, 3}, dst.Bytes())
	})

	t.Run("n < len(src)", func(t *testing.T) {
		var dst bytes.Buffer
		src := bytes.NewReader([]byte{1, 2, 3})

		_, err := copyAtMost(&dst, src, 1)
		require.Error(t, err)
		assert.True(t, trace.IsLimitExceeded(err))
	})
}
