/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package diag

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientAddressParsing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		addr string
		url  *url.URL
	}{
		{
			addr: "http://127.0.0.1:3000",
			url: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:3000",
			},
		},
		{
			addr: "http://localhost:3000",
			url: &url.URL{
				Scheme: "http",
				Host:   "localhost:3000",
			},
		},
		{
			addr: "localhost:3000",
			url: &url.URL{
				Scheme: "http",
				Host:   "localhost:3000",
			},
		},
		{
			addr: "127.0.0.1:3000",
			url: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:3000",
			},
		},
		{
			addr: "badurl:300:9:1",
		},
		{
			addr: "http//badurl",
		},
		{
			addr: "/var/lib/file.sock",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.addr, func(t *testing.T) {
			u, err := parseAddress(tc.addr)
			if tc.url == nil {
				require.Error(t, err)
				require.Nil(t, u)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.url.Host, u.Host)
				require.Equal(t, tc.url.Scheme, u.Scheme)
			}
		})
	}
}
