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

package alpnproxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

func TestWithDatabaseProtocol(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var config LocalProxyConfig
		require.NoError(t, WithDatabaseProtocol(defaults.ProtocolRedis)(&config))
		require.Equal(t, []common.Protocol{common.ProtocolRedisDB}, config.Protocols)
	})
	t.Run("fail", func(t *testing.T) {
		var config LocalProxyConfig
		require.Error(t, WithDatabaseProtocol("unknown")(&config))
	})
}

func TestWithMySQLVersionProto(t *testing.T) {
	mysql, err := types.NewDatabaseV3(types.Metadata{
		Name: "mysql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	t.Run("no version", func(t *testing.T) {
		config := LocalProxyConfig{
			Protocols: []common.Protocol{common.ProtocolMySQL},
		}
		require.NoError(t, WithMySQLVersionProto(mysql)(&config))
		require.Equal(t, []common.Protocol{common.ProtocolMySQL}, config.Protocols)
	})

	t.Run("with version", func(t *testing.T) {
		mysql.SetMySQLServerVersion("8.0.28")
		config := LocalProxyConfig{
			Protocols: []common.Protocol{common.ProtocolMySQL},
		}
		require.NoError(t, WithMySQLVersionProto(mysql)(&config))
		require.Equal(t, []common.Protocol{common.ProtocolMySQL, "teleport-mysql-OC4wLjI4"}, config.Protocols)
	})
}
