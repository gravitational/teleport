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

package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEndpointsResolver(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tests := []struct {
		desc          string
		uri           string
		wantEndpoints []string
		wantErr       string
	}{
		{
			// we don't allow invalid URIs anyway
			desc:    "URI parse fail",
			uri:     "host1,host2:123,host3",
			wantErr: "failed to parse",
		},
		{
			desc:          "single endpoint",
			uri:           "example.com:5432",
			wantEndpoints: []string{"example.com:5432"},
		},
		{
			desc:          "single endpoint custom port",
			uri:           "example.com:123",
			wantEndpoints: []string{"example.com:123"},
		},
		{
			desc:          "multiple endpoints",
			uri:           "host1,host2/somedb?target_session_attrs=any&application_name=myapp",
			wantEndpoints: []string{"host1:5432", "host2:5432"},
		},
		{
			desc:          "multiple endpoints custom ports",
			uri:           "host1,host2:456/somedb?target_session_attrs=any&application_name=myapp",
			wantEndpoints: []string{"host1:456", "host2:456"},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			resolver, err := newEndpointsResolver(test.uri)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			got, err := resolver.Resolve(ctx)
			require.NoError(t, err)
			require.EqualValues(t, test.wantEndpoints, got)
		})
	}
}
