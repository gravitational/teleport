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

package common

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestParseBeamCopySpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    []string
		want    beamCopySpec
		wantErr string
	}{
		{
			name: "push to beam",
			spec: []string{"/tmp/local.txt", "beam-123:/remote.txt"},
			want: beamCopySpec{
				Source: beamCopyTarget{
					Path: "/tmp/local.txt",
				},
				Destination: beamCopyTarget{
					Path:   "/remote.txt",
					BeamID: "beam-123",
					IsBeam: true,
				},
			},
		},
		{
			name: "pull from beam",
			spec: []string{"beam-123:/remote.txt", "/tmp/local.txt"},
			want: beamCopySpec{
				Source: beamCopyTarget{
					Path:   "/remote.txt",
					BeamID: "beam-123",
					IsBeam: true,
				},
				Destination: beamCopyTarget{
					Path: "/tmp/local.txt",
				},
			},
		},
		{
			name: "beam current directory",
			spec: []string{"beam-123:", "/tmp/local-dir"},
			want: beamCopySpec{
				Source: beamCopyTarget{
					BeamID: "beam-123",
					IsBeam: true,
				},
				Destination: beamCopyTarget{
					Path: "/tmp/local-dir",
				},
			},
		},
		{
			name: "local path with explicit prefix stays local",
			spec: []string{"./notes:2026.txt", "beam-123:/remote.txt"},
			want: beamCopySpec{
				Source: beamCopyTarget{
					Path: "./notes:2026.txt",
				},
				Destination: beamCopyTarget{
					Path:   "/remote.txt",
					BeamID: "beam-123",
					IsBeam: true,
				},
			},
		},
		{
			name: "copy between beams",
			spec: []string{"beam-123:/src.txt", "beam-456:/dst.txt"},
			want: beamCopySpec{
				Source: beamCopyTarget{
					Path:   "/src.txt",
					BeamID: "beam-123",
					IsBeam: true,
				},
				Destination: beamCopyTarget{
					Path:   "/dst.txt",
					BeamID: "beam-456",
					IsBeam: true,
				},
			},
		},
		{
			name:    "source and destination required",
			spec:    []string{"beam-123:/remote.txt"},
			wantErr: "source and destination are required",
		},
		{
			name:    "requires beam endpoint",
			spec:    []string{"/tmp/source.txt", "/tmp/dest.txt"},
			wantErr: "one of source or destination must be a beam path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseBeamCopySpec(tt.spec)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBeamsCPCLIParse(t *testing.T) {
	t.Parallel()

	stopErr := trace.BadParameter("stop after parse")
	err := Run(
		context.Background(),
		[]string{"beams", "cp", "--recursive", "/tmp/local.txt", "beam-123:/remote.txt"},
		setHomePath(t.TempDir()),
		func(cf *CLIConf) error {
			require.Equal(t, []string{"/tmp/local.txt", "beam-123:/remote.txt"}, cf.BeamCopySpec)
			require.True(t, cf.RecursiveCopy)
			return stopErr
		},
	)
	require.ErrorIs(t, err, stopErr)
}

// TODO(greedy52) DELETE ME
func TestBeamSpinner(t *testing.T) {
	stopCreating := startBeamSpinner(os.Stdout, "creating...")
	time.Sleep(2 * time.Second)
	stopCreating("◆ created beams-abc-123-fake-id")

	stopConnecting := startBeamSpinner(os.Stdout, "connecting...")
	time.Sleep(2 * time.Second)
	stopConnecting("↳ ready")
}
