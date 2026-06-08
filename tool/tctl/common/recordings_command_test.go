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
	"os"
	"strings"
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

// TestResumeCommand verifies that the rendered resume hint reuses the original
// invocation's arguments verbatim and only sets the resume token.
func TestResumeCommand(t *testing.T) {
	defer func(orig []string) { os.Args = orig }(os.Args)

	t.Run("appends token and preserves flags", func(t *testing.T) {
		os.Args = []string{
			"/usr/local/bin/tctl", "recordings", "search", "data exfiltration",
			"--kind", "ssh",
			"--from-utc", "2026-06-07",
			"--limit", "250",
			"--format", "json",
		}

		cmd := resumeCommand("tok-123")

		// The program name is shortened and every original flag is preserved.
		require.True(t, strings.HasPrefix(cmd, "tctl recordings search "))
		require.Contains(t, cmd, "--kind ssh")
		require.Contains(t, cmd, "--from-utc 2026-06-07")
		require.Contains(t, cmd, "--limit 250")
		require.Contains(t, cmd, "--format json")
		require.Contains(t, cmd, "'data exfiltration'") // space-containing arg quoted

		// The token is appended since it was not present originally.
		require.Contains(t, cmd, "--resume-token tok-123")
	})

	t.Run("replaces an existing token", func(t *testing.T) {
		os.Args = []string{"tctl", "recordings", "search", "--resume-token", "old", "--format", "yaml"}

		cmd := resumeCommand("new")

		require.Contains(t, cmd, "--resume-token new")
		require.NotContains(t, cmd, "old")
		require.Equal(t, 1, strings.Count(cmd, "--resume-token"))
	})
}

func TestReplaceOrAppendFlag(t *testing.T) {
	t.Run("replaces space-separated value", func(t *testing.T) {
		got := replaceOrAppendFlag([]string{"search", "--limit", "10"}, []string{"--limit"}, "--limit", "20")
		require.Equal(t, []string{"search", "--limit", "20"}, got)
	})
	t.Run("replaces inline value", func(t *testing.T) {
		got := replaceOrAppendFlag([]string{"search", "--limit=10"}, []string{"--limit"}, "--limit", "20")
		require.Equal(t, []string{"search", "--limit", "20"}, got)
	})
	t.Run("appends when absent", func(t *testing.T) {
		got := replaceOrAppendFlag([]string{"search"}, []string{"--resume-token"}, "--resume-token", "tok")
		require.Equal(t, []string{"search", "--resume-token", "tok"}, got)
	})
}

func TestShellQuote(t *testing.T) {
	require.Equal(t, "env=prod", shellQuote("env=prod"))
	require.Equal(t, "ssh", shellQuote("ssh"))
	require.Equal(t, "''", shellQuote(""))
	require.Equal(t, "'data exfiltration'", shellQuote("data exfiltration"))
	require.Equal(t, `'it'\''s'`, shellQuote("it's"))
}
