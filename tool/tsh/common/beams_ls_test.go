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

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestBeamsLSCommand(t *testing.T) {
	now := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)

	beams := []*beamsv1.Beam{
		makeTestBeam(
			"alpha",
			"11111111-1111-1111-1111-111111111111",
			"alice",
			"alpha-app",
			time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
			&beamsv1.PublishSpec{Protocol: beamsv1.Protocol_PROTOCOL_HTTP, Port: 8080},
		),
		makeTestBeam(
			"bravo",
			"22222222-2222-2222-2222-222222222222",
			"alice",
			"",
			time.Date(2026, time.January, 2, 4, 5, 6, 0, time.UTC),
			nil,
		),
		makeTestBeam(
			"charlie",
			"33333333-3333-3333-3333-333333333333",
			"bob",
			"charlie-app",
			time.Date(2026, time.January, 2, 5, 6, 7, 0, time.UTC),
			&beamsv1.PublishSpec{Protocol: beamsv1.Protocol_PROTOCOL_TCP, Port: 5432},
		),
	}

	tests := []struct {
		name    string
		format  string
		all     bool
		wantAll bool
	}{
		{
			name:    "text format",
			wantAll: false,
		},
		{
			name:    "text format with all",
			all:     true,
			format:  teleport.Text,
			wantAll: true,
		},
		{
			name:    "JSON format",
			format:  teleport.JSON,
			wantAll: false,
		},
		{
			name:    "YAML format",
			format:  teleport.YAML,
			all:     true,
			wantAll: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAll bool
			var buf bytes.Buffer

			cf := &CLIConf{
				Context:        context.Background(),
				Proxy:          "proxy.example.com:443",
				Username:       "alice",
				OverrideStdout: &buf,
				HomePath:       t.TempDir(),
			}
			mustCreateEmptyProfile(t, cf)

			cmd := beamsLSCommand{
				all:    tt.all,
				format: tt.format,
				fetchFn: func(_ context.Context, _ *client.TeleportClient, all bool) ([]*beamsv1.Beam, error) {
					gotAll = all
					return beams, nil
				},
				proxyAddrFn: func(*CLIConf) (string, error) {
					return "proxy.example.com:443", nil
				},
				humanizeTimeFn: func(tm time.Time) string {
					return humanize.RelTime(tm, now, "ago", "from now")
				},
			}

			err := cmd.run(cf)
			require.NoError(t, err)
			require.Equal(t, tt.wantAll, gotAll)

			if golden.ShouldSet() {
				golden.Set(t, buf.Bytes())
			}

			require.Equal(t, string(golden.Get(t)), buf.String())
		})
	}
}

func makeTestBeam(alias, name, owner, appName string, expires time.Time, publish *beamsv1.PublishSpec) *beamsv1.Beam {
	return &beamsv1.Beam{
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &beamsv1.BeamSpec{
			Expires: timestamppb.New(expires),
			Publish: publish,
		},
		Status: &beamsv1.BeamStatus{
			Alias:   alias,
			User:    owner,
			AppName: appName,
		},
	}
}
