/*
Copyright 2015-2021 Gravitational, Inc.

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

package common

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
)

// TestDatabaseResource tests tctl db rm/get commands.
func TestDatabaseResource(t *testing.T) {
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Databases: config.Databases{
			Service: config.Service{
				EnabledFlag: "true",
			},
			Databases: []*config.Database{
				{
					Name:        "example",
					Description: "Example MySQL",
					Protocol:    "mysql",
					URI:         "localhost:33306",
				},
				{
					Name:        "example2",
					Description: "Example2 MySQL",
					Protocol:    "mysql",
					URI:         "localhost:33307",
				},
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: mustGetFreeLocalListenerAddr(t),
			TunAddr: mustGetFreeLocalListenerAddr(t),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: mustGetFreeLocalListenerAddr(t),
			},
		},
	}

	auth := makeAndRunTestAuthServer(t, withFileConfig(fileConfig))
	waitForBackendDatabaseResourcePropagation(t, auth.GetAuthServer())

	var out []*types.DatabaseServerV3

	t.Run("get all database servers", func(t *testing.T) {
		buff, err := runResourceCommand(t, fileConfig, []string{"get", types.KindDatabaseServer, "--format=json"})
		require.NoError(t, err)
		mustDecodeJSON(t, buff, &out)
		require.Len(t, out, 1)
		require.Len(t, out[0].GetDatabases(), 2)
	})

	server := fmt.Sprintf("%v/%v", types.KindDatabaseServer, out[0].GetName())

	t.Run("get specific database server", func(t *testing.T) {
		buff, err := runResourceCommand(t, fileConfig, []string{"get", server, "--format=json"})
		require.NoError(t, err)
		mustDecodeJSON(t, buff, &out)
		require.Len(t, out, 1)
		require.Len(t, out[0].GetDatabases(), 2)
	})

	t.Run("remove database server", func(t *testing.T) {
		_, err := runResourceCommand(t, fileConfig, []string{"rm", server})
		require.NoError(t, err)

		_, err = runResourceCommand(t, fileConfig, []string{"get", server, "--format=json"})
		require.Error(t, err)
		require.IsType(t, &trace.NotFoundError{}, err.(*trace.TraceErr).OrigError())

		buff, err := runResourceCommand(t, fileConfig, []string{"get", "db", "--format=json"})
		require.NoError(t, err)
		mustDecodeJSON(t, buff, &out)
		require.Len(t, out, 0)
	})
}
