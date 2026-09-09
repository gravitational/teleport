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
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"pgregory.net/rapid"

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
			workloadidentityv1pb.WorkloadAttrsUnix_builder{
				Attested:   true,
				Pid:        int32(pid),
				Uid:        uint32(uid),
				Gid:        uint32(gid),
				BinaryPath: proto.String("/path/to/executable"),
				BinaryHash: proto.String("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"),
			}.Build(),
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
	require.Nil(t, proto.ValueOrNil(att.HasBinaryHash(), att.GetBinaryHash))
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
	// bytes.Reader never exhibits some behaviors io.Reader permits, such as
	// returning data and io.EOF from the same call, so vary the reader too.
	readers := map[string]func(io.Reader) io.Reader{
		"bytes.Reader":  func(r io.Reader) io.Reader { return r },
		"OneByteReader": iotest.OneByteReader,
		"HalfReader":    iotest.HalfReader,
		"DataErrReader": iotest.DataErrReader,
	}
	readerNames := slices.Sorted(maps.Keys(readers))

	rapid.Check(t, func(t *rapid.T) {
		src := rapid.SliceOf(rapid.Byte()).Draw(t, "src")
		n := rapid.Int64Range(-1, int64(len(src))+8).Draw(t, "n")
		reader := readers[rapid.SampledFrom(readerNames).Draw(t, "reader")]

		var dst bytes.Buffer
		copied, err := copyAtMost(&dst, reader(bytes.NewReader(src)), n)

		// As per Go convention, we expect copied to always equal the amount
		// of bytes written to dst, regardless of whether an error was returned.
		assert.EqualValues(t, dst.Len(), copied)

		if n == -1 || n >= int64(len(src)) {
			// If in unlimited mode, or n is greater or equal to len of src, we
			// expect all to be copied and no error.
			assert.NoError(t, err)
			assert.True(t, bytes.Equal(src, dst.Bytes()))
		} else {
			// If n is less than len of src, we expect an error, and n bytes to
			// be copied.
			assert.True(t, trace.IsLimitExceeded(err))
			assert.EqualValues(t, n, copied)
			assert.True(t, bytes.Equal(src[:n], dst.Bytes()))
		}
	})
}
