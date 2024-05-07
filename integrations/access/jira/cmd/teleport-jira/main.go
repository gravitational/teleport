/*
Copyright 2020-2021 Gravitational, Inc.

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
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/access/jira"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	DefaultDir = "/var/lib/teleport/plugins/jira"
)

//go:embed example_config.toml
var exampleConfig string

func main() {
	logger.Init()
	app := kingpin.New("teleport-jira", "Teleport plugin for access requests approval via Jira.")

	app.Command("configure", "Prints an example .TOML configuration file.")
	app.Command("version", "Prints teleport-jira version and exits.")

	startCmd := app.Command("start", "Starts a Teleport Jira Plugin.")
	path := startCmd.Flag("config", "TOML config file path").
		Short('c').
		Default("/etc/teleport-jira.toml").
		String()
	debug := startCmd.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		Bool()
	insecure := startCmd.Flag("insecure-no-tls", "Disable TLS for the callback server").
		Default("false").
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
		if err := run(*path, *insecure, *debug); err != nil {
			lib.Bail(err)
		} else {
			logger.Standard().Info("Successfully shut down")
		}
	}
}

func run(configPath string, insecure bool, debug bool) error {
	conf, err := jira.LoadConfig(configPath)
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

	conf.HTTP.Insecure = insecure
	app, err := jira.NewApp(*conf)
	if err != nil {
		return trace.Wrap(err)
	}

	go lib.ServeSignals(app, 15*time.Second)

	return trace.Wrap(
		app.Run(context.Background()),
	)
}
