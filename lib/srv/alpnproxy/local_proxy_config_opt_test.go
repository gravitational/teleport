/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
		require.Equal(t, config.Protocols, []common.Protocol{common.ProtocolRedisDB})
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
