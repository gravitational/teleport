// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/integrations/access/incidentio"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

var (
	appName                 = "teleport-incidentio"
	gracefulShutdownTimeout = 15 * time.Second
	configPath              = "/etc/teleport-incidentio.toml"
)

func main() {
	logger.Init()
	app := kingpin.New(appName, "Teleport incident.io plugin")

	app.Command("version", "Prints teleport-incidentio version and exits")

	startCmd := app.Command("start", "Starts Teleport incident.io plugin")
	startConfigPath := startCmd.Flag("config", "TOML config file path").
		Short('c').
		Default(configPath).
		String()
	debug := startCmd.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		Bool()

	selectedCmd, err := app.Parse(os.Args[1:])
	if err != nil {
		lib.Bail(err)
	}

	switch selectedCmd {

	case "version":
		lib.PrintVersion(app.Name, teleport.Version, teleport.Gitref)
	case "start":
		if err := run(*startConfigPath, *debug); err != nil {
			lib.Bail(err)
		} else {
			logger.Standard().Info("Successfully shut down")
		}
	}

}

func run(configPath string, debug bool) error {
	conf, err := incidentio.LoadConfig(configPath)
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
		logger.Standard().Debugf("DEBUG logging enabled")
	}

	app, err := incidentio.NewIncidentApp(context.Background(), conf)
	if err != nil {
		return trace.Wrap(err)
	}

	go lib.ServeSignals(app, gracefulShutdownTimeout)

	return trace.Wrap(
		app.Run(context.Background()),
	)
}
