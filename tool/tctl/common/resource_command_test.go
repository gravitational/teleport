/*
Copyright 2021 Gravitational, Inc.

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
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestDatabaseServerResource tests tctl db_server rm/get commands.
func TestDatabaseServerResource(t *testing.T) {
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
		require.Len(t, out, 2)
	})

	server := fmt.Sprintf("%v/%v", types.KindDatabaseServer, out[0].GetName())

	t.Run("get specific database server", func(t *testing.T) {
		buff, err := runResourceCommand(t, fileConfig, []string{"get", server, "--format=json"})
		require.NoError(t, err)
		mustDecodeJSON(t, buff, &out)
		require.Len(t, out, 1)
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

// TestDatabaseResource tests tctl commands that manage database resources.
func TestDatabaseResource(t *testing.T) {
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Databases: config.Databases{
			Service: config.Service{
				EnabledFlag: "true",
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

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig))

	dbA, err := types.NewDatabaseV3(types.Metadata{
		Name:   "dbA",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	dbB, err := types.NewDatabaseV3(types.Metadata{
		Name:   "dbB",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	var out []*types.DatabaseV3

	// Initially there are no databases.
	buf, err := runResourceCommand(t, fileConfig, []string{"get", types.KindDatabase, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 0)

	// Create the databases.
	dbYAMLPath := filepath.Join(t.TempDir(), "db.yaml")
	require.NoError(t, ioutil.WriteFile(dbYAMLPath, []byte(dbYAML), 0644))
	_, err = runResourceCommand(t, fileConfig, []string{"create", dbYAMLPath})
	require.NoError(t, err)

	// Fetch the databases, should have 2.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", types.KindDatabase, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Empty(t, cmp.Diff([]*types.DatabaseV3{dbA, dbB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	// Fetch specific database.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", fmt.Sprintf("%v/dbB", types.KindDatabase), "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Empty(t, cmp.Diff([]*types.DatabaseV3{dbB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	// Remove a database.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/dbA", types.KindDatabase)})
	require.NoError(t, err)

	// Fetch all databases again, should have 1.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", types.KindDatabase, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Empty(t, cmp.Diff([]*types.DatabaseV3{dbB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))
}

// TestAppResource tests tctl commands that manage application resources.
func TestAppResource(t *testing.T) {
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Apps: config.Apps{
			Service: config.Service{
				EnabledFlag: "true",
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

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig))

	appA, err := types.NewAppV3(types.Metadata{
		Name:   "appA",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)

	appB, err := types.NewAppV3(types.Metadata{
		Name:   "appB",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)

	var out []*types.AppV3

	// Initially there are no apps.
	buf, err := runResourceCommand(t, fileConfig, []string{"get", types.KindApp, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 0)

	// Create the apps.
	appYAMLPath := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, ioutil.WriteFile(appYAMLPath, []byte(appYAML), 0644))
	_, err = runResourceCommand(t, fileConfig, []string{"create", appYAMLPath})
	require.NoError(t, err)

	// Fetch the apps, should have 2.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", types.KindApp, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Empty(t, cmp.Diff([]*types.AppV3{appA, appB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	// Fetch specific app.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", fmt.Sprintf("%v/appB", types.KindApp), "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Empty(t, cmp.Diff([]*types.AppV3{appB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	// Remove an app.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/appA", types.KindApp)})
	require.NoError(t, err)

	// Fetch all apps again, should have 1.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", types.KindApp, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Empty(t, cmp.Diff([]*types.AppV3{appB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))
}

const (
	dbYAML = `kind: db
version: v3
metadata:
  name: dbA
spec:
  protocol: "postgres"
  uri: "localhost:5432"
---
kind: db
version: v3
metadata:
  name: dbB
spec:
  protocol: "mysql"
  uri: "localhost:3306"`

	appYAML = `kind: app
version: v3
metadata:
  name: appA
spec:
  uri: "localhost1"
---
kind: app
version: v3
metadata:
  name: appB
spec:
  uri: "localhost2"`
)
