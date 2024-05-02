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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

func TestCollectProfiles(t *testing.T) {
	ctx := context.Background()

	for _, test := range []struct {
		desc             string
		profilesInput    string
		seconds          int
		expectErr        bool
		expectedProfiles []string
	}{
		{
			desc:             "default profiles",
			profilesInput:    "",
			expectedProfiles: defaultCollectProfiles,
		},
		{
			desc:             "single profile",
			profilesInput:    "goroutine",
			expectedProfiles: []string{"goroutine"},
		},
		{
			desc:             "profile with seconds flag",
			profilesInput:    "block",
			seconds:          10,
			expectedProfiles: []string{"block"},
		},
		{
			desc:             "multiple profiles",
			profilesInput:    "allocs,goroutine",
			expectedProfiles: []string{"allocs", "goroutine"},
		},
		{
			desc:             "all valid profiles",
			profilesInput:    "allocs,block,cmdline,goroutine,heap,mutex,profile,threadcreate,trace",
			expectedProfiles: []string{"allocs", "block", "cmdline", "goroutine", "heap", "mutex", "profile", "threadcreate", "trace"},
		},
		{
			desc:          "invalid profile",
			profilesInput: "random",
			expectErr:     true,
		},
		{
			desc:          "invalid profile on the list",
			profilesInput: "goroutine,random",
			expectErr:     true,
		},
		{
			desc:          "invalid profiles separator",
			profilesInput: "goroutine random",
			expectErr:     true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			clt := &mockDebugClient{profileContents: make([]byte, 0)}
			var out bytes.Buffer
			err := collectProfiles(ctx, clt, &out, test.profilesInput, test.seconds, func(int) {})
			if test.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Len(t, clt.collectedProfiles, len(test.expectedProfiles))
			var requestedProfiles []string
			for _, profile := range clt.collectedProfiles {
				require.Equal(t, test.seconds, profile.seconds)
				requestedProfiles = append(requestedProfiles, profile.name)
			}
			require.ElementsMatch(t, test.expectedProfiles, requestedProfiles)

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
			require.ElementsMatch(t, test.expectedProfiles, files)
		})
	}
}

type collectedProfile struct {
	name    string
	seconds int
}

type mockDebugClient struct {
	debugclient.Client

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
