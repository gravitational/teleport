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

package connection

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseRedisURI(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		want   *Options
		errStr string
	}{
		{
			name: "correct URI",
			uri:  "redis://localhost:6379",
			want: &Options{
				Mode:    Standalone,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name: "correct host:port",
			uri:  "localhost:6379",
			want: &Options{
				Mode:    Standalone,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name: "rediss schema is accepted",
			uri:  "rediss://localhost:6379",
			want: &Options{
				Mode:    Standalone,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name: "IP address passes",
			uri:  "rediss://1.2.3.4:6379",
			want: &Options{
				Mode:    Standalone,
				Address: "1.2.3.4",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name: "single instance explicit",
			uri:  "redis://localhost:6379?mode=standalone",
			want: &Options{
				Mode:    Standalone,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name: "cluster enabled",
			uri:  "redis://localhost:6379?mode=cluster",
			want: &Options{
				Mode:    Cluster,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name:   "invalid connection mode",
			uri:    "redis://localhost:6379?mode=foo",
			want:   nil,
			errStr: "incorrect connection mode",
		},
		{
			name:   "invalid connection string",
			uri:    "localhost:6379?mode=foo",
			want:   nil,
			errStr: "failed to parse Redis URL",
		},
		{
			name: "only address default port",
			uri:  "localhost",
			want: &Options{
				Mode:    Standalone,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name: "default port",
			uri:  "redis://localhost",
			want: &Options{
				Mode:    Standalone,
				Address: "localhost",
				Port:    "6379",
			},
			errStr: "",
		},
		{
			name:   "incorrect URI schema is rejected",
			uri:    "http://localhost",
			want:   nil,
			errStr: "invalid Redis URI scheme",
		},
		{
			name:   "empty address",
			uri:    "",
			want:   nil,
			errStr: "address is empty",
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseRedisAddress(tt.uri)
			if err != nil {
				if tt.errStr == "" {
					require.FailNow(t, "unexpected error: %v", err)
					return
				}
				require.Contains(t, err.Error(), tt.errStr)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
