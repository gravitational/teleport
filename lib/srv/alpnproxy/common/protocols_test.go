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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithPingProtocols(t *testing.T) {
	require.Equal(t,
		[]Protocol{
			"teleport-tcp-ping",
			"teleport-redis-ping",
			"teleport-reversetunnel",
			"teleport-tcp",
			"teleport-redis",
			"h2",
		},
		WithPingProtocols([]Protocol{
			ProtocolReverseTunnel,
			ProtocolTCP,
			ProtocolRedisDB,
			ProtocolReverseTunnel,
			ProtocolHTTP2,
		}),
	)
}

func TestIsDBTLSProtocol(t *testing.T) {
	require.True(t, IsDBTLSProtocol("teleport-redis"))
	require.True(t, IsDBTLSProtocol("teleport-redis-ping"))
	require.False(t, IsDBTLSProtocol("teleport-tcp"))
	require.False(t, IsDBTLSProtocol(""))
}
