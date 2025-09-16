/*
Copyright 2019 Gravitational, Inc.

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

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/email"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

//go:embed example_config.toml
var exampleConfig string

func main() {
	logger.Init()
	app := kingpin.New("teleport-email", "Teleport plugin for access requests approval via E-mail.")

	app.Command("configure", "Prints an example .TOML configuration file.")
	app.Command("version", "Prints teleport-email version and exits.")

	startCmd := app.Command("start", "Starts a the Teleport Email plugin.")
	path := startCmd.Flag("config", "TOML config file path").
		Short('c').
		Default("/etc/teleport-email.toml").
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
			logger.Standard().Info("Successfully shut down")
		}
	}
}

func run(configPath string, debug bool) error {
	conf, err := email.LoadConfig(configPath)
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

	if conf.Delivery.Recipients != nil {
		logger.Standard().Warn("The delivery.recipients config option is deprecated, set role_to_recipients[\"*\"] instead for the same functionality")
	}

	app, err := email.NewApp(*conf)
	if err != nil {
		return trace.Wrap(err)
	}

	go lib.ServeSignals(app, common.PluginShutdownTimeout)

	logger.Standard().Infof("Starting Teleport Access Email Plugin %s:%s", teleport.Version, teleport.Gitref)
	return trace.Wrap(
		app.Run(context.Background()),
	)
}
