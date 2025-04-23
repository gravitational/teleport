/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/datadog"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

//go:embed example_config.toml
var exampleConfig string

func main() {
	logger.Init()
	app := kingpin.New("teleport-datadog", "Teleport plugin for access requests approval via Datadog.")

	app.Command("configure", "Prints an example .TOML configuration file.")
	app.Command("version", "Prints teleport-datadog version and exits.")

	startCmd := app.Command("start", "Starts a Teleport Datadog Incident Management plugin.")
	path := startCmd.Flag("config", "TOML config file path").
		Short('c').
		Default("/etc/teleport-datadog.toml").
		String()
	debug := startCmd.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		Bool()

	selectedCmd, err := app.Parse(os.Args[1:])
	if err != nil {
		lib.Bail(err)
	}

	switch selectedCmd {
	case "configure":
		fmt.Print(exampleConfig)
	case "version":
		lib.PrintVersion(app.Name, teleport.Version, teleport.Gitref)
	case "start":
		if err := run(*path, *debug); err != nil {
			lib.Bail(err)
		} else {
			slog.InfoContext(context.Background(), "Successfully shut down")
		}
	}
}

func run(configPath string, debug bool) error {
	ctx := context.Background()
	conf, err := datadog.LoadConfig(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	logConfig := conf.Log
	if debug {
		logConfig.Severity = "debug"
	}
	if err = logger.Setup(logConfig); err != nil {
		return err
	}
	if debug {
		slog.DebugContext(ctx, "DEBUG logging enabled")
	}

	app := datadog.NewDatadogApp(conf)
	go lib.ServeSignals(app, common.PluginShutdownTimeout)

	slog.InfoContext(ctx, "Starting Teleport Access Datadog Incident Management Plugin",
		"version", teleport.Version,
		"git_ref", teleport.Gitref,
	)
	return trace.Wrap(app.Run(ctx))
}
