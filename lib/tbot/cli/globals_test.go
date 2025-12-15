/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// TestGlobals tests that GlobalArgs initialize and parse properly, and mutate
// BotConfig as expected.
func TestGlobalArgs(t *testing.T) {
	app, _ := buildMinimalKingpinApp("start")
	globals := NewGlobalArgs(app)

	// Note: various flags here are already tested as part of sharedStartArgs.
	_, err := app.Parse([]string{
		"start",
		"--debug",
		"--config=foo.yaml",
		"--fips",
		"--trace",
		"--trace-exporter=foo",
		"--insecure",
		"--log-format=json",
	})
	require.NoError(t, err)

	require.True(t, globals.Debug)
	require.True(t, globals.FIPS)
	require.True(t, globals.Insecure)
	require.True(t, globals.Trace)
	require.Equal(t, "foo", globals.TraceExporter)
	require.Equal(t, "foo.yaml", globals.ConfigPath)

	// Clear the config path, otherwise LoadConfigWithMutators will try to load
	// it.
	globals.ConfigPath = ""

	// Convert these args to a BotConfig and check it. Globals don't set many
	// config flags, so not much to check here.
	cfg, err := LoadConfigWithMutators(globals)
	require.NoError(t, err)

	require.True(t, cfg.Debug)
	require.True(t, cfg.FIPS)
	require.True(t, cfg.Insecure)
}

func TestGlobalInvertedFlags(t *testing.T) {
	app, _ := buildMinimalKingpinApp("start")
	globals := NewGlobalArgs(app)

	_, err := app.Parse([]string{
		"start",
		"--no-debug",
		"--no-fips",
		"--no-insecure",
		"--config=foo.yaml",
		"--trace",
		"--trace-exporter=foo",
		"--log-format=json",
	})
	require.NoError(t, err)

	cfg, err := TestConfigWithMutators(&config.BotConfig{
		Debug:    true,
		FIPS:     true,
		Insecure: true,
	}, globals)
	require.NoError(t, err)

	require.False(t, cfg.Debug)
	require.False(t, cfg.FIPS)
	require.False(t, cfg.Insecure)
}
