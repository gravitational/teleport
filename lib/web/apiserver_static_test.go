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

package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsWebUIRoute(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		path string
		want bool
	}{
		{
			name: "web root",
			path: "/web",
			want: true,
		},
		{
			name: "web subroute",
			path: "/web/cluster/root/resources",
			want: true,
		},
		{
			name: "delegation authorize",
			path: "/delegation/authorize",
			want: true,
		},
		{
			name: "delegation authorize subroute",
			path: "/delegation/authorize/extra",
			want: false,
		},
		{
			name: "api route",
			path: "/webapi/ping",
			want: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isWebUIRoute(tt.path))
		})
	}
}
