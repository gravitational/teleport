/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMatchAppServerForRoute(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc string

		appName string
		appAddr string

		name string
		addr string

		wantMatch bool
	}{
		{
			desc:      "all match",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "foo",
			addr:      "foo.example.com",
			wantMatch: true,
		},
		{
			desc:      "fallback no name (match)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "",
			addr:      "foo.example.com",
			wantMatch: true,
		},
		{
			desc:      "fallback no name (mismatch)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "",
			addr:      "bar.example.com",
			wantMatch: false,
		},
		{
			desc:      "different name",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "bar",
			addr:      "foo.example.com",
			wantMatch: false,
		},
		{
			desc:      "different addr",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "foo",
			addr:      "bar.example.com",
			wantMatch: false,
		},
		{
			desc:      "name only (match)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "foo",
			addr:      "",
			wantMatch: true,
		},
		{
			desc:      "name only (mismatch)",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "bar",
			addr:      "",
			wantMatch: false,
		},
		{
			desc:      "neither name nor addr matches nothing",
			appName:   "foo",
			appAddr:   "foo.example.com",
			name:      "",
			addr:      "",
			wantMatch: false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			appServer, err := types.NewAppServerV3(
				types.Metadata{Name: test.appName},
				types.AppServerSpecV3{
					HostID: "test-host-id",
					App: &types.AppV3{
						Metadata: types.Metadata{Name: test.appName},
						Spec: types.AppSpecV3{
							PublicAddr: test.appAddr,
							URI:        "http://localhost:12345",
						},
					},
				},
			)
			require.NoError(t, err)

			require.Equal(
				t,
				test.wantMatch,
				MatchAppServerForRoute(test.name, test.addr)(appServer),
			)
		})
	}
}
