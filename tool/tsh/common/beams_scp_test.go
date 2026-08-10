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
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
)

func TestBeamsSCPCommandRun(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		dest               string
		recursive          bool
		quiet              bool
		nodeIDByBeamName   map[string]string
		wantSources        []string
		wantDestination    string
		wantProgressWriter bool
		wantOutput         string
		wantErr            string
	}{
		{
			name:      "remote source to local destination",
			src:       "alpha:/var/log/app.log",
			dest:      "/tmp/app.log",
			recursive: true,
			nodeIDByBeamName: map[string]string{
				"alpha": "node-123",
			},
			wantSources:        []string{"beams@node-123:/var/log/app.log"},
			wantDestination:    "/tmp/app.log",
			wantProgressWriter: true,
			wantOutput:         "Copied successfully.\n",
		},
		{
			name:  "local source to remote destination in quiet mode",
			src:   "/tmp/app.log",
			dest:  "alpha:/var/log/app.log",
			quiet: true,
			nodeIDByBeamName: map[string]string{
				"alpha": "node-123",
			},
			wantSources:        []string{"/tmp/app.log"},
			wantDestination:    "beams@node-123:/var/log/app.log",
			wantProgressWriter: false,
			wantOutput:         "Copied successfully.\n",
		},
		{
			name:    "both local paths error",
			src:     "/tmp/src.txt",
			dest:    "/tmp/dest.txt",
			wantErr: "at least one of <src> <dest> must be in the form `BEAM_ID:PATH`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			var gotReq client.SFTPRequest
			var sftpCalled bool

			cf := &CLIConf{
				Context:        context.Background(),
				Proxy:          "proxy.example.com:443",
				Username:       "alice",
				OverrideStdout: &output,
				HomePath:       t.TempDir(),
			}
			mustCreateEmptyProfile(t, cf)

			cmd := beamsSCPCommand{
				src:       tt.src,
				dest:      tt.dest,
				recursive: tt.recursive,
				quiet:     tt.quiet,
				getBeamFn: func(_ context.Context, _ authclient.ClientI, name string) (*beamsv1.Beam, error) {
					nodeID, ok := tt.nodeIDByBeamName[name]
					if !ok {
						t.Fatalf("unexpected getBeam call for %q", name)
					}
					beam := makeTestBeam(
						name,
						"11111111-1111-1111-1111-111111111111",
						"alice",
						name+"-app",
						time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
						nil,
					)
					beam.Status.NodeId = nodeID
					return beam, nil
				},
				withClusterFn: func(_ context.Context, _ *client.TeleportClient, fn func(authclient.ClientI) error) error {
					return fn(nil)
				},
				sftpFn: func(_ context.Context, _ *client.TeleportClient, req client.SFTPRequest) error {
					sftpCalled = true
					gotReq = req
					return nil
				},
			}

			err := cmd.run(cf)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				require.False(t, sftpCalled)
				return
			}

			require.NoError(t, err)
			require.True(t, sftpCalled)
			require.Equal(t, tt.wantSources, gotReq.Sources)
			require.Equal(t, tt.wantDestination, gotReq.Destination)
			require.Equal(t, tt.recursive, gotReq.Recursive)
			if tt.wantProgressWriter {
				require.Same(t, cf.Stdout(), gotReq.ProgressWriter)
			} else {
				require.Nil(t, gotReq.ProgressWriter)
			}
			require.Equal(t, tt.wantOutput, output.String())
		})
	}
}
