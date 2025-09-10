// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectProfiles(t *testing.T) {
	ctx := context.Background()

	for _, test := range []struct {
		desc             string
		profilesInput    string
		seconds          int
		expectedProfiles map[string]int
	}{
		{
			desc:             "single profile",
			profilesInput:    "goroutine",
			expectedProfiles: map[string]int{"goroutine": 0},
		},
		{
			desc:             "profile with seconds flag",
			profilesInput:    "block",
			seconds:          15,
			expectedProfiles: map[string]int{"block": 15},
		},
		{
			desc:             "multiple profiles",
			profilesInput:    "allocs,goroutine",
			expectedProfiles: map[string]int{"allocs": 0, "goroutine": 0},
		},
		{
			desc:             "profiles without snapshot support",
			profilesInput:    "trace,profile",
			expectedProfiles: map[string]int{"trace": defaultCollectProfileSeconds, "profile": defaultCollectProfileSeconds},
		},
		{
			desc:             "profiles without snapshot support with seconds provided",
			profilesInput:    "trace,profile",
			seconds:          20,
			expectedProfiles: map[string]int{"trace": 20, "profile": 20},
		},
		{
			desc:             "duplicated profiles",
			profilesInput:    "goroutine,goroutine",
			expectedProfiles: map[string]int{"goroutine": 0},
		},
		{
			desc:          "all valid profiles",
			profilesInput: "allocs,block,cmdline,goroutine,heap,mutex,profile,threadcreate,trace",
			expectedProfiles: map[string]int{
				"allocs":       0,
				"block":        0,
				"cmdline":      0,
				"goroutine":    0,
				"heap":         0,
				"mutex":        0,
				"profile":      defaultCollectProfileSeconds,
				"threadcreate": 0,
				"trace":        defaultCollectProfileSeconds,
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			clt := &mockDebugClient{profileContents: make([]byte, 0)}
			var out bytes.Buffer
			err := collectProfiles(ctx, clt, &out, test.profilesInput, test.seconds)
			require.NoError(t, err)

			var requestedProfiles []string
			for _, profile := range clt.collectedProfiles {
				expectedSeconds, ok := test.expectedProfiles[profile.name]
				require.True(t, ok, "unexpected profile %q collected", profile.name)
				require.Equal(t, expectedSeconds, profile.seconds)
				requestedProfiles = append(requestedProfiles, profile.name)
			}
			require.Len(t, test.expectedProfiles, len(requestedProfiles), "expected %d to be requested but got %d", len(test.expectedProfiles), len(requestedProfiles))

			reader, err := gzip.NewReader(&out)
			require.NoError(t, err)
			var files []string
			tarReader := tar.NewReader(reader)
			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				files = append(files, strings.TrimSuffix(header.Name, ".pprof"))
			}

			// We should have one file per profile collected.
			require.ElementsMatch(t, slices.Collect(maps.Keys(test.expectedProfiles)), files)
		})
	}
}

type collectedProfile struct {
	name    string
	seconds int
}

type mockDebugClient struct {
	DebugClient

	profileContents   []byte
	collectProfileErr error
	collectedProfiles []collectedProfile
}

// CollectProfile implements debug.Client.
func (m *mockDebugClient) CollectProfile(_ context.Context, name string, seconds int) ([]byte, error) {
	if m.collectedProfiles == nil {
		m.collectedProfiles = make([]collectedProfile, 0)
	}

	m.collectedProfiles = append(m.collectedProfiles, collectedProfile{name, seconds})
	return m.profileContents, m.collectProfileErr
}
