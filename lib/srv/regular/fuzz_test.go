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

package regular

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv"
)

func FuzzParseProxySubsys(f *testing.F) {
	f.Add("")
	f.Add("proxy:")
	f.Add("proxy:@")
	f.Add("proxy:foo@bar")
	f.Add("proxy:host:22")
	f.Add("proxy:@clustername")
	f.Add("proxy:host:22@clustername")
	f.Add("proxy:host:22@namespace@clustername")

	f.Fuzz(func(t *testing.T, request string) {
		server := &Server{
			hostname:  "redhorse",
			proxyMode: true,
			logger:    slog.New(slog.DiscardHandler),
		}

		ctx := &srv.ServerContext{}

		require.NotPanics(t, func() {
			server.parseProxySubsys(context.Background(), request, ctx)
		})
	})
}
