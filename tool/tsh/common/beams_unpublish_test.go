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
	"github.com/gravitational/teleport/lib/client"
)

func TestBeamsUnpublishCommand(t *testing.T) {
	tests := []struct {
		name       string
		wantName   string
		beam       *beamsv1.Beam
		wantOutput string
		wantErr    string
	}{
		{
			name:     "success",
			wantName: "beam-ref",
			beam: makeTestBeam(
				"alpha",
				"11111111-1111-1111-1111-111111111111",
				"alice",
				"alpha-app",
				time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
				&beamsv1.PublishSpec{Protocol: beamsv1.Protocol_PROTOCOL_HTTP, Port: 8080},
			),
			wantOutput: "Beam \"alpha\" successfully unpublished.\n",
		},
		{
			name:     "not published",
			wantName: "beam-ref",
			beam: makeTestBeam(
				"alpha",
				"11111111-1111-1111-1111-111111111111",
				"alice",
				"alpha-app",
				time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
				nil,
			),
			wantErr: "Beam \"alpha\" is not published.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotName string
			var updatedBeam *beamsv1.Beam
			var output bytes.Buffer

			cf := &CLIConf{
				Context:        context.Background(),
				Proxy:          "proxy.example.com:443",
				Username:       "alice",
				OverrideStdout: &output,
				HomePath:       t.TempDir(),
			}
			mustCreateEmptyProfile(t, cf)

			cmd := beamsUnpublishCommand{
				name: tt.wantName,
				getFn: func(_ context.Context, _ *client.TeleportClient, name string) (*beamsv1.Beam, error) {
					gotName = name
					return tt.beam, nil
				},
				updateFn: func(_ context.Context, _ *client.TeleportClient, beam *beamsv1.Beam) (*beamsv1.Beam, error) {
					updatedBeam = beam
					return beam, nil
				},
			}

			err := cmd.run(cf)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				require.Equal(t, tt.wantName, gotName)
				require.Nil(t, updatedBeam)
				require.Empty(t, output.String())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantName, gotName)
			require.NotNil(t, updatedBeam)
			require.Nil(t, updatedBeam.GetSpec().GetPublish())
			require.Equal(t, tt.wantOutput, output.String())
		})
	}
}
