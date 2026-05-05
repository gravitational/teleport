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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

func TestRecordingsSearchResourcePropertyFlags(t *testing.T) {
	var c RecordingsCommand
	app := utils.InitCLIParser("tctl", GlobalHelpString)
	c.Initialize(app, &tctlcfg.GlobalCLIFlags{}, servicecfg.MakeDefaultConfig())

	selectedCmd, err := app.Parse([]string{
		"recordings", "search",
		"--server-hostname=host-1",
		"--server-addr=10.0.0.1:3022",
		"--pod-namespace=prod",
		"--pod-name=api-7fd",
		"--database-name=postgres",
	})
	require.NoError(t, err)
	require.Equal(t, c.recordingsSearch.FullCommand(), selectedCmd)
	require.Equal(t, "host-1", c.searchServerHostname)
	require.Equal(t, "10.0.0.1:3022", c.searchServerAddr)
	require.Equal(t, "prod", c.searchPodNamespace)
	require.Equal(t, "api-7fd", c.searchPodName)
	require.Equal(t, "postgres", c.searchDatabaseName)
}

func TestBuildSearchResourceProperties(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		got, err := (&RecordingsCommand{}).buildSearchResourceProperties()
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("ssh", func(t *testing.T) {
		got, err := (&RecordingsCommand{
			searchServerHostname: "host-1",
			searchServerAddr:     "10.0.0.1:3022",
		}).buildSearchResourceProperties()
		require.NoError(t, err)
		ssh := got.GetSsh()
		require.NotNil(t, ssh)
		require.Equal(t, "host-1", ssh.GetServerHostname())
		require.Equal(t, "10.0.0.1:3022", ssh.GetServerAddr())
	})

	t.Run("kubernetes", func(t *testing.T) {
		got, err := (&RecordingsCommand{
			searchPodNamespace: "prod",
			searchPodName:      "api-7fd",
		}).buildSearchResourceProperties()
		require.NoError(t, err)
		kubernetes := got.GetKubernetes()
		require.NotNil(t, kubernetes)
		require.Equal(t, "prod", kubernetes.GetPodNamespace())
		require.Equal(t, "api-7fd", kubernetes.GetPodName())
	})

	t.Run("database", func(t *testing.T) {
		got, err := (&RecordingsCommand{
			searchDatabaseName: "postgres",
		}).buildSearchResourceProperties()
		require.NoError(t, err)
		database := got.GetDatabase()
		require.NotNil(t, database)
		require.Equal(t, "postgres", database.GetDatabaseName())
	})

	t.Run("mixed variants", func(t *testing.T) {
		got, err := (&RecordingsCommand{
			searchServerHostname: "host-1",
			searchPodName:        "api-7fd",
		}).buildSearchResourceProperties()
		require.ErrorContains(t, err, "resource property filters can only target one session kind")
		require.Nil(t, got)
	})
}
