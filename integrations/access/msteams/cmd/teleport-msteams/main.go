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
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/access/msteams"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

var (
	appName                 = "teleport-msteams"
	gracefulShutdownTimeout = 15 * time.Second
	configPath              = "/etc/teleport-msteams.toml"
)

func main() {
	logger.Init()
	app := kingpin.New(appName, "Teleport MS Teams plugin")

	app.Command("version", "Prints teleport-msteams version and exits")
	configureCmd := app.Command("configure", "Generates plugin and bot configuration")

	targetDir := configureCmd.Arg("dir", "Path to target directory").Required().String()
	appID := configureCmd.Flag("appID", "MS App ID").Required().String()
	appSecret := configureCmd.Flag("appSecret", "MS App Secret").Required().String()
	tenantID := configureCmd.Flag("tenantID", "MS App Tenant ID").Required().String()

	uninstallCmd := app.Command("uninstall", "Uninstall the application for all teams user.")
	uninstallConfigPath := uninstallCmd.Flag("config", "TOML config file path").
		Short('c').
		Default(configPath).
		String()

	validateCmd := app.Command("validate", "Validate bot installation")
	validateConfigPath := validateCmd.Flag("config", "TOML config file path").
		Short('c').
		Default(configPath).
		String()

	validateRecipientID := validateCmd.Arg("recipient", "UserID, email or channel to notify").Required().String()

	startCmd := app.Command("start", "Starts Teleport MS Teams plugin")
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
	case "configure":
		err := msteams.Configure(*targetDir, *appID, *appSecret, *tenantID)
		if err != nil {
			lib.Bail(err)
		}

	case "uninstall":
		err := msteams.Uninstall(context.Background(), *uninstallConfigPath)
		if err != nil {
			lib.Bail(err)
		}

	case "validate":
		err := msteams.Validate(*validateConfigPath, *validateRecipientID)
		if err != nil {
			lib.Bail(err)
		}

	case "version":
		lib.PrintVersion(app.Name, teleport.Version, teleport.Gitref)
	case "start":
		if err := run(*startConfigPath, *debug); err != nil {
			lib.Bail(err)
		} else {
			slog.InfoContext(context.Background(), "Successfully shut down")
		}
	}

}

func run(configPath string, debug bool) error {
	conf, err := msteams.LoadConfig(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if debug {
		conf.Log.Severity = "debug"
	}

	app, err := msteams.NewApp(*conf)
	if err != nil {
		return trace.Wrap(err)
	}

	go lib.ServeSignals(app, gracefulShutdownTimeout)

	return trace.Wrap(
		app.Run(context.Background()),
	)
}
