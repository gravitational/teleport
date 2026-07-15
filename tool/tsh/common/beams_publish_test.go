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

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestBeamsPublishCommand(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		tcp          bool
		wantName     string
		wantProtocol beamsv1.Protocol
		beamAlias    string
		beamName     string
		appName      string
		expires      time.Time
	}{
		{
			name:         "text format http",
			wantName:     "beam-ref",
			wantProtocol: beamsv1.Protocol_PROTOCOL_HTTP,
			beamAlias:    "alpha",
			beamName:     "11111111-1111-1111-1111-111111111111",
			appName:      "alpha-app",
			expires:      time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			name:         "text format tcp",
			tcp:          true,
			wantName:     "beam-ref",
			wantProtocol: beamsv1.Protocol_PROTOCOL_TCP,
			beamAlias:    "charlie",
			beamName:     "33333333-3333-3333-3333-333333333333",
			appName:      "charlie-app",
			expires:      time.Date(2026, time.January, 2, 5, 6, 7, 0, time.UTC),
		},
		{
			name:         "JSON format",
			format:       teleport.JSON,
			wantName:     "beam-ref",
			wantProtocol: beamsv1.Protocol_PROTOCOL_HTTP,
			beamAlias:    "alpha",
			beamName:     "11111111-1111-1111-1111-111111111111",
			appName:      "alpha-app",
			expires:      time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			name:         "YAML format",
			format:       teleport.YAML,
			tcp:          true,
			wantName:     "beam-ref",
			wantProtocol: beamsv1.Protocol_PROTOCOL_TCP,
			beamAlias:    "charlie",
			beamName:     "33333333-3333-3333-3333-333333333333",
			appName:      "charlie-app",
			expires:      time.Date(2026, time.January, 2, 5, 6, 7, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotName string
			var updatedBeam *beamsv1.Beam
			var output bytes.Buffer

			beam := makeTestBeam(
				tt.beamAlias,
				tt.beamName,
				"alice",
				tt.appName,
				tt.expires,
				nil,
			)

			cf := &CLIConf{
				Context:        context.Background(),
				Proxy:          "proxy.example.com:443",
				Username:       "alice",
				OverrideStdout: &output,
				overrideStderr: &output,
				HomePath:       t.TempDir(),
			}
			mustCreateEmptyProfile(t, cf)

			cmd := beamsPublishCommand{
				name:   tt.wantName,
				tcp:    tt.tcp,
				format: tt.format,
				getFn: func(_ context.Context, _ *client.TeleportClient, name string) (*beamsv1.Beam, error) {
					gotName = name
					return beam, nil
				},
				updateFn: func(_ context.Context, _ *client.TeleportClient, beam *beamsv1.Beam) (*beamsv1.Beam, error) {
					updatedBeam = beam
					return beam, nil
				},
				proxyAddrFn: func(*CLIConf) (string, error) {
					return "proxy.example.com:443", nil
				},
			}

			err := cmd.run(cf)
			require.NoError(t, err)
			require.Equal(t, tt.wantName, gotName)
			require.NotNil(t, updatedBeam)
			require.NotNil(t, updatedBeam.GetSpec().GetPublish())
			require.Equal(t, uint32(8080), updatedBeam.GetSpec().GetPublish().GetPort())
			require.Equal(t, tt.wantProtocol, updatedBeam.GetSpec().GetPublish().GetProtocol())

			if golden.ShouldSet() {
				golden.Set(t, output.Bytes())
			}

			require.Equal(t, string(golden.Get(t)), output.String())
		})
	}
}
